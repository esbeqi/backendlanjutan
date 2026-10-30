$ErrorActionPreference='Stop'
Add-Type -Namespace C6WorkerFinal -Name ConsoleApi -MemberDefinition '[System.Runtime.InteropServices.DllImport("kernel32.dll", SetLastError=true)] public static extern bool SetConsoleCtrlHandler(System.IntPtr handler, bool add); [System.Runtime.InteropServices.DllImport("kernel32.dll", SetLastError=true)] public static extern bool GenerateConsoleCtrlEvent(uint eventType, uint processGroupId);'
if (-not [C6WorkerFinal.ConsoleApi]::SetConsoleCtrlHandler([IntPtr]::Zero,$false)) { throw 'SetConsoleCtrlHandler(FALSE) failed' }
$project=(Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
Set-Location $project
$env:MAILER_DELAY_MS='5000'
$env:MAILER_FAIL_EVERY='0'
$baseCount=(Select-String -Path logs\app.log -Pattern '"msg":"relay_done"').Count
$worker=Start-Process -FilePath (Join-Path $env:TEMP 'latihan-fiber-c6-retest.exe') -ArgumentList '-role=worker' -WorkingDirectory $project -PassThru -NoNewWindow
"worker_pid=$($worker.Id) delay_ms=5000 ctrlc_inheritance_reset=true baseline_relay_count=$baseCount" | Add-Content (Join-Path $PSScriptRoot 'worker.log')
$deadline=(Get-Date).AddSeconds(20)
$relay=$null
while (-not $relay -and (Get-Date) -lt $deadline) {
  if ((Select-String -Path logs\app.log -Pattern '"msg":"relay_done"').Count -gt $baseCount) {
    $rows=Get-Content logs\app.log -Tail 60 | ForEach-Object { try { $_ | ConvertFrom-Json } catch { $null } }
    $relay=$rows | Where-Object { $_.msg -eq 'relay_done' -and $_.published -eq 1 } | Select-Object -Last 1
  }
  if (-not $relay) { Start-Sleep -Milliseconds 100 }
}
if (-not $relay) { throw 'new relay_done was not observed after worker start' }
$relay | ConvertTo-Json -Compress | Set-Content (Join-Path $PSScriptRoot 'relay-done.log')
$relayTime=[DateTimeOffset]::Parse($relay.time)
$due=$relayTime.LocalDateTime.AddSeconds(2)
while ((Get-Date) -lt $due) { Start-Sleep -Milliseconds 50 }
[C6WorkerFinal.ConsoleApi]::SetConsoleCtrlHandler([IntPtr]::Zero,$true) | Out-Null
$signalTime=Get-Date -Format o
$sent=[C6WorkerFinal.ConsoleApi]::GenerateConsoleCtrlEvent(0,0)
"ctrl_c_event_sent=$sent at=$signalTime; worker_pid=$($worker.Id)" | Add-Content (Join-Path $PSScriptRoot 'worker.log')
$deadline=(Get-Date).AddSeconds(15)
do { Start-Sleep -Milliseconds 100; $worker.Refresh() } while (-not $worker.HasExited -and (Get-Date) -lt $deadline)
"worker_exit=$($worker.HasExited) code=$($worker.ExitCode)" | Add-Content (Join-Path $PSScriptRoot 'worker.log')
$env:MAILER_DELAY_MS='1500'
'MAILER_DELAY_MS restored to 1500; .env unchanged' | Set-Content (Join-Path $PSScriptRoot 'worker-env-restored.txt')


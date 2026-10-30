Add-Type -Namespace C6Worker -Name Signal -MemberDefinition '[System.Runtime.InteropServices.DllImport("kernel32.dll", SetLastError=true)] public static extern bool SetConsoleCtrlHandler(System.IntPtr handler, bool add);'
if (-not [C6Worker.Signal]::SetConsoleCtrlHandler([IntPtr]::Zero,$false)) { throw 'Could not restore normal Ctrl+C handling before worker start' }
$project = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
Set-Location $project
$env:MAILER_DELAY_MS='5000'
$env:MAILER_FAIL_EVERY='0'
try {
  & (Join-Path $env:TEMP 'latihan-fiber-c6-retest.exe') -role=worker
} finally {
  $env:MAILER_DELAY_MS='1500'
  'MAILER_DELAY_MS=1500 (worker host process restored; .env unchanged)' | Set-Content (Join-Path $PSScriptRoot 'worker-env-restored.txt')
}

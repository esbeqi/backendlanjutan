# Modul 9 acceptance runner

Run these commands from the `latihan-fiber` project root. The API server must
already be running. The runner reads the project's `.env` through the same
configuration loader to obtain the database settings and JWT secret. It does
not reset rows automatically.

```powershell
go run ./tools/loadtest -h
go run ./tools/loadtest -mode crowd
go run ./tools/loadtest -mode credits
go run ./tools/loadtest -mode setup-swap
go run ./tools/loadtest -mode swap
go run ./tools/loadtest -mode functional
go run ./tools/loadtest -mode health
go run ./tools/loadtest -mode logs
```

`-mode crowd`, `credits`, `setup-swap`, `swap`, and `functional` connect to
PostgreSQL to look up seeded test users/courses and to verify counts. They also
send HTTP requests to the running API. `-mode health` only calls
`GET /api/v1/health`. `-mode logs` only reads `logs/app.log`; use `-log PATH` to
inspect another saved log excerpt.

## Prerequisites and state

- The practice database and test accounts `mhs001` through the required range
  must already exist. The app and runner must use the same `JWT_SECRET`,
  `JWT_ISSUER`, and `DB_*` settings. `APP_PORT` defaults to `3000`; override the
  API address with `-url` when needed.
- Before a fresh C.1, C.2, or C.4 run, reset the practice data as specified in
  Modul 9. The runner refuses to start those scenarios when their required
  enrollment state is dirty. It never runs reset SQL.
- For C.3, after reset run `-mode setup-swap` once to place `mhs001`–`mhs020`
  in IF101 and `mhs021`–`mhs040` in IF102, then run `-mode swap`. Repeat only
  after resetting and running `setup-swap` again.
- C.5: after a fresh reset, run `-mode crowd` three times consecutively without
  resetting or restarting the server. The first run expects 5 successes and 55
  full responses; later runs expect 60 full responses. Then run `-mode health`.
- Each request wave starts all goroutines behind one shared gate. The runner
  checks response status/code and database enrollment counts after the wave.

The functional suite uses the seeded IDs and the IF101/IF102/IF399 setup. It
checks duplicate enrollment, full quota, dropping an unowned enrollment, failed
and successful swaps, same-course swap validation, current/stale quota version,
quota below enrolled count, and ordinary-user quota authorization. It expects
the module's C.4 result of 422 for same-course swap; the current implementation
returns 400 through `helper.Validation`, so that case is expected to report the
known mismatch while the remaining cases continue.

## What is not automated

- The runner does not reset data, alter quotas for setup, or seed initial swap
  enrollments except when explicitly running `setup-swap` through the API.
- Log scanning can count `WARN request_rejected`, `courses_enrolled_within_quota`,
  and `context deadline exceeded` in the selected file. The application logger
  does not currently write database error causes into its structured log, so a
  missing constraint/deadline phrase cannot prove that no such error occurred.
  Unexpected 5xx responses are also counted in the scenario output.
- C.7 build, unit tests, and vet are standard project commands:

```powershell
go build ./...
go test ./...
go vet ./...
```

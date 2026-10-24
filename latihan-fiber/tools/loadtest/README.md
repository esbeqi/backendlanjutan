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
go run ./tools/loadtest -mode reviews -review-scenario read
go run ./tools/loadtest -mode reviews -review-scenario validation
go run ./tools/loadtest -mode reviews -review-scenario write
go run ./tools/loadtest -mode reviews -review-scenario concurrent
go run ./tools/loadtest -mode health
go run ./tools/loadtest -mode logs
```

`-mode crowd`, `credits`, `setup-swap`, `swap`, and `functional` connect to
PostgreSQL to look up seeded test users/courses and to verify counts. They also
send HTTP requests to the running API. `-mode health` only calls
`GET /api/v1/health`. `-mode logs` only reads `logs/app.log`; use `-log PATH` to
inspect another saved log excerpt.

## Modul 10 review scenarios

The review runner uses the existing PostgreSQL configuration and JWT settings,
then sends HTTP requests to the running API. `tokenFor` looks up the selected
user ID and username in PostgreSQL and signs a local access token with the
configured `JWT_SECRET` and `JWT_ISSUER`. It does not log in with a password and
does not print the token. A generated admin-role token is only a local test
credential; it does not establish the user's stored role or prove password
authentication. Keep the runner and server on the same local test
configuration. The runner signs tokens with a fixed 15-minute lifetime; keep
the application's `JWT_ACCESS_TTL_MINUTES` at that setting for matching token
lifetimes (the middleware checks the token's signature, issuer, and expiration).

- `-review-scenario read` is read-only. It selects an existing user and course
  from PostgreSQL at runtime and checks review list and summary responses.
- `-review-scenario validation` sends invalid or rejected requests only. It
  checks rating/comment/tag validation, NoSQL-shaped body and query inputs,
  malformed review ID, missing course, and a user/course pair with no
  enrollment. It selects IDs and enrollment state from PostgreSQL at runtime;
  these cases are expected not to create review data.
- `-review-scenario write` creates a temporary review only for a currently
  enrolled test user/course pair that does not already have a review. It checks
  normalized tags, duplicate creation, reading, summary, owner/non-owner update,
  reply, helpful vote and duplicate vote, then non-owner and moderator deletion.
  The created review is deleted through the API on success; deferred cleanup
  attempts owner deletion if an assertion fails. An abrupt process stop may
  leave the temporary review behind. The moderator case requires migration 007
  applied and the server restarted so its permission cache reloads.
- `-review-scenario concurrent` creates a temporary review, runs 20 simultaneous
  helpful votes from one user and 30 votes from distinct users, checks the
  resulting counter, then deletes the temporary review through the API. This
  is a write/concurrency scenario, not a read-only check.

Both write scenarios require existing `mhs001`–`mhs060` accounts, course rows,
and at least one enrolled user/course pair without an existing review. The
concurrent scenario also needs at least 31 other test users besides the review
owner. The validation scenario needs at least one non-enrolled user/course
pair. The runner discovers these records at runtime; it does not assume course
IDs or passwords. Write scenarios change the configured practice database and
should be run only against an isolated local test dataset after reviewing its
state. Do not run them against data you need to preserve. No review scenario
resets the database or deletes pre-existing reviews.

For the safest first request, start the app and run `go run ./tools/loadtest
-mode reviews -review-scenario read`. The `validation` scenario is also
non-writing by design. Do not begin with `write` or `concurrent` until the
database is isolated and the listed prerequisites are satisfied. Pass
`-url http://localhost:<port>` to target a non-default local port.

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

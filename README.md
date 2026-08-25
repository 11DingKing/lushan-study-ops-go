# Lushan Study Operations

Lushan Study Operations is a Go backend for delivering cross-venue study tours around Lushan. It coordinates school and family cohorts through application, plan composition, venue and mentor holds, risk acknowledgement, confirmation, attendance grouping, weather rerouting, artifact archival, cancellation settlement, and durable follow-up work.

## Runtime

- Go 1.25 or later
- SQLite with WAL, foreign keys, busy timeout, and versioned embedded migrations
- JSON HTTP API on `:8080` by default
- Durable outbox worker with restart recovery, bounded retry, exponential backoff, and permanent failure state

Copy `.env.example` values into the process environment as needed. No credentials are loaded from files by the application. The default bootstrap password is only for local startup and should always be replaced in a deployed environment.

```bash
go run ./cmd/server
```

The server creates the configured database directory, applies every pending migration transactionally, ensures the bootstrap operator exists, starts the outbox worker, and then serves HTTP. `SIGINT` and `SIGTERM` trigger graceful HTTP and worker shutdown.

## Identity And Roles

`POST /v1/auth/login` exchanges an email and password for an opaque server-side session token. Only a SHA-256 token hash is persisted. Sessions have an explicit expiry, can be revoked through `POST /v1/auth/logout`, and are rejected after revocation or expiry.

The business roles are:

- `leader`: applies for a cohort, acknowledges the current plan risk, records attendance, and can cancel a cohort it owns.
- `operator`: approves applications, composes plans, confirms resources, manages reroutes, and can cancel operationally.
- `venue_admin`: reads operational cohort schedules for venue coordination.
- `mentor`: records attendance and archives eligible learning artifacts.
- `safety`: manages attendance groups, records attendance, and performs safety reroutes.

## Business Invariants

Application approval and cohort lifecycle changes use optimistic versions. Venue seats and mentor assignments are held together with each plan item. Confirmation verifies the approved application, current plan revision, risk acknowledgement, non-expired holds, and matching venue/mentor ownership in one transaction. Audit and outbox records participate in critical transactions.

Attendance is unique per participant and cohort. Present and late participants consume group capacity; absent participants do not. Batch attendance intentionally returns per-item outcomes, preserving valid records when another participant is invalid. Artifacts can only be archived for present or late participants and are protected from duplicate archival.

Rerouting validates the replacement venue capacity and atomically changes both the venue hold and plan item under an optimistic hold version. Cancellation releases owned resources, transitions the cohort, creates the policy settlement, queues durable settlement work, and writes the audit event within one database transaction.

## API

Public endpoints:

```text
GET  /healthz
GET  /readyz
POST /v1/auth/login
```

Authenticated workflow endpoints:

```text
POST /v1/auth/logout
POST /v1/applications
POST /v1/cohorts/{id}/decision
GET  /v1/cohorts
POST /v1/cohorts/{id}/plan-items
POST /v1/cohorts/{id}/acknowledgements
POST /v1/cohorts/{id}/confirm
POST /v1/cohorts/{id}/attendance-groups
POST /v1/cohorts/{id}/attendance
POST /v1/cohorts/{id}/attendance/batch
POST /v1/cohorts/{id}/reroutes
POST /v1/cohorts/{id}/artifacts
POST /v1/cohorts/{id}/cancel
```

Every request carries or receives an `X-Request-ID`. Errors use a stable JSON envelope with `code`, a safe public `message`, and the request ID. The readiness endpoint checks the actual database dependency.

## Database

The initial migration creates related tables for users, sessions, cohorts, applications, courses, venues, mentors, plan items, venue holds, mentor assignments, risk acknowledgements, attendance groups and records, weather alerts, reroutes, artifacts, settlements, outbox jobs, idempotency records, and audit events. Foreign keys, unique constraints, lifecycle checks, time fields, versions, and conflict/query indexes are declared in the migration.

Migrations are embedded in the executable and tracked in `schema_migrations`. Repeated startup is idempotent. Tests create real temporary SQLite databases, including a close/reopen recovery test.

## Verification

```bash
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go build ./...
```

The test suite covers domain transitions, permissions, password/session lifecycle, migrations, foreign keys, rollback, optimistic concurrency, resource capacity and overlap, pagination, context cancellation, worker recovery and retry, partial batch outcomes, complete service workflows, and HTTP status/error/request-ID contracts.

## Docker

The root `Dockerfile` builds the real `./cmd/server` entry using the version compatible with `go.mod`. It uses BuildKit target arguments and does not copy host binaries, caches, local databases, Git metadata, or authoring checkpoints.

```bash
docker buildx build --platform linux/amd64 --load -t lushan-study-ops:amd64 .
docker buildx build --platform linux/arm64 --load -t lushan-study-ops:arm64 .
```

The default entry starts the server with persistent data at `/data/lushan-study.db`. The image health check calls `/healthz`; deployment readiness should call `/readyz`.

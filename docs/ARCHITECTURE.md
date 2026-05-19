# formae-plugin-supabase — Architecture

## Provider Overview

Supabase is a hosted Postgres-backed BaaS. Resources are managed via the
[Supabase Management API](https://api.supabase.com/api/v1) — a REST API with
Bearer token authentication. No official Go SDK exists; the Supabase
Terraform provider uses an `oapi-codegen` client at
`github.com/supabase/cli/pkg/api`.

## Transport Layer

REST over HTTPS. Pulled in as a minimal hand-rolled HTTP client to keep
dependency surface small (the supabase-cli package brings transitive deps
unrelated to plugin needs).

```
pkg/transport/supabase/
├── client.go      # HTTP client, Do(), Bearer auth, retries
├── errors.go      # HTTP status -> formae ErrorCode mapping
└── client_test.go # unit tests
```

Base URL: `https://api.supabase.com`
Default rate limit: 120 req/min (most endpoints). Plugin advertises 2 req/s
(`MaxRequestsPerSecondForNamespace = 2`) — well below the limit.

## Authentication

Bearer Personal Access Token. Read from environment:

| Env var | Required | Description |
|---------|----------|-------------|
| `SUPABASE_ACCESS_TOKEN` | yes | PAT created at https://supabase.com/dashboard/account/tokens |

Target config carries no secret — just deployment metadata:

```pkl
config = new Config {
    organizationId = "abcdefg"  // default organization for created projects
}
```

This mirrors `formae-plugin-sftp`: target config = location/scope,
credentials = environment.

## Native ID Format

Each resource type encodes its parent path:

- Project: `{ref}` — Supabase's 20-char project reference
- Sub-resources under a project: `{ref}/{kind}/{id}`

Encoded path is parsed back inside Read/Update/Delete.

## Async Operations

Project Create / Delete are long-running. Plugin returns:

```go
&resource.ProgressResult{
    OperationStatus: resource.OperationStatusInProgress,
    RequestID:       projectRef,
    NativeID:        projectRef,
}
```

`Status()` polls `GET /v1/projects/{ref}` and inspects `status`:

| API status | formae status |
|------------|---------------|
| `ACTIVE_HEALTHY` | `Success` |
| `INACTIVE`, `INIT_FAILED`, `REMOVED` | `Failure` |
| anything else (COMING_UP, RESTORING, ...) | `InProgress` |

Most other resources are synchronous.

## Error Mapping

| HTTP | formae ErrorCode |
|------|------------------|
| 400, 422 | `InvalidRequest` |
| 401, 403 | `Unauthorized` |
| 404 | `NotFound` |
| 409 | `AlreadyExists` |
| 429 | `Throttling` |
| 5xx | `ServiceError` |
| other | `InternalFailure` |

## Layout

```
formae-plugin-supabase/
├── formae-plugin.pkl
├── main.go                   # SDK entrypoint
├── supabase.go               # Plugin (ResourcePlugin) implementation
├── schema/pkl/
│   ├── PklProject
│   └── supabase.pkl          # Config + resource classes
├── pkg/
│   ├── transport/supabase/   # HTTP client
│   └── resources/            # per-resource CRUD bodies
│       └── project/
├── docs/
│   ├── ARCHITECTURE.md
│   └── RESOURCES.md
└── testdata/                 # conformance fixtures
```

## Phased Delivery

| Phase | Scope |
|-------|-------|
| 1 (this PR) | Transport client + `SUPABASE::Platform::Project` end-to-end |
| 2 | Branch, EdgeFunction, Secret |
| 3 | APIKey, Storage::Bucket |
| 4 | Config singletons (AuthSettings, APISettings, PoolerConfig) |

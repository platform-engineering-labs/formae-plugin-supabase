# Supabase Resource Types

Resource types this plugin manages. Based on the Supabase Management API v1 and the official Supabase Terraform provider.

## Platform Resources

| Resource Type | API Endpoint | CRUD | Priority |
|---------------|--------------|------|----------|
| `SUPABASE::Platform::Project` | `/v1/projects` | C, R, U, D, L | P1 |
| `SUPABASE::Platform::Branch` | `/v1/projects/{ref}/branches` | C, R, U, D, L | P1 |
| `SUPABASE::Platform::Organization` | `/v1/organizations` | R, L | P2 |

## Auth / Access Resources

| Resource Type | API Endpoint | CRUD | Priority |
|---------------|--------------|------|----------|
| `SUPABASE::Auth::APIKey` | `/v1/projects/{ref}/api-keys` | C, R, U, D, L | P1 |
| `SUPABASE::Auth::SSOProvider` | `/v1/projects/{ref}/config/auth/sso/providers` | C, R, U, D, L | P3 |

## Database Resources

| Resource Type | API Endpoint | CRUD | Priority |
|---------------|--------------|------|----------|
| `SUPABASE::Database::Settings` | `/v1/projects/{ref}/config/database/postgres` | R, U | P2 |
| `SUPABASE::Database::PoolerConfig` | `/v1/projects/{ref}/config/database/pgbouncer` | R, U | P2 |
| `SUPABASE::Database::NetworkRestriction` | `/v1/projects/{ref}/network-restrictions` | R, U | P3 |

## Functions

| Resource Type | API Endpoint | CRUD | Priority |
|---------------|--------------|------|----------|
| `SUPABASE::Functions::EdgeFunction` | `/v1/projects/{ref}/functions` | C, R, U, D, L | P1 |
| `SUPABASE::Functions::Secret` | `/v1/projects/{ref}/secrets` | C, R, U, D, L | P1 |

## Storage

| Resource Type | API Endpoint | CRUD | Priority |
|---------------|--------------|------|----------|
| `SUPABASE::Storage::Bucket` | `/v1/projects/{ref}/storage/buckets` | C, R, U, D, L | P2 |

## Settings (singletons per project)

| Resource Type | API Endpoint | CRUD | Priority |
|---------------|--------------|------|----------|
| `SUPABASE::Config::AuthSettings` | `/v1/projects/{ref}/config/auth` | R, U | P2 |
| `SUPABASE::Config::APISettings` | `/v1/projects/{ref}/postgrest` | R, U | P2 |

## Legend

- C: Create, R: Read, U: Update, D: Delete, L: List
- P1: implement first (this iteration)
- P2: follow-up
- P3: nice to have

## Native ID Format

- Project: `{project_ref}` (e.g. `abcdefghijklmnop`)
- Branch: `{project_ref}/branches/{branch_id}`
- APIKey: `{project_ref}/api-keys/{key_id}`
- EdgeFunction: `{project_ref}/functions/{slug}`
- Secret: `{project_ref}/secrets/{name}`
- Bucket: `{project_ref}/buckets/{name}`

## Project Create Fields (reference)

```json
{
  "name": "string",
  "organization_id": "string",
  "region": "us-east-1",
  "db_pass": "string (>= 8 chars)",
  "plan": "free|pro|team|enterprise",
  "desired_instance_size": "micro|small|medium|large|xlarge|2xlarge|..."
}
```

Returns: `id` (ref), `name`, `organization_id`, `region`, `created_at`, `status`, `database`.

Project creation is async — status transitions through `COMING_UP`, `ACTIVE_HEALTHY`, etc. Plugin returns `InProgress` with `RequestID = project_ref` and polls via `Status()`.

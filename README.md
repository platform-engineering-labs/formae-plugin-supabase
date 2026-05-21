# formae-plugin-supabase

[formae](https://formae.io/) plugin for [Supabase](https://supabase.com/).
Talks to the Supabase Management API at `https://api.supabase.com`.

> Status: feature-complete for the Management API resource set. APIKey
> conformance passes end-to-end live (Create, Verify, Extract, Sync,
> Update, Destroy, OOB Del). Full architectural notes in
> [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Supported Resources

10 resource types across 4 namespaces. Every type implements full Create,
Read, Update, Delete, List + Status (async polling where applicable).

### `SUPABASE::Platform::*` — top-level entities

| Resource | API endpoint | Async | Notes |
|---|---|---|---|
| `SUPABASE::Platform::Project` | `POST /v1/projects`, `GET/PATCH/DELETE /v1/projects/{ref}` | yes | Polls `status` until `ACTIVE_HEALTHY`. Provisioning bills the account and takes 2–3 min. |
| `SUPABASE::Platform::Branch` | `POST /v1/projects/{ref}/branches`, `GET/PATCH/DELETE /v1/branches/{id}` | yes | Polls until `FUNCTIONS_DEPLOYED` / `MIGRATIONS_PASSED`. Paid plan only. |
| `SUPABASE::Platform::Organization` | `POST/GET /v1/organizations` | — | Update + Delete reported as no-ops (unsupported by the API). |

### `SUPABASE::Auth::*` — credentials

| Resource | API endpoint | Notes |
|---|---|---|
| `SUPABASE::Auth::APIKey` | `/v1/projects/{ref}/api-keys{,/{id}}` | Publishable / secret keys with optional JWT template. `?reveal=true` returns the raw value. Same-plugin DAGs reference `anonKey.res.apiKey`. |

### `SUPABASE::Functions::*` — Edge Functions + secrets

| Resource | API endpoint | Notes |
|---|---|---|
| `SUPABASE::Functions::EdgeFunction` | `/v1/projects/{ref}/functions{,/{slug}}` | Inline JS/TS body. Eszip multipart deploy is out of scope. |
| `SUPABASE::Functions::Secret` | `/v1/projects/{ref}/secrets` | Bulk endpoints; modelled as one Forma resource per secret name. Values are write-only — drift on value is invisible. |

### `SUPABASE::Config::*` — per-project singletons

Each singleton is keyed by project ref; payload is opaque `Mapping<String, Any>`. Create + Update both translate to PATCH/PUT (singletons always exist server-side).

| Resource | Endpoint | Method | Payload |
|---|---|---|---|
| `SUPABASE::Config::AuthSettings` | `/v1/projects/{ref}/config/auth` | PATCH | `site_url`, mailer, providers, JWT, rate limits, … (~80 keys) |
| `SUPABASE::Config::APISettings` | `/v1/projects/{ref}/postgrest` | PATCH | `db_schema`, `max_rows`, `db_extra_search_path`, `jwt_secret` |
| `SUPABASE::Config::DatabaseSettings` | `/v1/projects/{ref}/config/database/postgres` | PUT | `statement_timeout`, `max_connections`, shared buffers, etc. |
| `SUPABASE::Config::NetworkRestriction` | `/v1/projects/{ref}/network-restrictions` | PATCH | `dbAllowedCidrs`, `dbAllowedCidrsV6` |

### Discovery + extract

All resources are `discoverable = true` (except the singletons, which surface via their parent project). `formae extract --schema-location local --query 'target:supabase-target' out.pkl` produces a complete PKL representation of an existing project — see [`examples/import-demo/`](examples/import-demo/).

### Not (yet) covered

| Feature | Why missing | Workaround |
|---|---|---|
| Storage buckets | Management API only exposes `GET`; create/delete need the Storage REST API on the project subdomain | Use the Storage SDK or REST API directly |
| SSO providers | Future work | Manage via dashboard |
| Third-party auth | Future work | Manage via dashboard |
| Custom hostname | Future work | Manage via dashboard |
| Database backups | Future work | Use the dashboard / `pg_dump` |
| Billing add-ons | Out of scope | Manage via dashboard |
| Read-replicas | Future work | Manage via dashboard |

## Configuration

Credentials live in environment variables, not the forma. Create a Personal Access Token at <https://supabase.com/dashboard/account/tokens>:

```bash
export SUPABASE_ACCESS_TOKEN=sbp_xxxxxxxxxxxx
```

A target in your forma carries only deployment metadata:

```pkl
import "@supabase/supabase.pkl"

new formae.Target {
    label = "supabase-prod"
    config = new supabase.Config {
        organizationId = "your-org-slug-or-id"
        baseUrl        = null   // defaults to https://api.supabase.com
    }
}
```

## Example

```pkl
import "@formae/formae.pkl"
import "@supabase/supabase.pkl"

new formae.Forma {
    resources {
        new supabase.Project {
            label          = "demo"
            name           = "demo"
            organizationId = "your-org"
            region         = "us-east-1"
            dbPass         = read("env:SUPABASE_DB_PASS")
            plan           = "free"
        }
    }
}
```

More end-to-end examples in [`examples/`](examples/):

- [`examples/basic/`](examples/basic/) — single Edge Function
- [`examples/k8s-supabase/`](examples/k8s-supabase/) — Next.js + Supabase
  Auth demo running in Kubernetes (cross-plugin)
- [`examples/import-demo/`](examples/import-demo/) — extract an existing
  Supabase project as PKL via `formae extract`

## Development

```bash
make build               # build plugin binary
make install             # build + install to ~/.pel/formae/plugins
go test -tags=unit ./... # unit tests (34 passing)
make conformance-test    # live API conformance tests (needs SUPABASE_ACCESS_TOKEN)
```

## Licensing

Apache-2.0.

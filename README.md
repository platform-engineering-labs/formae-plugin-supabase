# formae-plugin-supabase

[formae](https://formae.io/) plugin for [Supabase](https://supabase.com/).
Talks to the Supabase Management API at `https://api.supabase.com`.

> Status: feature-complete for the Management API resource set. APIKey
> conformance passes end-to-end live (Create, Verify, Extract, Sync,
> Update, Destroy, OOB Del). Full architectural notes in
> [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Supported Resources

11 resource types across 5 namespaces. Every type implements full Create,
Read, Update, Delete, List + Status (async polling where applicable).

Coverage versus the Supabase Management API (109 endpoints total):

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
| `SUPABASE::Functions::EdgeFunction` | `/v1/projects/{ref}/functions{,/{slug}}` | Inline JS/TS body. Eszip multipart deploy (`/functions/deploy`) is out of scope. |
| `SUPABASE::Functions::Secret` | `/v1/projects/{ref}/secrets` | Bulk endpoints; modelled as one Forma resource per secret name. Values are write-only — drift on value is invisible. |

### `SUPABASE::Config::*` — per-project singletons

Each singleton is keyed by project ref; payload is opaque `Mapping<String, Any>`. Create + Update both translate to PATCH/PUT (singletons always exist server-side).

| Resource | Endpoint | Method | Payload |
|---|---|---|---|
| `SUPABASE::Config::AuthSettings` | `/v1/projects/{ref}/config/auth` | PATCH | `site_url`, mailer, providers, JWT, rate limits, … (~80 keys) |
| `SUPABASE::Config::APISettings` | `/v1/projects/{ref}/postgrest` | PATCH | `db_schema`, `max_rows`, `db_extra_search_path`, `jwt_secret` |
| `SUPABASE::Config::DatabaseSettings` | `/v1/projects/{ref}/config/database/postgres` | PUT | `statement_timeout`, `max_connections`, shared buffers, etc. |
| `SUPABASE::Config::NetworkRestriction` | `/v1/projects/{ref}/network-restrictions` | PATCH | `dbAllowedCidrs`, `dbAllowedCidrsV6` |

### `SUPABASE::Database::*` — database tier

| Resource | Endpoint | Method | Payload |
|---|---|---|---|
| `SUPABASE::Database::PoolerConfig` | `/v1/projects/{ref}/config/database/pooler` | PATCH | `default_pool_size` (0..3000), `pool_mode` ("transaction" \| "session"). GET returns array of pooler configs (one per database); plugin auto-selects the PRIMARY entry. |

### Discovery + extract

All resources are `discoverable = true` (except the singletons, which surface via their parent project). `formae extract --schema-location local --query 'target:supabase-target' out.pkl` produces a complete PKL representation of an existing project — see [`examples/import-demo/`](examples/import-demo/).

### Roadmap — known gaps

Surface areas that exist in the Management API but are not yet modelled as `SUPABASE::*` resource types. Most are straightforward extensions of the existing plugin architecture.

#### Database management

| Resource (planned) | API endpoint | Status |
|---|---|---|
| `SUPABASE::Database::Backup` | `/v1/projects/{ref}/database/backups`, `/restore`, `/restore-pitr`, `/restore-point`, `/schedule`, `/undo` | not started — multiple verbs, async restore |
| `SUPABASE::Database::Migration` | `/v1/projects/{ref}/database/migrations{,/{version}}` | not started |
| `SUPABASE::Database::Webhook` | `/v1/projects/{ref}/database/webhooks/enable` | not started — toggle resource |
| `SUPABASE::Database::JITAccess` | `/v1/projects/{ref}/database/jit{,/list,/{user_id}}` | not started |
| `SUPABASE::Database::SSLEnforcement` | `/v1/projects/{ref}/ssl-enforcement` | not started |
| `SUPABASE::Database::ReadReplica` | `/v1/projects/{ref}/read-replicas/setup`, `/remove` | not started |
| `SUPABASE::Database::ReadOnlyMode` | `/v1/projects/{ref}/readonly{,/temporary-disable}` | not started |
| `SUPABASE::Database::Password` | `/v1/projects/{ref}/database/password` | not started — rotate-only |
| `SUPABASE::Database::PgSodium` | `/v1/projects/{ref}/pgsodium` | not started |

#### Auth (extra)

| Resource (planned) | API endpoint | Status |
|---|---|---|
| `SUPABASE::Auth::SSOProvider` | `/v1/projects/{ref}/config/auth/sso/providers{,/{id}}` | not started — full CRUD |
| `SUPABASE::Auth::ThirdPartyAuth` | `/v1/projects/{ref}/config/auth/third-party-auth{,/{tpa_id}}` | not started |
| `SUPABASE::Auth::SigningKey` | `/v1/projects/{ref}/config/auth/signing-keys{,/legacy,/{id}}` | not started |
| `SUPABASE::Auth::LegacyAPIKey` | `/v1/projects/{ref}/api-keys/legacy` | not started — only for migration |

#### Project lifecycle / infra

| Resource (planned) | API endpoint | Status |
|---|---|---|
| `SUPABASE::Platform::PauseState` | `/v1/projects/{ref}/pause`, `/restore`, `/restore/cancel` | not started |
| `SUPABASE::Platform::Upgrade` | `/v1/projects/{ref}/upgrade`, `/upgrade/eligibility`, `/upgrade/status` | not started — multi-step state machine |
| `SUPABASE::Platform::CustomHostname` | `/v1/projects/{ref}/custom-hostname{,/activate,/initialize,/reverify}` | not started |
| `SUPABASE::Platform::VanitySubdomain` | `/v1/projects/{ref}/vanity-subdomain{,/activate,/check-availability}` | not started |
| `SUPABASE::Platform::ClaimToken` | `/v1/projects/{ref}/claim-token`, `/v1/oauth/authorize/project-claim` | not started |
| `SUPABASE::Platform::BillingAddon` | `/v1/projects/{ref}/billing/addons{,/{variant}}` | not started — bills the account |
| `SUPABASE::Platform::JITAccess` | `/v1/projects/{ref}/jit-access` | not started |

#### Storage / Realtime

| Resource (planned) | API endpoint | Status |
|---|---|---|
| `SUPABASE::Storage::Bucket` | `GET /v1/projects/{ref}/storage/buckets` | partial — Management API only exposes `GET`; create/delete needs the Storage REST API on the project subdomain |
| `SUPABASE::Storage::Settings` | `/v1/projects/{ref}/config/storage` | not started — singleton |
| `SUPABASE::Realtime::Settings` | `/v1/projects/{ref}/config/realtime{,/shutdown}` | not started — singleton |
| `SUPABASE::Disk::Settings` | `/v1/projects/{ref}/config/disk{,/autoscale,/util}` | not started — singleton |

#### Operations / observability (likely never resources — read-only or one-shots)

These map naturally to dedicated CLI verbs, not to declarative `apply`. Listed for completeness so future contributors know they have been considered:

- `/v1/projects/{ref}/advisors/performance`, `/security` — read-only insights
- `/v1/projects/{ref}/analytics/endpoints/*` — analytics queries
- `/v1/projects/{ref}/health` — health check
- `/v1/projects/{ref}/database/query{,/read-only}`, `/context`, `/openapi`, `/types/typescript` — one-shot RPC, not declarative
- `/v1/projects/{ref}/network-bans{,/retrieve,/retrieve/enriched}` — operations endpoint
- `/v1/projects/{ref}/actions/*` — GitHub-style action runs
- `/v1/projects/{ref}/cli/login-role` — CLI handshake
- `/v1/profile`, `/v1/oauth/*`, `/v1/snippets/*` — user-scoped, not project-scoped resources

If you need any of the unimplemented surface above, contributions follow the existing pattern: drop a new file under `pkg/resources/<namespace>/`, register via `init()`, add a PKL class in `schema/pkl/supabase.pkl`, add conformance fixtures in `testdata/`.

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

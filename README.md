# formae-plugin-supabase

[formae](https://formae.io/) plugin for [Supabase](https://supabase.com/).
Talks to the Supabase Management API at `https://api.supabase.com`.

> Status: feature-complete for the Management API resource set. APIKey
> conformance passes end-to-end live (Create, Verify, Extract, Sync,
> Update, Destroy, OOB Del). Full architectural notes in
> [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Supported Resources

5 resource types across 3 namespaces. Every type implements full Create,
Read, Update, Delete, List + Status (async polling where applicable).

### `SUPABASE::Platform::*` — top-level entities

| Resource | API endpoint | Async | Notes |
|---|---|---|---|
| `SUPABASE::Platform::Project` | `POST /v1/projects`, `GET/PATCH/DELETE /v1/projects/{ref}` | yes | Polls `status` until `ACTIVE_HEALTHY`. Provisioning bills the account and takes 2–3 min. Optional nested config blocks (`auth`, `api`, `database`, `networkRestriction`) — see below. |
| `SUPABASE::Platform::Branch` | `POST /v1/projects/{ref}/branches`, `GET/PATCH/DELETE /v1/branches/{id}` | yes | Polls until `FUNCTIONS_DEPLOYED` / `MIGRATIONS_PASSED`. Paid plan only. |

### `SUPABASE::Auth::*` — credentials

| Resource | API endpoint | Notes |
|---|---|---|
| `SUPABASE::Auth::APIKey` | `/v1/projects/{ref}/api-keys{,/{id}}` | Publishable / secret keys with optional JWT template. `?reveal=true` returns the raw value. Same-plugin DAGs reference `anonKey.res.apiKey`. |

### `SUPABASE::Functions::*` — Edge Functions + secrets

| Resource | API endpoint | Notes |
|---|---|---|
| `SUPABASE::Functions::EdgeFunction` | `/v1/projects/{ref}/functions{,/{slug}}` | Inline JS/TS body. Eszip multipart deploy (`/functions/deploy`) is out of scope. |
| `SUPABASE::Functions::Secret` | `/v1/projects/{ref}/secrets` | Bulk endpoints; modelled as one Forma resource per secret name. Values are write-only — drift on value is invisible. |

### Nested project configuration

Per-project config blocks nested inside `SUPABASE::Platform::Project`. Lifecycle owned by the project — `formae destroy` of the project removes all config server-side. Each block is opaque `Mapping<String, Any>`; the plugin tracks the keys you manage so unmanaged cloud fields don't surface as drift.

| Block | Endpoint | Method | Payload |
|---|---|---|---|
| `Project.auth` (`ProjectAuthConfig`) | `/v1/projects/{ref}/config/auth` | PATCH | `site_url`, mailer, providers, JWT, rate limits, … (~80 keys) |
| `Project.api` (`ProjectAPIConfig`) | `/v1/projects/{ref}/postgrest` | PATCH | `db_schema`, `max_rows`, `db_extra_search_path`, `jwt_secret` |
| `Project.database` (`ProjectDatabaseConfig`) | `/v1/projects/{ref}/config/database/postgres` | PUT | `statement_timeout`, `max_connections`, shared buffers, etc. |
| `Project.networkRestriction` (`ProjectNetworkRestriction`) | `/v1/projects/{ref}/network-restrictions` | PATCH | `dbAllowedCidrs`, `dbAllowedCidrsV6` |

### Discovery + extract

All resources are `discoverable = true`. `formae extract --schema-location local --query 'target:supabase-target' out.pkl` produces a complete PKL representation of an existing project — see [`examples/import-demo/`](examples/import-demo/).

## Configuration

Credentials live in environment variables, not the forma. Create a Personal Access Token at <https://supabase.com/dashboard/account/tokens>:

```bash
export SUPABASE_ACCESS_TOKEN=sbp_xxxxxxxxxxxx
```

A target in your forma carries only deployment metadata. Every field is
optional — a bare `Config {}` plus `SUPABASE_ACCESS_TOKEN` is enough to
deploy and to discover every project the token can see:

```pkl
import "@supabase/supabase.pkl"

new formae.Target {
    label = "supabase-prod"
    config = new supabase.Config {
        baseUrl = null   // defaults to https://api.supabase.com
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
go test -tags=unit ./... # unit tests (41 passing)
make conformance-test    # live API conformance tests (needs SUPABASE_ACCESS_TOKEN)
```

> ⚠️ **`make conformance-test` hits live Supabase.**
> Project + Branch fixtures provision real infrastructure that bills
> the account (Branches require a paid plan; Project create takes
> 2–3 min). Run with `TEST=<prefix>` to scope, e.g.
> `make conformance-test TEST=apikey TIMEOUT=5m`. `scripts/ci/clean-environment.sh`
> runs before and after the suite to delete residue.

## Licensing

Apache-2.0. See [LICENSE](LICENSE) and [CHANGELOG.md](CHANGELOG.md).

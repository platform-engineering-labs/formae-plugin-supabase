# Supabase Plugin for formae

[![CI](https://github.com/platform-engineering-labs/formae-plugin-supabase/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/platform-engineering-labs/formae-plugin-supabase/actions/workflows/ci.yml)
[![Nightly](https://github.com/platform-engineering-labs/formae-plugin-supabase/actions/workflows/nightly.yml/badge.svg?branch=main)](https://github.com/platform-engineering-labs/formae-plugin-supabase/actions/workflows/nightly.yml)

A formae plugin for managing [Supabase](https://supabase.com/) resources via the
Supabase Management API (`https://api.supabase.com`).

## Installation

```bash
make install
```

## Supported Resources

Five resource types across three namespaces. Each implements full Create, Read,
Update, Delete, and List, with async Status polling where the API is
asynchronous.

| Resource Type | Description |
|---------------|-------------|
| `SUPABASE::Platform::Project` | A Supabase project. Async create (polls to `ACTIVE_HEALTHY`); provisioning bills the account and takes ~2–3 min. Carries optional nested config blocks (see below). |
| `SUPABASE::Platform::Branch` | A preview branch on a project. Async; requires a paid plan. |
| `SUPABASE::Auth::APIKey` | A publishable or secret project API key. Referenced by other resources via `.res.apiKey`. |
| `SUPABASE::Functions::EdgeFunction` | An Edge Function deployed from an inline JS/TS body. |
| `SUPABASE::Functions::Secrets` | All of a project's Edge Function secrets as one bag (`values` is a name→value map). The API has no per-secret endpoint, so the whole bag is one resource and every change is one atomic bulk write. Values are write-only. |

### Nested project configuration

Per-project config is nested inside `SUPABASE::Platform::Project` rather than
modelled as standalone resources. Its lifecycle is owned by the project, so
`formae destroy` of the project removes the config server-side. Each block is an
opaque `Mapping<String, Any>`; the plugin tracks the keys you manage so
unmanaged cloud fields don't surface as drift.

| Block | Endpoint |
|-------|----------|
| `Project.auth` | `PATCH /v1/projects/{ref}/config/auth` |
| `Project.api` | `PATCH /v1/projects/{ref}/postgrest` |
| `Project.database` | `PUT /v1/projects/{ref}/config/database/postgres` |
| `Project.networkRestriction` | `PATCH /v1/projects/{ref}/network-restrictions` |

## Configuration

Configure a target in your forma file. Every field is optional — a bare
`Config {}` is enough to deploy and to discover every project the token can see:

```pkl
import "@supabase/supabase.pkl"

new formae.Target {
  label = "supabase"
  config = new supabase.Config {}
}
```

### Credentials

The plugin reads a Supabase Personal Access Token from the environment. Create
one at <https://supabase.com/dashboard/account/tokens>:

| Variable | Description |
|----------|-------------|
| `SUPABASE_ACCESS_TOKEN` | Personal Access Token (`sbp_…`) |

Set it before starting the formae agent.

## Examples

Each example is a self-contained Pkl project — `cd` in or pass the path to
`formae apply`.

| Example | Shows | Needs |
|---------|-------|-------|
| [`full-project/`](examples/full-project/) | Whole stack from one apply: Project + nested config, API keys, secret, edge function wired via `project.res.id` | `SUPABASE_ORG_ID`, `SUPABASE_DB_PASS` |
| [`branching/`](examples/branching/) | Preview environments: persistent develop branch + git-tracked ephemeral branch | `SUPABASE_PROJECT_REF` (paid plan) |
| [`edge-secrets/`](examples/edge-secrets/) | Edge function + the write-only secrets it reads | `SUPABASE_PROJECT_REF` |
| [`basic/`](examples/basic/) | Smallest possible forma: one edge function | `SUPABASE_PROJECT_REF` |
| [`discover/`](examples/discover/) | Bare target; agent discovers every project the PAT can see | — |
| [`import-demo/`](examples/import-demo/) | `formae extract`: adopt an existing project as PKL | `SUPABASE_ACCESS_TOKEN` only |

All examples additionally require `SUPABASE_ACCESS_TOKEN`.

```bash
formae apply --mode reconcile --watch examples/full-project/main.pkl
```

## Development

### Prerequisites

- Go 1.26+
- [Pkl CLI](https://pkl-lang.org/main/current/pkl-cli/index.html)
- A Supabase Personal Access Token (for conformance testing)

### Building

```bash
make build      # Build plugin binary
make test       # Run unit tests
make lint       # Run linter
make install    # Build + install locally
```

### Local Testing

```bash
# Install plugin locally
make install

# Start formae agent (token in env)
SUPABASE_ACCESS_TOKEN=sbp_xxx formae agent start

# Apply example resources
formae apply --mode reconcile examples/basic/main.pkl
```

### Conformance Testing

Run the full CRUD lifecycle + discovery tests:

```bash
make conformance-test                  # Latest formae version
make conformance-test VERSION=0.80.0   # Specific version
make conformance-test TEST=apikey      # Scope to one resource type
```

> ⚠️ **`make conformance-test` hits live Supabase.** Project and Branch
> fixtures provision real infrastructure that bills the account (Branches
> require a paid plan; Project create takes 2–3 min). Scope with `TEST=<prefix>`
> to limit cost. `scripts/ci/clean-environment.sh` runs before and after the
> suite to delete residue.

## License

Apache-2.0. See [LICENSE](LICENSE) and [CHANGELOG.md](CHANGELOG.md).

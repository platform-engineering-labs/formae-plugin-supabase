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
| `SUPABASE::Auth::APIKey` | A publishable or secret project API key. A first-class secret resource: reference its key material as `.res.secretValue` (or `.res.apiKey`), re-read live from the Management API on every plugin call. |
| `SUPABASE::Functions::EdgeFunction` | An Edge Function deployed from an inline JS/TS body. |
| `SUPABASE::Functions::Secrets` | All of a project's Edge Function secrets as one bag (`values` is a name→value map). The API has no per-secret endpoint, so the whole bag is one resource and every change is one atomic bulk write. Values are write-only, and each entry accepts a generator output or a secret reference in place of a literal. |

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

### Secrets and generated credentials

Requires formae 0.89.0 or newer.

`SUPABASE::Auth::APIKey` is a first-class secret resource. Its key material is
reachable through the uniform accessor, and the agent re-reads it from the
Management API on every plugin call, so a key rotated out of band takes effect
without restarting the agent:

```pkl
local backendKey = new supabase.APIKey {
  label = "backend-key"
  projectRef = project.res.id
  apikey_type = "secret"
  name = "backend_secret"
}
backendKey

new supabase.Secrets {
  label = "edge-secrets"
  projectRef = project.res.id
  values {
    // A reference, not a literal — resolved on apply, re-read live after.
    ["BACKEND_API_KEY"] = backendKey.res.secretValue
  }
}
```

Formae can also draw credentials for you. `Project.dbPass` and every
`Secrets.values` entry accept a generator output:

```pkl
local dbPassGen = new formae.PasswordGenerator {
  label = "db-password"
  stack = appStack.res
  length = 24
}
dbPassGen

new supabase.Project {
  label = "app"
  // ...
  dbPass = dbPassGen.gen.value
}
```

A generator with no `rotation` draws once and never changes — the replacement
for minting a password at evaluation time and pinning it with `setOnce`. Give
it a `rotation` cadence and it turns the credential over unattended, moving
every bound property with it. Two caveats specific to this plugin:

- `Project.dbPass` is `createOnly`. Do not attach a rotation cadence to a
  generator bound there; rotating an immutable field would replace the project.
  `Secrets.values` entries are mutable, so a cadence is fine on those.
- `Secrets.values` entries are **not** hashed at rest. Formae derives a field's
  opacity from its declared type and does not descend into map value positions,
  so bag entries are stored unhashed in agent state. `Project.dbPass` and
  `APIKey.apiKey` are hashed.

## Examples

Each example is a self-contained Pkl project — `cd` in or pass the path to
`formae apply`.

| Example | Shows | Needs |
|---------|-------|-------|
| [`full-project/`](examples/full-project/) | Whole stack from one apply: Project + nested config, API keys, secret, edge function wired via `project.res.id`. DB password is drawn by a `formae.PasswordGenerator`; the secret key's value is wired into the secret bag by reference | `SUPABASE_ORG_ID` |
| [`branching/`](examples/branching/) | Preview environments: persistent develop branch + git-tracked ephemeral branch | `SUPABASE_PROJECT_REF` (paid plan) |
| [`edge-secrets/`](examples/edge-secrets/) | Edge function + the write-only secrets it reads, one of them a generator-drawn signing secret rotating every 30 days | `SUPABASE_PROJECT_REF` |
| [`basic/`](examples/basic/) | Smallest possible forma: one edge function | `SUPABASE_PROJECT_REF` |
| [`discover/`](examples/discover/) | Bare target; agent discovers every project the PAT can see | — |

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
make conformance-test VERSION=0.89.0   # Specific version
make conformance-test TEST=apikey      # Scope to one resource type
```

> ⚠️ **`make conformance-test` hits live Supabase.** Project and Branch
> fixtures provision real infrastructure that bills the account (Branches
> require a paid plan; Project create takes 2–3 min). Scope with `TEST=<prefix>`
> to limit cost. `scripts/ci/clean-environment.sh` runs before and after the
> suite to delete residue.

## License

This plugin is licensed under the [Functional Source License, Version 1.1, ALv2
Future License (FSL-1.1-ALv2)](LICENSE). See also [CHANGELOG.md](CHANGELOG.md).

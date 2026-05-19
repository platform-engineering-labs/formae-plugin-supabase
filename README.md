# formae-plugin-supabase

[formae](https://formae.io/) plugin for [Supabase](https://supabase.com/). Talks
to the Supabase Management API at `https://api.supabase.com`.

> Status: early. P1 surface (`SUPABASE::Platform::Project`) is implemented
> end-to-end with unit tests. Branches, Edge Functions, API keys, and Storage
> Buckets are in the roadmap — see [docs/RESOURCES.md](docs/RESOURCES.md).

## Supported Resources

| Resource Type | CRUD | Async | Notes |
|---|---|---|---|
| `SUPABASE::Platform::Project` | C, R, U, D, L | yes | Polls until `ACTIVE_HEALTHY` |

Roadmap: see [docs/RESOURCES.md](docs/RESOURCES.md).
Design: see [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Configuration

The plugin reads credentials from the environment, not the forma. Create a
Personal Access Token at
<https://supabase.com/dashboard/account/tokens> and export it before
running `formae`:

```bash
export SUPABASE_ACCESS_TOKEN=sbp_xxxxxxxxxxxx
```

A target in your forma carries only deployment metadata:

```pkl
import "@supabase/supabase.pkl"

new formae.Target {
    label = "supabase-prod"
    namespace = "SUPABASE"
    config = new supabase.Config {
        organizationId = "your-org-slug-or-id"
    }
}
```

`baseUrl` may be set to point at a non-production Supabase instance; it
defaults to `https://api.supabase.com`.

## Example

```pkl
import "@formae/formae.pkl"
import "@supabase/supabase.pkl"

new formae.Forma {
    resources {
        new supabase.Project {
            name = "demo"
            organizationId = "your-org"
            region = "us-east-1"
            dbPass = read("env:SUPABASE_DB_PASS")
            plan = "free"
        }
    }
}
```

## Development

```bash
make build               # build plugin binary
make install             # build + install to ~/.pel/formae/plugins
go test -tags=unit ./... # unit tests
make conformance-test    # live API conformance tests (needs SUPABASE_ACCESS_TOKEN)
```

## Licensing

Apache-2.0.

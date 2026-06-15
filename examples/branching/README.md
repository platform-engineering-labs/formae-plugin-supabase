# Preview environments with branches

Supabase branches as code: a persistent `develop` branch for shared testing
plus an ephemeral preview branch tracking a Git feature branch. Each branch
is a complete Supabase stack (own Postgres, auth, edge functions) cloned
from the parent project.

> **Requires a paid plan.** Branching is available on Pro and above; the
> parent project must be on a paid plan or the API rejects branch creation.

## Prerequisites

| Variable | Description |
|----------|-------------|
| `SUPABASE_ACCESS_TOKEN` | Personal Access Token (`sbp_…`) |
| `SUPABASE_PROJECT_REF` | Ref of an existing paid-plan project to branch from |

## Run it

```bash
export SUPABASE_ACCESS_TOKEN=sbp_xxx
export SUPABASE_PROJECT_REF=abcdefghijklmnop
formae apply --mode reconcile --watch examples/branching/main.pkl
```

Branch creation is async; the plugin polls until each branch reaches
`FUNCTIONS_DEPLOYED` / `MIGRATIONS_PASSED`.

## The workflow this models

1. `develop` stays up permanently (`persistent = true`).
2. For each feature, copy the preview-branch block, point `git_branch` at
   the feature branch, apply.
3. When the feature merges, delete the block and reconcile — formae tears
   the preview branch down.

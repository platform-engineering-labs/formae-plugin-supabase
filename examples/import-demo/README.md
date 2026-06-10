# Demo: import an existing Supabase project as code

**Pitch for the call**: most Supabase customers already have one or more
projects configured via the dashboard. They click around to change auth
settings, rotate API keys, edit Postgres tuning, manage edge function
secrets, etc. There's no audit trail, no review process, no diff between
environments. formae fixes that by **adopting** an existing project as
PKL — no migration, no replatform, no greenfield rewrite.

## The 60-second demo

### Step 1: nothing on disk yet

```bash
ls
# (empty)
```

The customer's Supabase project lives at https://app.supabase.com/project/$REF.
Every setting was clicked in via the dashboard.

### Step 2: run extract

```bash
export SUPABASE_ACCESS_TOKEN=sbp_...
formae extract \
  --schema-location local \
  --query 'target:supabase-target' \
  project.pkl
```

Output: a single PKL file describing every managed Supabase resource on the
project — the project itself with its auth / PostgREST / Postgres / network
config nested inside it, plus API keys, edge functions, and secrets.

This is `project.pkl` in this directory — a redacted capture from a live
free-tier Supabase project.

### Step 3: open the file

It's plain PKL — diff-able, reviewable, version-controllable. Per-project
config nests inside the `Project` resource:

```pkl
new supabase.Project {
    label = "REDACTED_PROJECT_REF"
    name = "my-project"
    organizationId = "REDACTED_ORG_ID"
    region = "us-east-1"

    api = new supabase.ProjectAPIConfig {
        settings = new Mapping {
            ["db_schema"] = "public,graphql_public"
            ["max_rows"] = 1000
        }
    }

    auth = new supabase.ProjectAuthConfig {
        settings = new Mapping {
            ["site_url"] = "http://localhost:3000"
            ["disable_signup"] = false
            // … and many more knobs
        }
    }
}
```

### Step 4: change one thing

Edit `project.pkl`:

```diff
-    ["max_rows"] = 1000
+    ["max_rows"] = 5000
```

### Step 5: apply

```bash
formae apply --mode reconcile --yes project.pkl
```

formae diffs the desired state vs. the live state on Supabase, makes only
the necessary PATCH call, returns Success in ~2 seconds.

Refresh the Supabase dashboard → API Settings → max rows is now 5000.

### Step 6: drift

Click around in the Supabase dashboard. Change `max_rows` back to 1000.

```bash
formae apply --mode reconcile --yes project.pkl
```

formae detects the drift, re-applies 5000. The PKL file is the source of
truth.

## Why customers care

| Pain point in vanilla Supabase | What this demo shows |
|---|---|
| "Two devs touched the same auth config, who changed what?" | git blame on `project.pkl` |
| "Staging behaves differently from prod" | One PKL, two `Target`s, identical configs |
| "We need to rotate the JWT secret across 12 projects" | Loop a PKL template; `formae apply` |
| "What's our actual current state?" | `formae extract` — answer is a file |
| "How do we onboard a new project to our standards?" | Apply the shared base PKL |

## Why this is interesting for Supabase specifically

- Hosted Supabase has no Terraform-style first-class config-as-code story; the [official Terraform provider](https://github.com/supabase/terraform-provider-supabase) exists but covers a narrower surface and has no discovery / extract.
- Supabase customers that scale into multi-project setups (one per
  customer, one per env) currently maintain their own scripts. formae
  collapses that into a single declarative file per project.
- formae's plugin architecture means new Supabase resources (Pooler
  config, Storage buckets, Branches, SSO providers, etc.) can be added
  without forking formae itself.

## Files in this directory

| File | Purpose |
|---|---|
| `project.pkl` | Redacted extracted state from a free-tier project — the `Project` with nested config plus its API keys, edge function, and secret. |
| `PklProject` | PKL deps (local supabase plugin + formae hub package). |

## Running it yourself

`project.pkl` ships with `REDACTED_*` placeholders, so **don't apply it
verbatim** — `organizationId`/`projectRef` won't match a real project and the
apply would try to create a new one. Extract your *own* project first (below),
then apply that:

```bash
cd examples/import-demo
pkl project resolve         # one-time
export SUPABASE_ACCESS_TOKEN=sbp_...
formae extract --schema-location local --query 'target:supabase-target' project.pkl
formae apply --mode reconcile --yes project.pkl
```

## Recreating the extract from scratch

```bash
formae extract \
  --schema-location local \
  --query 'target:supabase-target' \
  project.pkl
```

Adjust the query to filter by stack, type, label etc.

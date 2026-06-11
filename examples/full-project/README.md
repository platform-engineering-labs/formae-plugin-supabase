# Full project from scratch

The whole plugin surface in one apply: a Supabase **Project** (with nested
auth, Data API, and Postgres configuration), two **API keys**, an
edge-function **Secret**, and an **Edge Function** that reads it.

Child resources reference the freshly created project through
`project.res.id` — formae resolves the generated project ref at apply
time, so nothing exists before you run this and no second apply is needed.

## Prerequisites

| Variable | Description |
|----------|-------------|
| `SUPABASE_ACCESS_TOKEN` | Personal Access Token (`sbp_…`) — create at <https://supabase.com/dashboard/account/tokens> |
| `SUPABASE_ORG_ID` | Organization slug or id that will own the project |
| `SUPABASE_DB_PASS` | Initial Postgres password (min 8 chars, write-only) |
| `OPENAI_API_KEY` | Optional — demo fallback value is used when unset |

## Run it

```bash
export SUPABASE_ACCESS_TOKEN=sbp_xxx
export SUPABASE_ORG_ID=your-org
export SUPABASE_DB_PASS='a-strong-password'
formae apply --mode reconcile --watch examples/full-project/main.pkl
```

Project creation is **async** — the plugin polls until the project reaches
`ACTIVE_HEALTHY`, typically ~2 minutes. `--watch` shows the progress.
Children apply automatically once the project ref resolves.

Then check <https://supabase.com/dashboard>: the `formae-full-demo` project
has the two API keys (Settings → API Keys), the `OPENAI_API_KEY` function
secret, and the `ask-ai` edge function. Invoke the function to prove the
secret is wired through:

```bash
curl https://<project-ref>.supabase.co/functions/v1/ask-ai \
  -H "Authorization: Bearer <publishable key>"
# {"openaiKeyPrefix":"sk-demo"}
```

## Good to know

- **Free-tier cap:** 2 projects per organization. Tear down when done:
  `formae destroy examples/full-project/main.pkl`
- **Write-only fields:** `dbPass` and the secret `value` are never returned
  by the Supabase API, so formae cannot detect drift on them.
- The nested `auth` / `api` / `database` blocks PATCH the project's
  configuration after creation; only the keys you set are managed — other
  dashboard settings won't show up as drift.

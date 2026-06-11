# Edge function + secrets

The everyday deploy unit: an edge function and the secrets it reads, managed
together. Two `Functions::Secret`s and a webhook-handler `EdgeFunction`
consuming them via `Deno.env.get()`, deployed to an existing project.

## Prerequisites

| Variable | Description |
|----------|-------------|
| `SUPABASE_ACCESS_TOKEN` | Personal Access Token (`sbp_…`) |
| `SUPABASE_PROJECT_REF` | Ref of an existing project |
| `OPENAI_API_KEY` | Optional — demo fallback value when unset |
| `WEBHOOK_SECRET` | Optional — demo fallback value when unset |

## Run it

```bash
export SUPABASE_ACCESS_TOKEN=sbp_xxx
export SUPABASE_PROJECT_REF=abcdefghijklmnop
formae apply --mode reconcile --watch examples/edge-secrets/main.pkl
```

Test the function (note `verify_jwt = false` — webhook senders don't carry
Supabase JWTs, the signing secret is the auth mechanism):

```bash
curl -X POST https://$SUPABASE_PROJECT_REF.supabase.co/functions/v1/webhook-handler \
  -H "x-webhook-signature: whsec-demo-not-a-real-secret"
# {"ok":true,"openAIConfigured":true}
```

## Good to know

- **Secret names must not start with `SUPABASE_`** — the API reserves that
  prefix.
- **Values are write-only.** The API never returns secret values, so formae
  detects drift on secret *existence* but not on the value itself. Rotating
  a value: change it in the forma and reconcile — formae re-sends it.
- Function `body` is also never returned by the API; the deployed source is
  tracked by formae's own state, not by reading it back.

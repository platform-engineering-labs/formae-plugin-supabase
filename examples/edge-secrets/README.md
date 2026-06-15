# Edge function + secrets

The everyday deploy unit: an edge function and the secrets it reads, managed
together. One `Functions::Secrets` bag (two secrets in its `values` map) and a
webhook-handler `EdgeFunction` consuming them via `Deno.env.get()`, deployed to
an existing project.

A project's secrets are a single bag server-side — the API has no per-secret
endpoint — so all of them live in one `Secrets` resource and every change is
one atomic bulk write.

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
  can't detect drift on a value. Rotating a value: change it in the `values`
  map and reconcile — formae re-sends the whole bag in one POST.
- **Removing a key** from the `values` map deletes that secret on the next
  reconcile; the bag is the full declaration of the project's managed secrets.
- Function `body` is also never returned by the API; the deployed source is
  tracked by formae's own state, not by reading it back.

# k8s ↔ Supabase cross-plugin demo

A single `forma.pkl` that:

1. Mints a publishable API key on an existing Supabase project (`SUPABASE::Auth::APIKey`).
2. Stamps a k8s `Secret` carrying `NEXT_PUBLIC_SUPABASE_URL` + `NEXT_PUBLIC_SUPABASE_ANON_KEY`. **Today the anon key is read from `$SUPABASE_ANON_KEY` at apply time**; once the supabase plugin's PKL schema exposes `APIKey.api_key` as a `formae.Resolvable`, this will become a direct cross-plugin reference (see TODO in `forma.pkl`).
3. Runs a tiny Go app in k8s (`Deployment` + `Service` + `Ingress`) that hits Supabase's Auth `/settings` endpoint at request time. Visit it and you see the live JSON response — end-to-end proof.

> Per-project Auth config (e.g. `site_url`) is no longer a standalone resource —
> it nests inside `SUPABASE::Platform::Project`. This demo uses an existing
> project by ref and doesn't manage it, so set `site_url` in the dashboard if
> your redirect flow needs it.

```
┌──────────────────┐   formae apply   ┌──────────────────────┐
│ supabase plugin  │ ───────────────► │ Supabase project     │
│  - APIKey        │                  │  - publishable key   │
└────────┬─────────┘                  └──────────────────────┘
         │ anonKey.api_key
         ▼
┌──────────────────┐   formae apply   ┌──────────────────────┐
│ k8s plugin       │ ───────────────► │ kubernetes cluster   │
│  - Secret        │                  │  Deployment + Service│
│  - Deployment    │                  │  Ingress             │
└──────────────────┘                  └──────────────────────┘
```

## Prerequisites

- formae CLI (`v0.85+`)
- `formae-plugin-supabase` installed (`make install` from the repo root)
- `formae-plugin-k8s` installed (`v0.1.1+`)
- A Supabase project (free tier OK) — see the repo's main `README.md`
- A k8s cluster: orbstack / kind / minikube / EKS / GKE — anything `~/.kube/config` points at
- Container registry the cluster can pull from (ghcr, dockerhub, ECR)

## 1. Build the demo image

For a public cluster:

```bash
cd examples/k8s-supabase
docker build -t ghcr.io/YOUR-ORG/k8s-supabase-demo:latest .
docker push  ghcr.io/YOUR-ORG/k8s-supabase-demo:latest
```

For a local **kind** cluster (no push needed):

```bash
cd examples/k8s-supabase
docker build -t k8s-supabase-demo:latest .
kind load docker-image k8s-supabase-demo:latest --name YOUR-KIND-CLUSTER
```

The forma sets `imagePullPolicy: IfNotPresent` so the kubelet uses the loaded image instead of trying to pull.

## 2. Set environment

```bash
export SUPABASE_ACCESS_TOKEN=sbp_xxx
export SUPABASE_ORGANIZATION_ID=your-org-slug
export SUPABASE_PROJECT_REF=your-project-ref
export SUPABASE_ANON_KEY=$(curl -s -H "Authorization: Bearer $SUPABASE_ACCESS_TOKEN" \
    "https://api.supabase.com/v1/projects/$SUPABASE_PROJECT_REF/api-keys?reveal=true" \
    | jq -r '.[] | select(.type == "publishable") | .api_key' | head -1)
export DEMO_IMAGE=k8s-supabase-demo:latest   # or ghcr.io/YOUR-ORG/... for a public cluster
export INGRESS_HOST=supabase-demo.local      # optional, defaults to this
```

## 3. Apply

The agent inherits its env from the process that launched it, so restart the agent if `SUPABASE_ACCESS_TOKEN` isn't already in its env:

```bash
formae agent stop
formae agent start
```

Then:

```bash
formae apply --mode reconcile --yes forma.pkl
```

formae will:

1. Create `SUPABASE::Auth::APIKey` and capture the secret value.
2. Create the k8s namespace, secret (with the API key from step 1), deployment, service, ingress.

## 4. Hit it

For local clusters, `/etc/hosts` or use `kubectl port-forward`:

```bash
kubectl -n supabase-demo port-forward svc/supabase-demo 8080:80
curl http://localhost:8080
```

You should see:

```
k8s <> supabase demo
====================
SUPABASE_URL: https://xxx.supabase.co
ANON_KEY prefix: sb_publish…

live Supabase Auth /settings response:
{
  "external": { ... },
  "disable_signup": false,
  ...
}
```

The JSON proves the k8s pod successfully authenticated against Supabase using the credentials that the supabase plugin minted in the same `forma apply`.

## 5. Tear down

```bash
formae destroy forma.pkl
```

Deletes the k8s namespace (cascades to deployment/service/ingress/secret) and revokes the Supabase API key. Auth settings + organization remain.

## Files

| Path | Purpose |
|---|---|
| `forma.pkl` | The cross-plugin forma (~140 lines) |
| `PklProject` | PKL dependency wiring — points at local supabase + installed k8s schemas |
| `Dockerfile` | Multi-stage build for the demo container |
| `app/main.go` | The minimal Go server (~80 lines) |

## Troubleshooting

- **"could not resolve `@k8s/...`"** — verify `~/.pel/formae/plugins/k8s/v0.1.1/` exists. Adjust the version in `PklProject` if your installed k8s plugin differs.
- **`ImagePullBackOff`** — your cluster cannot pull `DEMO_IMAGE`. Either push to a public registry or add an `imagePullSecrets` to the deployment.
- **Auth /settings returns 401** — the publishable key from Supabase is not propagating. Check `kubectl -n supabase-demo get secret supabase-creds -o yaml` and confirm `NEXT_PUBLIC_SUPABASE_ANON_KEY` is base64-decoded to a `sb_publishable_...` value, not empty or the string literal `${anonKey.api_key}`.

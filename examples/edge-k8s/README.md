# edge-k8s — Supabase Edge Function called from a k8s pod

End-to-end cross-plugin demo on the **post-refactor** schema. One
`formae apply` mints a Supabase API key, deploys an Edge Function,
seeds a k8s `Secret`, and rolls a Go pod that calls the function on
every HTTP request.

```
┌──────────────────────┐  formae apply  ┌────────────────────────┐
│ supabase plugin      │ ─────────────► │ Supabase project (existing)
│  - Auth::APIKey      │                │  - publishable key      │
│  - Functions::Edge…  │                │  - edge function live   │
└──────────┬───────────┘                └────────────────────────┘
           │ anonKey.api_key   (env shim today, Resolvable later)
           ▼
┌──────────────────────┐  formae apply  ┌────────────────────────┐
│ k8s plugin           │ ─────────────► │ kubernetes cluster      │
│  - Core::Secret      │                │  Deployment + Service   │
│  - Apps::Deployment  │                │  Ingress                │
│  - Networking::…     │                │  pod hits edge function │
└──────────────────────┘                └────────────────────────┘
```

## Prerequisites

| Tool / asset            | Notes                                                                |
|-------------------------|----------------------------------------------------------------------|
| `formae` CLI            | `v0.85+` (`formae --version`)                                        |
| `formae-plugin-supabase`| `make install` from this repo root                                   |
| `formae-plugin-k8s`     | `make install` from `../formae-plugin-k8s` (installs `v0.1.1+`)      |
| Supabase project        | Any tier; supply its ref via `SUPABASE_PROJECT_REF`                  |
| k8s cluster             | `~/.kube/config` default context — kind / orbstack / EKS / GKE / AKS |
| Container registry      | Or `kind load docker-image` for local-only                           |

If the installed k8s plugin version isn't `v0.1.1`, edit the path in
`PklProject` to match.

## 1. Build the image

```bash
cd examples/edge-k8s
make image                          # docker build -t edge-k8s-demo:latest .
make kind-load KIND_CLUSTER=mycluster   # OR push to a registry
```

## 2. Set env

```bash
cp demo.env.example ~/.edge-k8s.env  # don't edit the in-repo copy
$EDITOR ~/.edge-k8s.env
set -a; . ~/.edge-k8s.env; set +a
```

Required: `SUPABASE_ACCESS_TOKEN`, `SUPABASE_PROJECT_REF`,
`SUPABASE_ORGANIZATION_ID`, `DEMO_IMAGE`.

The formae agent inherits env from the process that launched it. If
the agent's already running, restart so it picks up new values:

```bash
formae agent stop && formae agent start
```

## 3. Apply

```bash
make apply
```

`make apply` runs the [two-phase apply](#why-two-phase) automatically:

1. **Phase 1** — `formae apply` with an empty `SUPABASE_ANON_KEY` env
   var. The supabase plugin creates the `Auth::APIKey` resource
   (returns the freshly-minted publishable key). k8s resources land
   too but the `Secret`'s `SUPABASE_ANON_KEY` field is blank.
2. **Fetch the key** — `make anon-key` calls the Supabase REST API
   `GET /v1/projects/{ref}/api-keys?reveal=true` and prints the value.
3. **Phase 2** — re-applies with `SUPABASE_ANON_KEY` populated. The k8s
   `Secret` updates, pod rolls a new replica with the right env.

If you'd rather drive it by hand:

```bash
make apply-phase1
export SUPABASE_ANON_KEY=$(make anon-key)
make apply-phase2
```

## 4. Hit it

```bash
# in one terminal:
make port-forward       # kubectl -n edge-k8s-demo port-forward svc/edge-k8s-demo 8080:80

# in another:
make curl               # curl http://localhost:8080
```

Expected output:

```
edge-k8s-demo
=============
supabase url: https://xxxxxxxxxxxxxxx.supabase.co
function slug: edge_k8s_demo_hello
anon key prefix: sb_publishable_...

edge function HTTP 200:
{"message":"hello from supabase edge function","slug":"edge_k8s_demo_hello","ts":"2026-06-08T12:34:56.000Z"}
```

The `200` + JSON body prove the k8s pod authenticated against Supabase
using the credential the supabase plugin minted in the same apply.

## 5. Tear down

```bash
make destroy
```

Deletes the k8s namespace (cascades the deployment / service / ingress
/ secret) and revokes the Supabase API key + edge function. The
existing project is **not** touched — we never owned it.

## Why two-phase?

`SUPABASE::Auth::APIKey` exposes a `Resolvable` (`anonKey.res.apiKey`)
so within the supabase plugin you'd reference it directly. The k8s
plugin's `Core::Secret.stringData` is typed `Mapping<String, String>`
without a Resolvable extension, so cross-plugin references don't
resolve at apply time today. The env shim is the workaround until
either schema relaxes.

When that happens, the forma collapses to a single apply:

```pkl
new ksecret.Secret {
  …
  stringData {
    ["SUPABASE_ANON_KEY"] = anonKey.res.apiKey   // ← real cross-plugin DAG
  }
}
```

## Files

| Path              | Purpose                                                       |
|-------------------|---------------------------------------------------------------|
| `forma.pkl`       | The cross-plugin forma (~170 lines, post-refactor schema)     |
| `PklProject`      | Local supabase + installed k8s + hub formae deps              |
| `app/main.go`     | Tiny Go server that calls the edge function                   |
| `Dockerfile`      | Multi-stage Go → distroless image                             |
| `Makefile`        | `make image / kind-load / apply / destroy`                    |
| `demo.env.example`| Env template (copy out of repo, fill in, source)              |

## Troubleshooting

- **`Could not find module @k8s/...`** — version mismatch. Check
  `~/.pel/formae/plugins/k8s/` for the actually-installed version and
  update `PklProject`.
- **`ImagePullBackOff`** — your cluster can't pull `DEMO_IMAGE`. Push
  to a public registry or `make kind-load` for local clusters.
- **`HTTP 401` from the edge function** — `verify_jwt = false` in the
  forma should disable auth, but if it slipped to `true` or you reused
  an old slug, the publishable key path won't match. `kubectl -n edge-k8s-demo get secret supabase-creds -o yaml`
  and confirm `SUPABASE_ANON_KEY` base64-decodes to `sb_publishable_…`.
- **Pod restart loop** — check `kubectl -n edge-k8s-demo logs deploy/edge-k8s-demo`.
  The Go binary `log.Fatal`s when required env vars are empty (most
  likely `SUPABASE_ANON_KEY` from a stale phase-1 secret); re-run
  `make apply` to refresh.

# Talking points for the Supabase call

## Opening (30 sec)

"We built a formae plugin that talks to your Management API. The
interesting part isn't that we can _create_ Supabase resources — anyone
can curl your API. The interesting part is `formae extract`: we suck an
entire existing Supabase project out as code, then put it under version
control."

## The killer demo (2 min)

1. Open Supabase dashboard, point at the live project.
2. Run `formae extract --query 'target:supabase-target' project.pkl`
3. Open `project.pkl` — show ~700 lines covering APIKey, AuthSettings,
   APISettings, DatabaseSettings, NetworkRestriction, Organization,
   EdgeFunction.
4. Diff one line, `formae apply` — change live in 2 seconds.
5. Click revert in dashboard. `formae apply` again — drift gone.

## The "why we" pitch (1 min)

Three things hosted Supabase doesn't currently solve:

1. **Multi-project parity** — customers running per-tenant projects have
   no way to enforce identical config across 100 projects. formae loops
   a template.
2. **Audit / review** — dashboard clicks have no PR review. formae's
   PKL is plain text, lives in git.
3. **Disaster recovery / cloning** — `formae extract` then re-`apply`
   to a different project ref = config clone.

## Plugin coverage today

Implemented + tested live:

- `SUPABASE::Platform::Project` (async create, ~3-min poll)
- `SUPABASE::Platform::Branch` (async, paid plans only)
- `SUPABASE::Platform::Organization` (read + create)
- `SUPABASE::Auth::APIKey` (CRUD; survived OOB delete + sync)
- `SUPABASE::Functions::EdgeFunction` (CRUD, inline body)
- `SUPABASE::Functions::Secret` (bulk endpoints, write-only values)
- `SUPABASE::Config::AuthSettings` (singleton PATCH)
- `SUPABASE::Config::APISettings` (singleton PATCH)
- `SUPABASE::Config::DatabaseSettings` (singleton PUT)
- `SUPABASE::Config::NetworkRestriction` (singleton PATCH)

Architecture mirrors the K8s plugin: per-namespace subpackages,
self-registration via `init()`, slim main dispatcher. ~2000 LoC Go +
PKL.

## Asks for Supabase

Things that would make this much sharper:

1. **Branch API GA**: today branches need the paid plan and the API is
   labelled experimental in places. Customers on free tier can't
   demo branches.
2. **Field-level resolvable outputs**: Supabase's Management API
   responses sometimes flatten or rename fields between Create and Get
   (e.g. `apikey_type` vs `type`); we wrote dual structs to bridge it.
   Stable response shapes would let us drop ~200 LoC.
3. **Bulk endpoint shapes that don't lie about cardinality**: secrets
   POST/DELETE are array-only, no single-item RESTful pattern. We
   model one Forma resource per secret and wrap arrays of size 1.
4. **Storage buckets via the Management API**: today only `GET` is
   exposed there; create/delete requires the Storage REST API on the
   project subdomain. Inconsistent surface.

## Anti-pitch (be honest)

- We're not running Supabase _internals_ — the plugin treats Supabase
  as a black-box HTTP API. We do not manage the underlying VPC,
  primaries, replicas, etc.
- For greenfield "give me a Supabase project from scratch", the
  bottleneck is Supabase's own create-project API which is async and
  bills the user. We're not faster than the dashboard for that one
  case.
- We don't replace the dashboard. People still want a UI for
  ad-hoc inspection. formae complements the dashboard rather than
  replacing it.

## If they ask "why not Terraform?"

- Their official TF provider exists and works; we read the OpenAPI to
  build ours, so feature surface overlaps.
- formae's edge here is the extract pipeline, async-aware reconcile
  loop, and the fact that you can drop Supabase + k8s + AWS + GCP +
  K8s into a single forma file with cross-plugin Resolvables.
- If a customer is happy with TF, they should use TF. formae is for
  customers who want one tool across the whole stack.

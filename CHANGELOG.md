# Changelog

All notable changes to `formae-plugin-supabase` are recorded here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
versions follow [SemVer](https://semver.org/).

## [Unreleased]

### Changed

- **BREAKING:** `SUPABASE::Functions::Secret` (one resource per secret name)
  is replaced by `SUPABASE::Functions::Secrets` — one bag resource per
  project holding a `values` name→value map, identified by `$.projectRef`.
  The Supabase API exposes only a bulk secrets endpoint with no per-secret
  operation; the old per-name model issued concurrent single-item writes that
  raced on the shared bag (read-modify-write, last writer wins) and silently
  dropped secrets during a multi-secret apply. The bag model makes every
  mutation a single atomic bulk call. Removing a key from `values` deletes
  that secret on reconcile.

## [0.1.0] — 2026-05-26

Initial public release. 11 resource types covering the Supabase
Management API surface; matches feature parity with the official
Supabase Terraform provider plus extras (Organization CRUD,
NetworkRestriction CRUD, separate `*Settings` singletons).

### Added

- `SUPABASE::Platform::Project` — async create / read / update / delete
  / list, polls until `ACTIVE_HEALTHY`.
- `SUPABASE::Platform::Branch` — async create on existing project,
  polls until `FUNCTIONS_DEPLOYED` / `MIGRATIONS_PASSED`. Paid plan
  only.
- `SUPABASE::Platform::Organization` — POST/GET only (API limit).
- `SUPABASE::Auth::APIKey` — publishable + secret keys, JWT template
  support, `?reveal=true` integration, `APIKeyResolvable` for
  same-plugin DAGs.
- `SUPABASE::Functions::EdgeFunction` — inline JS/TS body deploy.
- `SUPABASE::Functions::Secret` — bulk-endpoint bridge, one Forma
  resource per secret name.
- `SUPABASE::Config::AuthSettings` — full Auth config singleton
  (`/v1/projects/{ref}/config/auth`).
- `SUPABASE::Config::APISettings` — PostgREST singleton
  (`/v1/projects/{ref}/postgrest`).
- `SUPABASE::Config::DatabaseSettings` — Postgres singleton
  (`/v1/projects/{ref}/config/database/postgres`, uses PUT).
- `SUPABASE::Config::NetworkRestriction` — CIDR allowlist singleton.

- Per-namespace subpackages (`pkg/resources/{platform,auth,functions,config}`),
  self-registration via `init()`, slim main dispatcher modelled on the
  K8s plugin.
- Minimal HTTP transport (`pkg/transport/supabase`) — Bearer auth,
  rate-limit ready, error classification including the 406 Supabase
  returns on deleted API keys.
- 41 unit tests (httptest-driven).
- 23 conformance fixtures (`testdata/*.pkl`) — Create + Update +
  Replace variants per resource where applicable.
- `scripts/ci/clean-environment.sh` — live cleanup hook for conformance
  test residue.
- `examples/basic/` — single Edge Function forma.
- `examples/k8s-supabase/` — cross-plugin demo running a real Next.js
  Supabase Starter Kit in a kind cluster, end-to-end verified against
  live Supabase Auth `/settings`.

### Known limitations

- Cross-plugin Resolvable from `SUPABASE::Auth::APIKey` into k8s
  `Secret.stringData` requires k8s schema to widen to
  `Mapping<String, (String|formae.Resolvable)>`. Today the value flows
  via env shim. Same-plugin DAGs work (`anonKey.res.apiKey`).
- Storage buckets surface as discovery-only — Management API exposes
  `GET` but no Create/Update; full management requires the per-project
  Storage REST API.
- See README "Roadmap — known gaps" for the full list of unimplemented
  Management API endpoints (~80 of 109).

[Unreleased]: https://github.com/platform-engineering-labs/formae-plugin-supabase/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/platform-engineering-labs/formae-plugin-supabase/releases/tag/v0.1.0

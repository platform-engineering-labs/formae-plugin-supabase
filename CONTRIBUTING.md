# Contributing

This document covers local development for plugin authors. For user-facing
plugin docs (configuration, supported resources, examples), see
[README.md](README.md).

## Prerequisites

- Go 1.25+
- [Pkl CLI](https://pkl-lang.org/main/current/pkl-cli/index.html)
- Cloud provider credentials (for conformance testing)

## Local Installation

The Hub-facing install path for end users is `formae plugin install
<publisher>/<plugin>` — that pulls signed artifacts from the orbital
repo. For plugin authors building locally, install from source:

```bash
make install
```

This builds the plugin binary and installs it into your local formae
plugin directory so the agent picks it up on the next start.

## Building

```bash
make build      # Build plugin binary
make test       # Run unit tests
make lint       # Run linter
make install    # Build + install locally
```

## Local Testing

```bash
# Install plugin locally
make install

# Start formae agent
formae agent start

# Apply example resources
formae apply --mode reconcile --watch examples/basic/main.pkl
```

## Conformance Testing

Conformance tests validate your plugin's CRUD lifecycle using the test
fixtures in `testdata/`:

| File | Purpose |
|------|---------|
| `resource.pkl` | Initial resource creation |
| `resource-update.pkl` | In-place update (mutable fields) |
| `resource-replace.pkl` | Replacement (createOnly fields) |

The test harness sets `FORMAE_TEST_RUN_ID` for unique resource naming
between runs.

```bash
make conformance-test                  # Latest formae version
make conformance-test VERSION=0.80.0   # Specific version
```

The `scripts/ci/clean-environment.sh` script cleans up test resources.
It runs before and after conformance tests and should be idempotent.

## Publishing to the Hub

This plugin uses `FSL-1.1-ALv2` (Functional Source License, ALv2 Future
License), matching the other Platform Engineering cloud plugins. The Hub
also accepts `Apache-2.0`, `BSD-3-Clause`, `MIT`, and `MPL-2.0`.

The Hub matches the **`LICENSE` file body** against canonical license text,
so `LICENSE` must be the verbatim license — not a reflowed or truncated copy.
Set the `license` field in `formae-plugin.pkl` to the matching SPDX identifier
and keep `LICENSE` in sync. Plugins under any other license can still be built
and used locally, but the Hub registration step will reject them.

For the full publishing flow, see the
[Plugin SDK Documentation](https://docs.formae.io/plugin-sdk).

# Local dev env for running conformance tests against live Supabase.
#
# Usage:
#   cp scripts/dev/env.example.sh scripts/dev/env.sh
#   $EDITOR scripts/dev/env.sh           # fill in your values
#   source scripts/dev/env.sh
#   make conformance-test-crud TEST=project
#
# `scripts/dev/env.sh` is gitignored — never commit secrets.

# Supabase Management API personal access token. Create at:
#   https://supabase.com/dashboard/account/tokens
export SUPABASE_ACCESS_TOKEN=sbp_xxxxxxxxxxxx

# Organization that owns the test project. Required for Project Create.
# Find under Settings → General → Organization in the dashboard.
export SUPABASE_ORGANIZATION_ID=your_org_id

# Pre-existing PAID project ref used by every sub-resource test
# (Branch, APIKey, EdgeFunction, Secret). Keeps the test loop fast and
# avoids paying for full project provisioning each run.
export SUPABASE_PROJECT_REF=your_project_ref

# Postgres password for the Project Create test (>= 8 chars).
export SUPABASE_DB_PASS=plugin-sdk-test-pass

# --- Optional overrides ---

# Region used for new projects created by the Project conformance test.
# export SUPABASE_REGION=us-east-1

# Region used by the Project replace test (must differ from REGION).
# export SUPABASE_REPLACEMENT_REGION=eu-west-1

#!/bin/bash
# © 2026 Platform Engineering Labs Inc.
# SPDX-License-Identifier: Apache-2.0
#
# Clean Environment Hook for the Supabase plugin.
#
# Called before and after conformance tests. Removes test resources from the
# Supabase project pointed at by SUPABASE_PROJECT_REF, and any orphaned
# projects under SUPABASE_ORGANIZATION_ID whose name matches the test prefix.
#
# Required env:
#   SUPABASE_ACCESS_TOKEN   Personal Access Token (Bearer auth)
#   SUPABASE_PROJECT_REF    Project hosting Branch/APIKey/EdgeFunction/Secret tests
#   SUPABASE_ORGANIZATION_ID  Organization that owns ephemeral test projects
#
# Optional:
#   TEST_PREFIX (default: "formae-sdk-test-"). Names must start with this.
#   SUPABASE_API_BASE (default: https://api.supabase.com)

set -euo pipefail

TEST_PREFIX="${TEST_PREFIX:-formae-sdk-test-}"
API_BASE="${SUPABASE_API_BASE:-https://api.supabase.com}"

if [[ -z "${SUPABASE_ACCESS_TOKEN:-}" ]]; then
  echo "clean-environment.sh: SUPABASE_ACCESS_TOKEN unset — skipping cleanup"
  exit 0
fi

auth_curl() {
  curl --silent --show-error \
    --header "Authorization: Bearer ${SUPABASE_ACCESS_TOKEN}" \
    --header "Accept: application/json" \
    "$@"
}

echo "clean-environment.sh: cleaning resources with prefix '${TEST_PREFIX}'"

# --- Sub-resources on the shared project -------------------------------------
if [[ -n "${SUPABASE_PROJECT_REF:-}" ]]; then
  PROJ_BASE="${API_BASE}/v1/projects/${SUPABASE_PROJECT_REF}"

  echo "  edge functions..."
  fns=$(auth_curl "${PROJ_BASE}/functions" | jq -r --arg p "${TEST_PREFIX}" \
    '.[]? | select(.slug | startswith($p) or startswith("sdk-test-")) | .slug' || true)
  for slug in ${fns}; do
    echo "    DELETE function ${slug}"
    auth_curl -X DELETE "${PROJ_BASE}/functions/${slug}" >/dev/null || true
  done

  echo "  edge function secrets..."
  names=$(auth_curl "${PROJ_BASE}/secrets" | jq -r \
    '.[]? | select(.name | startswith("SDK_TEST_")) | .name' || true)
  if [[ -n "${names}" ]]; then
    payload=$(echo "${names}" | jq -R . | jq -s .)
    echo "    DELETE secrets: ${names//$'\n'/ }"
    auth_curl -X DELETE -H "Content-Type: application/json" \
      --data "${payload}" "${PROJ_BASE}/secrets" >/dev/null || true
  fi

  echo "  api keys..."
  ids=$(auth_curl "${PROJ_BASE}/api-keys" | jq -r \
    '.[]? | select(.name | startswith("sdk_test_")) | .id' || true)
  for id in ${ids}; do
    echo "    DELETE api-key ${id}"
    auth_curl -X DELETE "${PROJ_BASE}/api-keys/${id}" >/dev/null || true
  done

  echo "  branches..."
  branch_ids=$(auth_curl "${PROJ_BASE}/branches" | jq -r --arg p "${TEST_PREFIX}" \
    '.[]? | select(.name | startswith($p) or startswith("sdk-test-")) | .id' || true)
  for bid in ${branch_ids}; do
    echo "    DELETE branch ${bid}"
    auth_curl -X DELETE "${API_BASE}/v1/branches/${bid}" >/dev/null || true
  done
fi

# --- Top-level test projects (created by project.pkl conformance) ------------
echo "  projects..."
proj_refs=$(auth_curl "${API_BASE}/v1/projects" | jq -r --arg p "${TEST_PREFIX}" \
  '.[]? | select(.name | startswith($p)) | .id' || true)
for ref in ${proj_refs}; do
  echo "    DELETE project ${ref}"
  auth_curl -X DELETE "${API_BASE}/v1/projects/${ref}" >/dev/null || true
done

# Organizations are not deletable through the Management API. Skip.

echo "clean-environment.sh: done"

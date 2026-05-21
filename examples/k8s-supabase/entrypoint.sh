#!/bin/sh
# © 2026 Platform Engineering Labs Inc.
# SPDX-License-Identifier: Apache-2.0
#
# Runtime env replacement for Next.js NEXT_PUBLIC_* values.
#
# Next.js inlines NEXT_PUBLIC_* into the client bundle and prerendered
# HTML at build time. To keep the image portable across Supabase
# projects, the build bakes fixed placeholder strings, and this
# entrypoint rewrites them on container start using env vars from the
# k8s Secret.

set -eu

PLACEHOLDER_URL="https://placeholder.supabase.co"
PLACEHOLDER_KEY="sb_publishable_PLACEHOLDER_KEY_DO_NOT_USE_IN_PROD"

: "${NEXT_PUBLIC_SUPABASE_URL:?must be set}"
: "${NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY:?must be set}"

echo "entrypoint: NEXT_PUBLIC_SUPABASE_URL -> $NEXT_PUBLIC_SUPABASE_URL"
echo "entrypoint: NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY -> ${NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY%???}…"

# Rewrite every text-y file under /app/.next.
find /app/.next -type f \( -name '*.js' -o -name '*.html' -o -name '*.json' -o -name '*.txt' -o -name '*.rsc' \) -print0 \
  | xargs -0 sed -i \
      -e "s|$PLACEHOLDER_URL|$NEXT_PUBLIC_SUPABASE_URL|g" \
      -e "s|$PLACEHOLDER_KEY|$NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY|g"

echo "entrypoint: launching server.js"
exec node server.js

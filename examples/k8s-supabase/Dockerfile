# © 2026 Platform Engineering Labs Inc.
# SPDX-License-Identifier: Apache-2.0
#
# Next.js (vercel/next.js examples/with-supabase) multi-stage build.
# Result is the standalone Next.js server in a slim node:alpine runtime.
#
# Build + push to a registry your cluster can pull from, or build locally
# and load into kind:
#   docker build -t k8s-supabase-demo:latest .
#   kind load docker-image k8s-supabase-demo:latest --name <cluster>

FROM node:24-alpine AS deps
WORKDIR /src
COPY app/package.json app/package-lock.json* ./
RUN npm ci --no-audit --no-fund

FROM node:24-alpine AS build
WORKDIR /src
COPY --from=deps /src/node_modules ./node_modules
COPY app/ ./
# Bake placeholder strings that entrypoint.sh rewrites at runtime with
# the real values from the k8s Secret. Keeps the image portable across
# Supabase projects.
ENV NEXT_PUBLIC_SUPABASE_URL=https://placeholder.supabase.co
ENV NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY=sb_publishable_PLACEHOLDER_KEY_DO_NOT_USE_IN_PROD
RUN npm run build

FROM node:24-alpine AS runtime
WORKDIR /app
ENV NODE_ENV=production
ENV PORT=8080
ENV HOSTNAME=0.0.0.0

# next.js standalone output already contains node_modules + server.js
COPY --from=build /src/.next/standalone ./
COPY --from=build /src/.next/static ./.next/static

COPY entrypoint.sh /entrypoint.sh

EXPOSE 8080
# Run as root so the entrypoint can sed-rewrite the built bundle. The
# Next.js server itself drops nothing privileged.
ENTRYPOINT ["/entrypoint.sh"]

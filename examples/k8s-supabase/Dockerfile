# © 2026 Platform Engineering Labs Inc.
# SPDX-License-Identifier: Apache-2.0
#
# Tiny demo container for examples/k8s-supabase/.
# Build + push to a registry your cluster can pull from:
#   docker build -t ghcr.io/YOUR-ORG/k8s-supabase-demo:latest .
#   docker push ghcr.io/YOUR-ORG/k8s-supabase-demo:latest

FROM golang:1.25-alpine AS build
WORKDIR /src
COPY app/ ./
RUN go mod init demo 2>/dev/null || true \
 && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/demo .

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/demo /demo
USER nonroot
EXPOSE 8080
ENTRYPOINT ["/demo"]

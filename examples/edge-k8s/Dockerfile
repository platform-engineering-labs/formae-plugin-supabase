# © 2026 Platform Engineering Labs Inc.
# SPDX-License-Identifier: Apache-2.0
#
# Multi-stage Go build for the edge-k8s demo. Statically linked
# distroless final stage so the image is ~20 MB and pulls fast on a
# fresh kind cluster.
#
# Build + push:
#   docker build -t ghcr.io/your-org/edge-k8s-demo:latest .
#   docker push  ghcr.io/your-org/edge-k8s-demo:latest
#
# Or for local kind:
#   docker build -t edge-k8s-demo:latest .
#   kind load docker-image edge-k8s-demo:latest --name <cluster>

FROM golang:1.22-alpine AS build
WORKDIR /src
COPY app/go.mod ./app/go.mod
WORKDIR /src/app
RUN go mod download
COPY app/. ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./...

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/server /server
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/server"]

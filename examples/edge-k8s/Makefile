# © 2026 Platform Engineering Labs Inc.
# SPDX-License-Identifier: Apache-2.0
#
# Convenience targets for the edge-k8s cross-plugin demo.
#
# Required env (source ./.demo.env or export manually):
#   SUPABASE_ACCESS_TOKEN  PAT from https://supabase.com/dashboard/account/tokens
#   SUPABASE_PROJECT_REF   existing project ref
#   SUPABASE_ORGANIZATION_ID
#   DEMO_IMAGE             registry/image:tag the cluster can pull

DEMO_IMAGE   ?= edge-k8s-demo:latest
INGRESS_HOST ?= edge-k8s-demo.local
KIND_CLUSTER ?= kind

.PHONY: image kind-load apply-phase1 anon-key apply-phase2 apply destroy port-forward curl

## image: build the demo container image
image:
	docker build -t $(DEMO_IMAGE) .

## kind-load: load the image into a local kind cluster
kind-load: image
	kind load docker-image $(DEMO_IMAGE) --name $(KIND_CLUSTER)

## apply-phase1: create the Supabase resources (APIKey + EdgeFunction)
## and the k8s namespace/secret with an empty anon key placeholder.
apply-phase1:
	DEMO_IMAGE=$(DEMO_IMAGE) INGRESS_HOST=$(INGRESS_HOST) \
	  SUPABASE_ANON_KEY="" formae apply --mode reconcile --yes forma.pkl

## anon-key: print the publishable key the supabase plugin minted, so
## you can export SUPABASE_ANON_KEY for phase 2.
anon-key:
	@curl -s -H "Authorization: Bearer $$SUPABASE_ACCESS_TOKEN" \
	  "https://api.supabase.com/v1/projects/$$SUPABASE_PROJECT_REF/api-keys?reveal=true" \
	  | jq -r '.[] | select(.name == "edge_k8s_demo_publishable") | .api_key'

## apply-phase2: re-apply with the real anon key so the k8s Secret is
## populated end-to-end. Run `export SUPABASE_ANON_KEY=$(make anon-key)` first.
apply-phase2:
	@test -n "$$SUPABASE_ANON_KEY" || (echo "SUPABASE_ANON_KEY must be set; run: export SUPABASE_ANON_KEY=\$$(make anon-key)" && exit 1)
	DEMO_IMAGE=$(DEMO_IMAGE) INGRESS_HOST=$(INGRESS_HOST) \
	  formae apply --mode reconcile --yes forma.pkl

## apply: end-to-end (phase 1 + key fetch + phase 2)
apply: apply-phase1
	@echo "Fetching minted anon key..."
	@$(MAKE) --no-print-directory _phase2-with-key

_phase2-with-key:
	@key=$$($(MAKE) --no-print-directory anon-key); \
	  test -n "$$key" || (echo "anon key was empty"; exit 1); \
	  echo "Re-applying with anon key prefix: $$(echo $$key | cut -c1-18)…"; \
	  SUPABASE_ANON_KEY="$$key" \
	  DEMO_IMAGE=$(DEMO_IMAGE) INGRESS_HOST=$(INGRESS_HOST) \
	    formae apply --mode reconcile --yes forma.pkl

## destroy: tear down everything formae apply created
destroy:
	DEMO_IMAGE=$(DEMO_IMAGE) INGRESS_HOST=$(INGRESS_HOST) \
	  SUPABASE_ANON_KEY="$$SUPABASE_ANON_KEY" formae destroy --yes forma.pkl

## port-forward: forward the demo service to localhost:8080
port-forward:
	kubectl -n edge-k8s-demo port-forward svc/edge-k8s-demo 8080:80

## curl: hit the demo via port-forward (run port-forward in another terminal)
curl:
	@curl -s http://localhost:8080

// © 2026 Platform Engineering Labs Inc.
//
// SPDX-License-Identifier: Apache-2.0
//
// Tiny HTTP server that proves the k8s → Supabase wiring.
//
// On every GET /, it calls the Supabase Edge Function whose slug was
// stamped into our env by the k8s Secret, using the publishable API
// key the supabase plugin minted in the same `formae apply`. The
// function's JSON body is returned to the caller so you can see live
// proof that both plugins co-operated.

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	url := os.Getenv("SUPABASE_URL")
	key := os.Getenv("SUPABASE_ANON_KEY")
	slug := os.Getenv("SUPABASE_FUNCTION_SLUG")
	addr := ":8080"

	// Empty anon key = phase-1 of the two-phase apply (k8s Secret stamped
	// with a blank value while formae waits for the user to seed it).
	// Don't crash — render a placeholder so the Deployment goes Ready
	// and the apply reaches Success. Phase-2 swaps the Secret value and
	// the rolling restart picks up the real key without disturbing the
	// rest of the stack.
	if key == "" || url == "" {
		log.Printf("startup: SUPABASE_ANON_KEY or SUPABASE_URL empty — running in placeholder mode")
	} else {
		log.Printf("startup: calling edge function at %s/functions/v1/%s", url, slug)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "edge-k8s-demo\n=============\n")
		fmt.Fprintf(w, "supabase url:   %s\n", orPlaceholder(url))
		fmt.Fprintf(w, "function slug:  %s\n", orPlaceholder(slug))
		fmt.Fprintf(w, "anon key:       %s\n", keyState(key))

		if url == "" || key == "" || slug == "" {
			fmt.Fprintf(w, "\nplaceholder mode — phase-2 of the two-phase apply hasn't\n")
			fmt.Fprintf(w, "seeded the k8s Secret yet. Run: make apply\n")
			return
		}

		endpoint := fmt.Sprintf("%s/functions/v1/%s", url, slug)
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, "POST", endpoint, nil)
		if err != nil {
			fmt.Fprintf(w, "\nbuild request error: %v\n", err)
			return
		}
		req.Header.Set("Authorization", "Bearer "+key)
		req.Header.Set("apikey", key)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Fprintf(w, "\nedge call error: %v\n", err)
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(w, "\nedge function HTTP %d:\n%s\n", resp.StatusCode, body)
	})

	http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	})

	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func orPlaceholder(s string) string {
	if s == "" {
		return "<unset — phase 1 placeholder>"
	}
	return s
}

func keyState(s string) string {
	if s == "" {
		return "<unset — phase 1 placeholder>"
	}
	if len(s) <= 18 {
		return s
	}
	return s[:18] + "…"
}

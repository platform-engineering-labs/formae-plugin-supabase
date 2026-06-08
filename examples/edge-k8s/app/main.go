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

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Fatalf("env %s must be set (sourced from k8s Secret)", k)
	}
	return v
}

func main() {
	url := mustEnv("SUPABASE_URL")
	key := mustEnv("SUPABASE_ANON_KEY")
	slug := mustEnv("SUPABASE_FUNCTION_SLUG")
	addr := ":8080"

	endpoint := fmt.Sprintf("%s/functions/v1/%s", url, slug)
	log.Printf("calling edge function: %s", endpoint)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "POST", endpoint, nil)
		if err != nil {
			http.Error(w, "build request: "+err.Error(), 500)
			return
		}
		req.Header.Set("Authorization", "Bearer "+key)
		req.Header.Set("apikey", key)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, "edge call: "+err.Error(), 502)
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "edge-k8s-demo\n")
		fmt.Fprintf(w, "=============\n")
		fmt.Fprintf(w, "supabase url: %s\n", url)
		fmt.Fprintf(w, "function slug: %s\n", slug)
		fmt.Fprintf(w, "anon key prefix: %s...\n", safePrefix(key, 18))
		fmt.Fprintf(w, "\nedge function HTTP %d:\n%s\n", resp.StatusCode, body)
	})

	http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	})

	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func safePrefix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

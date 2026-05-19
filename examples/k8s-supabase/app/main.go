// © 2026 Platform Engineering Labs Inc.
//
// SPDX-License-Identifier: Apache-2.0

// Tiny demo HTTP server for examples/k8s-supabase/.
//
// On GET / it prints the SUPABASE_URL it sees, then performs a single
// REST call against the Supabase Auth /settings endpoint using the
// supplied anon key. Proves end-to-end that the k8s pod received the
// credentials that the supabase plugin minted.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	addr := ":8080"
	if v := os.Getenv("PORT"); v != "" {
		addr = ":" + v
	}
	http.HandleFunc("/", root)
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) })
	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func root(w http.ResponseWriter, r *http.Request) {
	url := os.Getenv("NEXT_PUBLIC_SUPABASE_URL")
	key := os.Getenv("NEXT_PUBLIC_SUPABASE_ANON_KEY")

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintln(w, "k8s <> supabase demo")
	fmt.Fprintln(w, "====================")
	fmt.Fprintf(w, "SUPABASE_URL: %s\n", url)
	fmt.Fprintf(w, "ANON_KEY prefix: %s\n\n", short(key))

	if url == "" || key == "" {
		fmt.Fprintln(w, "credentials missing — env not injected")
		return
	}

	settings, err := fetchAuthSettings(url, key)
	if err != nil {
		fmt.Fprintf(w, "auth settings fetch failed: %v\n", err)
		return
	}
	fmt.Fprintln(w, "live Supabase Auth /settings response:")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(settings)
}

func fetchAuthSettings(baseURL, key string) (map[string]any, error) {
	req, err := http.NewRequest("GET", baseURL+"/auth/v1/settings", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("apikey", key)
	req.Header.Set("Authorization", "Bearer "+key)

	c := &http.Client{Timeout: 5 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, body)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func short(s string) string {
	if len(s) <= 12 {
		return s
	}
	return s[:8] + "…"
}

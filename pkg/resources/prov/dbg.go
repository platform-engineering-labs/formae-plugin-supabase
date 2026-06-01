// © 2026 Platform Engineering Labs Inc.
//
// SPDX-License-Identifier: Apache-2.0

package prov

import (
	"fmt"
	"os"
)

// Dbg prints a debug line to stderr prefixed with `[supabase-plugin]`.
// The formae agent picks up the plugin's stderr and tags entries
// "Plugin error tag=SUPABASE: ..." which keeps the message in logs even
// when the agent doesn't surface plugin stdout.
func Dbg(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[supabase-plugin] "+format+"\n", args...)
}

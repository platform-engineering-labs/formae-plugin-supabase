// © 2026 Platform Engineering Labs Inc.
//
// SPDX-License-Identifier: Apache-2.0

package prov

import (
	"fmt"
	"strings"
)

// ParseTwoPart splits a composite native id like "{project_ref}/{child_id}"
// into its two segments. Either segment empty is an error.
func ParseTwoPart(id string) (parent, child string, err error) {
	parts := strings.SplitN(id, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("native id must be {parent}/{child}, got %q", id)
	}
	return parts[0], parts[1], nil
}

// JoinTwoPart formats a composite native id.
func JoinTwoPart(parent, child string) string {
	return parent + "/" + child
}

package shared

import (
	"fmt"
	"strings"
)

// SplitAndTrimCSV splits each entry on ',' and trims whitespace from every
// element. It returns an error if any element is empty after trimming.
func SplitAndTrimCSV(entries []string) ([]string, error) {
	var out []string
	for _, entry := range entries {
		for _, part := range strings.Split(entry, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				return nil, fmt.Errorf("empty entry in comma-separated list")
			}
			out = append(out, part)
		}
	}
	return out, nil
}

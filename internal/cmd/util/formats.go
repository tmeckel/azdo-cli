package util

import (
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
)

// FormatTimePtr formats an *azuredevops.Time as a *string using its query parameter format.
// Returns nil for nil input.
func FormatTimePtr(ts *azuredevops.Time) *string {
	if ts == nil {
		return nil
	}
	formatted := ts.AsQueryParameter()
	if strings.TrimSpace(formatted) == "" {
		return nil
	}
	return &formatted
}

// FormatTimeShort formats an *azuredevops.Time as a human-readable string ("2006-01-02 15:04:05").
// Returns "" for nil or zero-value input.
func FormatTimeShort(ts *azuredevops.Time) string {
	if ts == nil {
		return ""
	}
	t := ts.Time
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

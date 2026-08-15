package shared

import (
	"strings"
	"unicode/utf8"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

// NormalizePath normalizes an area/iteration path for the given project:
// trims whitespace, converts '/' separators to the '\' separators Azure
// DevOps uses in tree paths, strips a leading '\', and prefixes the project
// name when the input is relative. Paths that are already rooted at the
// project (compared case-insensitively) are preserved unchanged.
func NormalizePath(project, raw string) string {
	path := strings.TrimSpace(raw)
	path = strings.ReplaceAll(path, "/", `\`)
	path = strings.TrimPrefix(path, `\`)
	if path == "" {
		return ""
	}
	if strings.EqualFold(path, project) || strings.HasPrefix(strings.ToLower(path), strings.ToLower(project)+`\`) {
		return path
	}
	return project + `\` + path
}

// SplitUnderPrefix reports whether raw carries the "Under:" subtree prefix
// (matched case-insensitively) and returns the remaining path text with the
// prefix removed, preserving the original casing of the path.
func SplitUnderPrefix(raw string) (under bool, path string) {
	const prefix = "under:"
	if len(raw) < len(prefix) || !strings.EqualFold(raw[:len(prefix)], prefix) {
		return false, raw
	}
	return true, raw[len(prefix):]
}

// ValidateUnderPaths rejects empty area/iteration path values.
func ValidateUnderPaths(flag string, values []string) error {
	for _, raw := range values {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		_, path := SplitUnderPrefix(raw)
		path = strings.TrimSpace(path)
		if path == "" {
			return util.FlagErrorf("%s value %q is invalid; path must not be empty", flag, raw)
		}
	}
	return nil
}

// ValidateTags rejects tag values that Azure DevOps cannot store: empty or
// whitespace-only values, values containing the ',' or ';' separator
// characters, and values longer than 400 characters (Unicode code points).
// The '@' character is allowed: Microsoft recommends avoiding it in tag
// names, but that is a recommendation, not a hard restriction, so it is not
// rejected client-side.
func ValidateTags(flag string, values []string) error {
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return util.FlagErrorf("%s value cannot be empty", flag)
		}
		if strings.ContainsAny(trimmed, ",;") {
			return util.FlagErrorf("%s value %q is invalid; tags cannot contain ',' or ';'", flag, trimmed)
		}
		if utf8.RuneCountInString(trimmed) > 400 {
			return util.FlagErrorf("%s value is too long (%d characters); tags are limited to 400 characters", flag, utf8.RuneCountInString(trimmed))
		}
	}
	return nil
}

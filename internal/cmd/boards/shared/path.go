package shared

import (
	"net/url"
	"regexp"
	"strings"
)

// collapseMultipleSlashes replaces consecutive slashes with a single slash.
var multipleSlashes = regexp.MustCompile(`/+`)

// NormalizeClassificationPath normalizes classification node paths.
//
// Behavior:
// - If raw is empty or only whitespace/slashes, returns an empty string.
// - Converts backslashes to forward slashes.
// - Trims surrounding whitespace.
// - Collapses consecutive slashes (e.g. "a//b" -> "a/b").
// - Removes any leading and trailing slashes.
// Examples:
//
//	NormalizeClassificationPath("  \\Project\\Area\\Foo\\ ") -> "Project/Area/Foo"
//	NormalizeClassificationPath("/a//b/") -> "a/b"
func NormalizeClassificationPath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	// Normalize separators
	s := strings.ReplaceAll(raw, "\\", "/")

	// Collapse repeated slashes
	s = multipleSlashes.ReplaceAllString(s, "/")

	// Trim leading/trailing slashes and surrounding whitespace
	s = strings.Trim(s, "/")
	s = strings.TrimSpace(s)

	return s
}

// BuildClassificationPath prepares the path segment for Azure DevOps REST endpoints.
//
// Rules and validations:
// - Accepts user input with either forward or backward slashes.
// - Uses the same normalization rules as NormalizeClassificationPath.
// - If the trimmed input is empty, returns ("", nil) to indicate "no path" (keeps existing callers behavior).
// - Removes a leading project segment (case-insensitive) if present.
// - If includesScope is true, removes a leading scopeName segment (case-insensitive) if present after project removal.
// - Rejects input that would produce empty segments (e.g., "a//b" after normalization should not happen because we collapse slashes).
// - Trims whitespace around each segment.
// - URL-escapes each segment with url.PathEscape and joins with forward slashes.
func BuildClassificationPath(project string, includesScope bool, scopeName string, raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		// No path provided; caller semantics: return empty path without error.
		return "", nil
	}

	normalized := NormalizeClassificationPath(trimmed)
	if normalized == "" {
		// Input had only slashes/whitespace.
		return "", nil
	}

	segments := strings.Split(normalized, "/")

	// Remove leading project segment if present (case-insensitive)
	if len(segments) > 0 && project != "" && strings.EqualFold(segments[0], project) {
		segments = segments[1:]
	}

	// Remove leading scope segment if requested
	if includesScope && len(segments) > 0 && scopeName != "" && strings.EqualFold(segments[0], scopeName) {
		segments = segments[1:]
	}

	if len(segments) == 0 {
		// After removing project/scope there is no path left → treat as no path
		return "", nil
	}

	// Filter out and skip empty (space-only) segments that may appear between separators,
	// e.g. "SP4DB/Area/ /   /WKB" -> ["SP4DB","Area","","","WKB"] -> skip empties -> ["SP4DB","Area","WKB"]
	filtered := make([]string, 0, len(segments))
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			// Skip segments that are only whitespace.
			continue
		}
		filtered = append(filtered, seg)
	}

	if len(filtered) == 0 {
		// Nothing meaningful left after filtering
		return "", nil
	}

	escaped := make([]string, 0, len(filtered))
	for _, seg := range filtered {
		escaped = append(escaped, url.PathEscape(seg))
	}

	return strings.Join(escaped, "/"), nil
}

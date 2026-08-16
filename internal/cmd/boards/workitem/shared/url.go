package shared

import (
	"net/url"
	"strings"

	"github.com/tmeckel/azdo-cli/internal/azdo"
)

// ParseWorkItemURL extracts the organization and project from a work item URL:
// the subdomain of *.visualstudio.com hosts or the first path segment of
// dev.azure.com URLs, and the path segment directly before "/_apis" as the
// project with a fallback to the first segment. It is best-effort: malformed
// URLs and non-Azure hosts yield empty or host-derived values instead of
// errors, which keeps the relation fallback behavior forgiving. The
// organization extraction is delegated to azdo.ParseURL (lax mode); only the
// work-item-specific "_apis" project rule lives here.
func ParseWorkItemURL(raw string) (organization string, project string) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Hostname() == "" {
		return "", ""
	}
	id, err := azdo.ParseURL(u, true)
	if err != nil {
		return "", ""
	}
	segs := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i, s := range segs {
		if s == "_apis" && i > 0 {
			return id.Organization, segs[i-1]
		}
	}
	if len(segs) > 0 {
		return id.Organization, segs[0]
	}
	return id.Organization, ""
}

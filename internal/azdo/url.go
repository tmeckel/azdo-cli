package azdo

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// Sentinel errors reported by ParseURL for degenerate inputs. Callers
// classify them with errors.Is instead of comparing error text.
var (
	// ErrNotAzDO reports a URL whose hostname is neither dev.azure.com,
	// ssh.dev.azure.com, nor a *.visualstudio.com subdomain.
	ErrNotAzDO = errors.New("not an Azure DevOps host")

	// ErrInvalidPath reports an Azure DevOps URL whose path lacks the
	// segments required to identify an organization.
	ErrInvalidPath = errors.New("invalid Azure DevOps URL path")
)

// IsVisualStudioHost reports whether hostname is a *.visualstudio.com
// subdomain (the classic DevOps host style, e.g.
// https://{organization}.visualstudio.com). The match is case-insensitive
// and mirrors the suffix check that ParseURL and RepositoryFromURL share.
func IsVisualStudioHost(hostname string) bool {
	return strings.HasSuffix(strings.ToLower(hostname), ".visualstudio.com")
}

// URLIdentity captures the organization and optional project carried by an
// Azure DevOps URL.
type URLIdentity struct {
	// Organization is the organization identified by the URL. It is empty
	// only for degenerate paths (with lax parsing) or unparsable URLs.
	Organization string
	// Project is empty when the URL carries no project segment.
	Project string
}

// invalidPathError renders like the legacy "invalid path %q" message so
// existing error-string comparisons keep passing, while Unwrap lets callers
// classify it with errors.Is(err, ErrInvalidPath).
type invalidPathError struct {
	path string
}

func (e *invalidPathError) Error() string {
	return fmt.Sprintf("invalid path %q", e.path)
}

func (e *invalidPathError) Unwrap() error {
	return ErrInvalidPath
}

// ParseURL extracts the organization and project identity from an Azure
// DevOps URL. It understands the three canonical host styles:
//
//	https://{organization}.visualstudio.com/{project}/...
//	https://dev.azure.com/{organization}/{project}/...
//	ssh://ssh.dev.azure.com/v3/{organization}/{project}/...
//
// With lax=false, non-Azure hosts yield an error wrapping ErrNotAzDO and
// paths without an organization segment yield an error wrapping
// ErrInvalidPath. With lax=true the parser is best-effort: any hostname is
// accepted as an organization and missing segments leave the corresponding
// field empty. Strict validation of repository paths, schemes, and configured
// hostnames stays with the callers (RepositoryFromURL, ProjectFromURL, ...).
func ParseURL(u *url.URL, lax bool) (URLIdentity, error) {
	if u == nil {
		return URLIdentity{}, fmt.Errorf("url must not be nil")
	}
	hostname := strings.ToLower(u.Hostname())
	if hostname == "" {
		return URLIdentity{}, fmt.Errorf("url must have a hostname")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")

	var id URLIdentity
	switch {
	case IsVisualStudioHost(hostname):
		// {org}.visualstudio.com/{project}/... carries the organization in
		// the subdomain.
		id.Organization = strings.SplitN(hostname, ".", 2)[0]
		if len(parts) > 0 {
			id.Project = parts[0]
		}
	case hostname == "dev.azure.com":
		// dev.azure.com/{org}/{project}/... carries segments in the path.
		if len(parts) > 0 {
			id.Organization = strings.ToLower(strings.TrimSpace(parts[0]))
		}
		if len(parts) > 1 {
			id.Project = parts[1]
		}
	case hostname == "ssh.dev.azure.com":
		// ssh.dev.azure.com/v3/{org}/{project}/.../ skips the protocol
		// version segment.
		if len(parts) > 1 {
			id.Organization = strings.ToLower(strings.TrimSpace(parts[1]))
		}
		if len(parts) > 2 {
			id.Project = parts[2]
		}
	default:
		if !lax {
			return URLIdentity{}, fmt.Errorf("not an Azure DevOps host %q: %w", u.Host, ErrNotAzDO)
		}
		id.Organization = hostname
		if len(parts) > 0 {
			id.Project = parts[0]
		}
	}

	if !lax && id.Organization == "" {
		return URLIdentity{}, &invalidPathError{path: u.Path}
	}
	return id, nil
}

package util

import (
	"fmt"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
	"github.com/tmeckel/azdo-cli/internal/types"
)

// Path represents a parsed user-input path of the form
// [ORGANIZATION[/PROJECT]]/TARGET[/SUBTARGET[/...]].
// Organization is always populated after a successful Parse.
type Path struct {
	Organization string
	Project      string
	Targets      []string
}

// ParseOptions configures how a raw user input is split into a Path.
type ParseOptions struct {
	AllowImplicitOrg bool
	RequireProject   bool
	MinTargets       int
	MaxTargets       int
}

// Parse splits raw command input into a Path using fixed Azure DevOps-style path rules.
//
// Input is first trimmed, then split on "/". Each segment is trimmed again, and empty
// segments are rejected. The resulting path is interpreted as:
//
//	[ORGANIZATION][/PROJECT][/TARGET[/TARGET...]]
//
// `opts` controls which prefix segments are allowed and how many trailing target
// segments must exist:
//
//   - AllowImplicitOrg allows organization to be omitted. When omitted, Parse loads the
//     default organization from ctx.Config().Authentication().GetDefaultOrganization().
//   - RequireProject requires one project segment before any target segments.
//   - MinTargets defines required trailing target count.
//   - MaxTargets defines allowed trailing target count. When MaxTargets is zero, Parse
//     treats the target count as unbounded (arbitrary length).
//
// Parse only supports at most two non-target prefix segments: optional organization and
// optional project. Targets are always the trailing segments and the prefix is the
// leading segments. The parser works from the back: for each candidate prefix length
// the target count is n - prefixLen, and only candidates whose target count falls
// within [MinTargets, MaxTargets] are considered.
//
// When MaxTargets is zero the target count is unbounded. In that case Parse pins the
// prefix to its maximum shape so that explicitly provided organization and project are
// retained and only the genuinely trailing segments are treated as targets.
//
// When MaxTargets is non-zero and MinTargets == MaxTargets the target count is fixed,
// so the prefix is simply the remaining leading segments (0, 1, or 2). When
// MaxTargets > MinTargets multiple prefix lengths may be valid. In that case Parse
// disambiguates as follows:
//
//   - If the smallest valid prefix would push targets to MaxTargets, prefer the largest
//     prefix so target capacity is not exhausted.
//   - If the largest valid prefix would push targets to MinTargets, prefer the smallest
//     prefix so target capacity is preserved.
//   - Otherwise, when project is optional (RequireProject is false) prefer the smallest
//     prefix so the optional project segment is not consumed; when organization is
//     optional (AllowImplicitOrg is true) prefer the largest prefix so an explicitly
//     provided organization is retained.
//
// If only a sub-maximum prefix is valid (the full org+project prefix does not fit) and
// organization is required, Parse rejects the input rather than guessing whether the
// missing segment is organization or project.
//
// Examples:
//
//	Parse(ctx, "org", ParseOptions{AllowImplicitOrg: false})
//	// => &Path{Organization: "org"}
//
//	Parse(ctx, "", ParseOptions{AllowImplicitOrg: true})
//	// => &Path{Organization: <default org>}
//
//	Parse(ctx, "project", ParseOptions{AllowImplicitOrg: true, RequireProject: true})
//	// => &Path{Organization: <default org>, Project: "project"}
//
//	Parse(nil, "org/project/group", ParseOptions{MinTargets: 1, MaxTargets: 1})
//	// => &Path{Organization: "org", Project: "project", Targets: []string{"group"}}
//
//	Parse(ctx, "pool/agent", ParseOptions{AllowImplicitOrg: true, MinTargets: 2, MaxTargets: 2})
//	// => &Path{Organization: <default org>, Targets: []string{"pool", "agent"}}
//
//	Parse(nil, "org/project/target/extra", ParseOptions{MinTargets: 1, MaxTargets: 2})
//	// => &Path{Organization: "org", Project: "project", Targets: []string{"target", "extra"}}
//
//	Parse(ctx, "org/project/a/b/c", ParseOptions{AllowImplicitOrg: false, MinTargets: 0, MaxTargets: 0})
//	// => &Path{Organization: "org", Project: "project", Targets: []string{"a", "b", "c"}}
//
// Error conditions:
//   - opts are invalid, for example MaxTargets is non-zero and smaller than MinTargets
//   - any segment is empty after trimming, for example "org/" or "org/ /project"
//   - total segment count falls outside the allowed range derived from opts
//   - only a sub-maximum prefix is valid, organization is required, and target count is
//     variable (bounded) — the input is ambiguous
//   - organization is omitted but ctx is nil
//   - organization is omitted and default organization lookup fails or returns empty
func Parse(ctx CmdContext, raw string, opts ParseOptions) (*Path, error) {
	trimmed := strings.TrimSpace(raw)
	var parts []string
	if trimmed != "" {
		parts = strings.Split(trimmed, "/")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
			if parts[i] == "" {
				return nil, fmt.Errorf("input %q contains empty segment", raw)
			}
		}
	}

	n := len(parts)

	minTargets := opts.MinTargets
	maxTargets := opts.MaxTargets
	unbounded := maxTargets == 0
	if minTargets < 0 {
		return nil, fmt.Errorf("invalid options: target range [%d,%d] is not satisfiable", opts.MinTargets, opts.MaxTargets)
	}
	if !unbounded && maxTargets < minTargets {
		return nil, fmt.Errorf("invalid options: target range [%d,%d] is not satisfiable", opts.MinTargets, opts.MaxTargets)
	}

	minOrg := 0
	if !opts.AllowImplicitOrg {
		minOrg = 1
	}
	minProject := 0
	if opts.RequireProject {
		minProject = 1
	}
	const maxOrg = 1
	const maxProject = 1
	minPrefix := minOrg + minProject
	maxPrefix := maxOrg + maxProject

	minTotal := minPrefix + minTargets

	if unbounded {
		if n < minTotal {
			return nil, fmt.Errorf("invalid input %q: expected at least %d segments, got %d", raw, minTotal, n)
		}
	} else {
		maxTotal := maxPrefix + maxTargets
		if n < minTotal || n > maxTotal {
			return nil, fmt.Errorf("invalid input %q: expected %d-%d segments, got %d", raw, minTotal, maxTotal, n)
		}
	}

	type candidate struct {
		hasOrg     bool
		hasProject bool
		prefixLen  int
	}

	var candidates []candidate
	candidates = append(candidates, candidate{true, true, 2})
	if !opts.RequireProject {
		candidates = append(candidates, candidate{true, false, 1})
	}
	if opts.AllowImplicitOrg && opts.RequireProject {
		candidates = append(candidates, candidate{false, true, 1})
	}
	if opts.AllowImplicitOrg && !opts.RequireProject {
		candidates = append(candidates, candidate{false, false, 0})
	}

	var feasible []candidate
	for _, c := range candidates {
		if c.prefixLen > n {
			continue
		}
		tc := n - c.prefixLen
		if tc < minTargets {
			continue
		}
		if !unbounded && tc > maxTargets {
			continue
		}
		feasible = append(feasible, c)
	}

	if len(feasible) == 0 {
		if unbounded {
			return nil, fmt.Errorf("invalid input %q: expected at least %d segments, got %d", raw, minTotal, n)
		}
		maxTotal := maxPrefix + maxTargets
		return nil, fmt.Errorf("invalid input %q: expected %d-%d segments, got %d", raw, minTotal, maxTotal, n)
	}

	var chosen candidate
	if len(feasible) == 1 {
		chosen = feasible[0]
	} else {
		high := feasible[0]
		low := feasible[len(feasible)-1]
		tHigh := n - high.prefixLen
		tLow := n - low.prefixLen

		switch {
		case unbounded:
			if opts.AllowImplicitOrg && opts.RequireProject && n-high.prefixLen == 1 {
				chosen = low
			} else {
				chosen = high
			}
		case tLow == maxTargets:
			chosen = high
		case tHigh == minTargets:
			chosen = low
		case !opts.RequireProject:
			chosen = low
		case opts.AllowImplicitOrg:
			chosen = high
		default:
			chosen = high
		}
	}

	p := &Path{}
	targetCount := n - chosen.prefixLen
	if targetCount > 0 {
		p.Targets = make([]string, targetCount)
		copy(p.Targets, parts[chosen.prefixLen:])
	}

	idx := 0
	if chosen.hasOrg {
		p.Organization = parts[idx]
		idx++
	}
	if chosen.hasProject {
		p.Project = parts[idx]
	}

	if p.Organization == "" {
		if ctx == nil {
			return nil, fmt.Errorf("no organization specified and no default organization configured")
		}
		cfg, err := ctx.Config()
		if err != nil {
			return nil, err
		}
		org, err := cfg.Authentication().GetDefaultOrganization()
		if err != nil {
			return nil, fmt.Errorf("no organization specified and no default organization configured: %w", err)
		}
		org = strings.TrimSpace(org)
		if org == "" {
			return nil, fmt.Errorf("no organization specified and no default organization configured")
		}
		p.Organization = org
	}

	return p, nil
}

// ParseScope resolves the organization and optional project from an input argument of the form
// "[ORGANIZATION[/PROJECT]]". When the input is empty, the default organization from the user configuration
// is returned. The function trims whitespace around individual segments and ensures the resulting values are
// non-empty when provided.
func ParseScope(ctx CmdContext, scope string) (*Path, error) {
	return Parse(ctx, scope, ParseOptions{
		AllowImplicitOrg: true,
	})
}

// ParseOrganizationArg resolves the organization from an input argument of the form "[ORGANIZATION]".
// When the input is empty, the default organization from the user configuration is returned.
func ParseOrganizationArg(ctx CmdContext, arg string) (string, error) {
	if strings.Count(arg, "/") > 0 {
		return "", fmt.Errorf("project or target scopes are not allowed")
	}
	scope, err := Parse(ctx, arg, ParseOptions{
		AllowImplicitOrg: true,
	})
	if err != nil {
		return "", err
	}
	return scope.Organization, nil
}

// ParseProjectScope parses arguments in the form [ORGANIZATION/]PROJECT. When the organization
// segment is omitted the default organization from the user's configuration is used. The function
// trims whitespace around individual segments and ensures the resulting values are non-empty.
func ParseProjectScope(ctx CmdContext, arg string) (*Path, error) {
	if strings.Count(arg, "/") > 1 {
		return nil, fmt.Errorf("invalid project target %q", arg)
	}
	return Parse(ctx, arg, ParseOptions{
		AllowImplicitOrg: true,
		RequireProject:   true,
	})
}

// ParseTarget validates and parses a target argument of form ORGANIZATION/TARGET or ORGANIZATION/PROJECT/TARGET.
func ParseTarget(target string) (*Path, error) {
	return Parse(nil, target, ParseOptions{
		AllowImplicitOrg: false,
		MinTargets:       1,
		MaxTargets:       1,
	})
}

// ParseTargetWithDefaultOrganization resolves a group-oriented target that allows an implicit organization by
// falling back to the configured default. Accepted formats are [ORGANIZATION/]GROUP and
// [ORGANIZATION/]PROJECT/GROUP (used by security membership commands where the middle segment is optional).
func ParseTargetWithDefaultOrganization(ctx CmdContext, target string) (*Path, error) {
	return Parse(ctx, target, ParseOptions{
		AllowImplicitOrg: true,
		MinTargets:       1,
		MaxTargets:       1,
	})
}

// ParseProjectTargetWithDefaultOrganization resolves targets that must include a project segment. It accepts
// arguments in the form [ORGANIZATION/]PROJECT/TARGET, falling back to the user's default organization when the
// organization segment is omitted.
func ParseProjectTargetWithDefaultOrganization(ctx CmdContext, target string) (*Path, error) {
	return Parse(ctx, target, ParseOptions{
		AllowImplicitOrg: true,
		RequireProject:   true,
		MinTargets:       1,
		MaxTargets:       1,
	})
}

// ParsePoolAgentTargetWithDefaultOrganization resolves a pool/agent target that allows an implicit
// organization by falling back to the configured default. Accepted formats are
// [ORGANIZATION/]POOL/AGENT (2 or 3 segments).
func ParsePoolAgentTargetWithDefaultOrganization(ctx CmdContext, raw string) (*Path, error) {
	return Parse(ctx, raw, ParseOptions{
		AllowImplicitOrg: true,
		MinTargets:       2,
		MaxTargets:       2,
	})
}

// ResolveScopeDescriptor fetches the descriptor representing the project scope when a project is supplied.
// It returns the descriptor value along with the project ID string to support callers that need to distinguish
// between identically named groups scoped to different projects.
func ResolveScopeDescriptor(ctx CmdContext, organization, project string) (*string, *string, error) {
	if project == "" {
		return nil, nil, nil
	}
	if strings.TrimSpace(organization) == "" {
		return nil, nil, fmt.Errorf("organization is required")
	}

	coreClient, err := ctx.ClientFactory().Core(ctx.Context(), organization)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create core client: %w", err)
	}

	projectRef, err := coreClient.GetProject(ctx.Context(), core.GetProjectArgs{
		ProjectId: types.ToPtr(project),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get project: %w", err)
	}
	if projectRef == nil || projectRef.Id == nil {
		return nil, nil, fmt.Errorf("project storage key is missing")
	}

	graphClient, err := ctx.ClientFactory().Graph(ctx.Context(), organization)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create graph client: %w", err)
	}

	descriptor, err := graphClient.GetDescriptor(ctx.Context(), graph.GetDescriptorArgs{
		StorageKey: projectRef.Id,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get project descriptor: %w", err)
	}
	if descriptor == nil || descriptor.Value == nil || *descriptor.Value == "" {
		return nil, nil, fmt.Errorf("project descriptor is empty")
	}

	projectID := types.ToPtr(projectRef.Id.String())
	return descriptor.Value, projectID, nil
}

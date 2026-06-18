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
//     treats it as exactly MinTargets.
//
// Parse only supports at most two non-target prefix segments: optional organization and
// optional project. Targets are always the trailing segments and the prefix is the
// leading segments.
//
// When MinTargets == MaxTargets the target count is fixed, so the prefix is simply the
// remaining leading segments (0, 1, or 2). When MaxTargets > MinTargets the split would
// otherwise be ambiguous, so Parse pins the prefix to its maximum shape (organization +
// project) and treats every remaining segment as a target. In that mode both
// organization and project must be present in the path.
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
// Error conditions:
//   - opts are invalid, for example MaxTargets is non-zero and smaller than MinTargets
//   - any segment is empty after trimming, for example "org/" or "org/ /project"
//   - total segment count (and therefore the resulting target count) falls outside the
//     allowed range derived from opts
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

	// Validate and normalize the allowed target range.
	minTargets := opts.MinTargets
	maxTargets := opts.MaxTargets
	if maxTargets == 0 {
		maxTargets = minTargets
	}
	if minTargets < 0 || maxTargets < minTargets {
		return nil, fmt.Errorf("invalid options: target range [%d,%d] is not satisfiable", opts.MinTargets, opts.MaxTargets)
	}

	// Allowed prefix (organization + project) segment counts.
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
	maxTotal := maxPrefix + maxTargets

	// Determine the prefix length.
	//
	// Fixed target count: the prefix is whatever leads the fixed-size target tail.
	// Variable target count: pin the prefix to its maximum shape so the split is
	// unambiguous, and let the remaining segments be targets.
	var prefixLen int
	if minTargets == maxTargets {
		prefixLen = n - minTargets
	} else {
		prefixLen = maxPrefix
	}
	targetCount := n - prefixLen

	if prefixLen < minPrefix || prefixLen > maxPrefix || targetCount < minTargets || targetCount > maxTargets {
		return nil, fmt.Errorf("invalid input %q: expected %d-%d segments, got %d", raw, minTotal, maxTotal, n)
	}

	p := &Path{}
	if targetCount > 0 {
		p.Targets = make([]string, targetCount)
		copy(p.Targets, parts[prefixLen:])
	}

	switch prefixLen {
	case 0:
	case 1:
		if opts.RequireProject {
			p.Project = parts[0]
		} else {
			p.Organization = parts[0]
		}
	case 2:
		p.Organization = parts[0]
		p.Project = parts[1]
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
	scope, err := Parse(ctx, arg, ParseOptions{
		AllowImplicitOrg: true,
	})
	if err != nil {
		return "", err
	}
	if scope.Project != "" {
		return "", fmt.Errorf("project scope not allowed for this command")
	}
	return scope.Organization, nil
}

// ParseProjectScope parses arguments in the form [ORGANIZATION/]PROJECT. When the organization
// segment is omitted the default organization from the user's configuration is used. The function
// trims whitespace around individual segments and ensures the resulting values are non-empty.
func ParseProjectScope(ctx CmdContext, arg string) (*Path, error) {
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

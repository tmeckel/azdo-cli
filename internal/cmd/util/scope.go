package util

import (
	"fmt"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
	"github.com/tmeckel/azdo-cli/internal/types"
)

// Path represents a parsed user-input scope of the form
// [ORG:][PROJECT/]TARGET[/TARGET...] or [ORG:]/TARGET[/TARGET...].
// Organization is always populated after a successful Parse, except when
// DisallowOrganization is set, in which case it is left empty.
type Path struct {
	Organization string
	Project      string
	Targets      []string
}

// ParseOptions configures how raw command input is split into a Path.
//
// The parser is shape-driven, never heuristic: an explicit "ORG:" prefix
// carries the organization, a leading "/" marks the no-project form, and a
// bare leading segment is always a project unless AllowBareTargets
// reclassifies it as a target. The options select which shapes a mode accepts
// and how many targets it expects; they never change what a shape means.
//
// The options fall into three axes plus the two target-count bounds:
//
//	Organization:
//	  AllowImplicitOrg       ORG: may be omitted; when omitted, Parse loads the
//	                         default organization from
//	                         ctx.Config().Authentication().GetDefaultOrganization().
//	  DisallowOrganization   ORG: is rejected and no organization is resolved;
//	                         Path.Organization is left empty.
//	  -> Mutually exclusive.
//
//	Project:
//	  RequireProject         the project-first form PROJECT/TARGET... is
//	                         required; the "/" no-project form is rejected.
//	  DisallowProject        only the no-project form /TARGET... is accepted;
//	                         any project segment is rejected.
//	  -> Mutually exclusive. With both unset, both forms are accepted.
//
//	Targets:
//	  MinTargets             minimum trailing target count.
//	  MaxTargets             maximum trailing target count; 0 means unbounded.
//	  DisallowTargets        rejects inputs that carry target segments. This is
//	                         the only way to express "no targets", because
//	                         MaxTargets == 0 means unbounded.
//	  AllowBareTargets       reclassifies a lone segment without the "/" marker
//	                         as a bare target instead of a project.
//	  -> AllowBareTargets cannot be combined with RequireProject,
//	     DisallowProject, or DisallowTargets.
//
// Contradictory combinations are rejected by validateParseOptions before any
// input is parsed. The wrappers below (ParseScope, ParseTarget, ...) are the
// intended entry points; most callers should pick one instead of composing
// ParseOptions by hand.
type ParseOptions struct {
	// AllowImplicitOrg allows the ORG: prefix to be omitted. When omitted, Parse
	// loads the default organization from ctx.Config().Authentication().GetDefaultOrganization().
	AllowImplicitOrg bool
	// RequireProject requires one project segment before any target segments.
	RequireProject bool
	// DisallowProject rejects inputs that carry a project segment. Inputs must
	// use the leading-slash no-project marker, for example "/POOL/AGENT" or
	// "ORG:/POOL/AGENT".
	DisallowProject bool
	// DisallowOrganization rejects inputs that carry an explicit ORG: prefix.
	// When set, no organization is resolved at all — not even the configured
	// default — and Path.Organization is left empty. It cannot be combined
	// with AllowImplicitOrg. Combined with DisallowProject it accepts only
	// the no-project form, for example "/TARGET".
	DisallowOrganization bool
	// AllowBareTargets reclassifies a lone segment without the "/" marker as a
	// target instead of a project. It cannot be combined with RequireProject,
	// DisallowProject, or DisallowTargets.
	AllowBareTargets bool
	// DisallowTargets rejects inputs that carry target segments. It must be set
	// whenever a wrapper accepts no targets at all; MaxTargets cannot express
	// this because MaxTargets == 0 means unbounded.
	DisallowTargets bool
	// MinTargets is the required number of trailing target segments.
	MinTargets int
	// MaxTargets is the maximum number of trailing target segments. Zero means
	// unbounded; use DisallowTargets to reject targets entirely.
	MaxTargets int
}

// Parse splits raw command input into a Path using deterministic Azure
// DevOps-style scope rules. Heuristics never decide whether a segment is an
// organization, project, or target: an explicit "ORG:" prefix carries the
// organization and a leading "/" marks the no-project form.
//
// # Input grammar
//
// The input is trimmed, then an optional ORG: prefix is recognized, then the
// remainder is split on "/" into trimmed segments (empty segments are
// rejected). After the organization prefix the grammar has three shapes:
//
//	PROJECT/TARGET...   project-first form; the first segment is the project
//	/TARGET...          no-project form; every segment is a target
//	TARGET              bare-target form; a lone segment counts as a target
//	                    instead of a project when AllowBareTargets is set
//
// # How the options select valid shapes
//
//	AllowImplicitOrg      ORG: optional; falls back to the default organization
//	DisallowOrganization  ORG: forbidden; organization never resolved
//	RequireProject        project-first form required
//	DisallowProject       no-project form required
//	DisallowTargets       target segments forbidden
//	MinTargets/MaxTargets trailing target count range; 0 means unbounded
//	AllowBareTargets      lone segment is a target, not a project
//
// Project and organization options are mutually exclusive within their axes;
// see ParseOptions for the full compatibility rules. With no project option
// set, both project-first and no-project forms are accepted. Ambiguous inputs
// such as a legacy "ORG/SUBJECT" are interpreted as canonical project-first
// forms and never auto-detected as an organization; structurally detectable
// legacy organization forms are rejected with ORG: guidance.
//
// # Examples
//
//	Parse(ctx, "org:/group", ParseOptions{AllowImplicitOrg: false, MinTargets: 1, MaxTargets: 1})
//	// => &Path{Organization: "org", Targets: []string{"group"}}
//
//	Parse(nil, "org:project/group", ParseOptions{AllowImplicitOrg: false, MinTargets: 1, MaxTargets: 1})
//	// => &Path{Organization: "org", Project: "project", Targets: []string{"group"}}
//
//	Parse(ctx, "/group", ParseOptions{AllowImplicitOrg: true, MinTargets: 1, MaxTargets: 1})
//	// => &Path{Organization: <default org>, Targets: []string{"group"}}
//
//	Parse(ctx, "project/group", ParseOptions{AllowImplicitOrg: true, MinTargets: 1, MaxTargets: 1})
//	// => &Path{Organization: <default org>, Project: "project", Targets: []string{"group"}}
//
//	Parse(ctx, "project", ParseOptions{AllowImplicitOrg: true, DisallowTargets: true})
//	// => &Path{Organization: <default org>, Project: "project"}
//
//	Parse(ctx, "org:", ParseOptions{AllowImplicitOrg: true, DisallowTargets: true})
//	// => &Path{Organization: "org"}
//
//	Parse(ctx, "/pool/agent", ParseOptions{AllowImplicitOrg: true, DisallowProject: true, MinTargets: 2, MaxTargets: 2})
//	// => &Path{Organization: <default org>, Targets: []string{"pool", "agent"}}
//
//	Parse(nil, "project/5", ParseOptions{DisallowOrganization: true, RequireProject: true, MinTargets: 1, MaxTargets: 1})
//	// => &Path{Project: "project", Targets: []string{"5"}}
//
//	Parse(nil, "5678", ParseOptions{DisallowOrganization: true, AllowBareTargets: true, MinTargets: 1, MaxTargets: 1})
//	// => &Path{Targets: []string{"5678"}}
//
// # Error conditions
//
//   - opts are invalid: a negative target count, MaxTargets below MinTargets,
//     DisallowTargets combined with a positive target bound, RequireProject
//     combined with DisallowProject, DisallowOrganization combined with
//     AllowImplicitOrg, or AllowBareTargets combined with RequireProject,
//     DisallowProject, or DisallowTargets
//   - the input contains multiple or misplaced colons, an empty ORG: prefix, or
//     an empty segment after trimming, for example "org:project/" or "org:/ /x"
//   - the input carries an explicit ORG: prefix while DisallowOrganization is set
//   - the input shape violates the project rule or the target range, including
//     a bare target when AllowBareTargets is unset
//   - an explicit organization is required but the ORG: prefix is missing
//   - a structurally detectable legacy ORGANIZATION/... form is used without ORG:
//   - "ORG:" is used in a mode that accepts targets instead of the "ORG:/" marker
//   - organization is omitted but ctx is nil, or the default organization lookup
//     fails or returns empty
func Parse(ctx CmdContext, raw string, opts ParseOptions) (*Path, error) {
	if err := validateParseOptions(opts); err != nil {
		return nil, err
	}

	org, hasOrg, noProject, segments, err := splitScope(raw)
	if err != nil {
		return nil, err
	}

	// An explicit ORG: prefix is never allowed when organizations are
	// disallowed for this mode.
	if hasOrg && opts.DisallowOrganization {
		return nil, fmt.Errorf("invalid input %q: organization is not allowed (expected %s)", raw, expectedForms(opts))
	}

	// An explicit ORG: prefix is mandatory when the mode does not allow the
	// default organization.
	if !hasOrg && !opts.AllowImplicitOrg && !opts.DisallowOrganization {
		return nil, fmt.Errorf("invalid input %q: explicit organization is required, use ORG: syntax (expected %s)", raw, expectedForms(opts))
	}

	var project string
	var targets []string
	if noProject {
		if opts.RequireProject {
			return nil, fmt.Errorf("invalid input %q: project is required (expected %s)", raw, expectedForms(opts))
		}
		if opts.DisallowTargets {
			return nil, fmt.Errorf("invalid input %q: targets are not allowed (expected %s)", raw, expectedForms(opts))
		}
		targets = segments
	} else {
		if len(segments) > 0 {
			project = segments[0]
		}
		if len(segments) > 1 {
			targets = segments[1:]
		}
		// A lone segment is a bare target instead of a project when the mode
		// opts into AllowBareTargets.
		if opts.AllowBareTargets && len(segments) == 1 {
			project = ""
			targets = segments
		}
		if opts.DisallowProject && len(segments) > 0 {
			return nil, fmt.Errorf("invalid input %q: project is not allowed, use the / no-project marker (expected %s)", raw, expectedForms(opts))
		}
		if opts.RequireProject && len(segments) == 0 {
			return nil, fmt.Errorf("invalid input %q: project is required (expected %s)", raw, expectedForms(opts))
		}
		// Reject structurally detectable legacy organization/slash forms. A
		// form is only classified as legacy when no colon is present and the
		// segment count exceeds every canonical shape of the mode: a second
		// segment in a no-target mode, or more than project plus the fixed
		// target count. Other non-colon inputs follow canonical project-first
		// parsing.
		legacy := !hasOrg && len(segments) > 0 &&
			((opts.DisallowTargets && len(segments) > 1) ||
				(opts.MaxTargets > 0 && opts.MinTargets == opts.MaxTargets && len(segments) > opts.MaxTargets+1))
		if legacy {
			guidance := "legacy ORGANIZATION/... form is not supported, use ORG: syntax"
			if opts.DisallowOrganization {
				guidance = "organization is not allowed"
			}
			return nil, fmt.Errorf("invalid input %q: %s (expected %s)", raw, guidance, expectedForms(opts))
		}
		// Organization-only input requires the "/" marker whenever targets are
		// allowed, so the explicit no-project shape is unambiguous.
		if hasOrg && len(segments) == 0 && !opts.DisallowTargets {
			return nil, fmt.Errorf("invalid input %q: use ORG:/ to specify an organization without a project or targets", raw)
		}
	}

	if opts.DisallowTargets && len(targets) > 0 {
		return nil, fmt.Errorf("invalid input %q: targets are not allowed (expected %s)", raw, expectedForms(opts))
	}
	if len(targets) < opts.MinTargets {
		return nil, targetCountError(raw, opts, len(targets))
	}
	if opts.MaxTargets > 0 && len(targets) > opts.MaxTargets {
		return nil, targetCountError(raw, opts, len(targets))
	}

	if org == "" && !opts.DisallowOrganization {
		var err error
		org, err = defaultOrganization(ctx)
		if err != nil {
			return nil, err
		}
	}

	p := &Path{
		Organization: org,
		Project:      project,
		Targets:      targets,
	}
	return p, nil
}

// validateParseOptions rejects option combinations that cannot describe a valid
// parse mode.
func validateParseOptions(opts ParseOptions) error {
	if opts.MinTargets < 0 || opts.MaxTargets < 0 {
		return fmt.Errorf("invalid options: target counts must not be negative")
	}
	if opts.MaxTargets > 0 && opts.MaxTargets < opts.MinTargets {
		return fmt.Errorf("invalid options: target range [%d,%d] is not satisfiable", opts.MinTargets, opts.MaxTargets)
	}
	if opts.DisallowTargets && opts.MinTargets > 0 {
		return fmt.Errorf("invalid options: targets are disallowed but min targets is %d", opts.MinTargets)
	}
	if opts.DisallowTargets && opts.MaxTargets > 0 {
		return fmt.Errorf("invalid options: targets are disallowed but max targets is %d", opts.MaxTargets)
	}
	if opts.RequireProject && opts.DisallowProject {
		return fmt.Errorf("invalid options: project cannot be required and disallowed at the same time")
	}
	if opts.DisallowOrganization && opts.AllowImplicitOrg {
		return fmt.Errorf("invalid options: organization cannot be disallowed when the implicit organization is allowed")
	}
	if opts.AllowBareTargets && opts.RequireProject {
		return fmt.Errorf("invalid options: bare targets cannot be combined with a required project")
	}
	if opts.AllowBareTargets && opts.DisallowProject {
		return fmt.Errorf("invalid options: bare targets cannot be combined with a disallowed project")
	}
	if opts.AllowBareTargets && opts.DisallowTargets {
		return fmt.Errorf("invalid options: bare targets cannot be combined with disallowed targets")
	}
	return nil
}

// expectedForms describes the canonical input shapes of a mode for error
// messages, so every syntax error points users to the ORG: and "/" forms. ORG:
// alternatives are omitted when organizations are disallowed for the mode.
func expectedForms(opts ParseOptions) string {
	if opts.DisallowOrganization {
		switch {
		case opts.DisallowTargets:
			return "PROJECT"
		case opts.DisallowProject:
			return "/TARGET..."
		case opts.RequireProject:
			return "PROJECT/TARGET..."
		default:
			return targetForms(opts)
		}
	}
	switch {
	case opts.DisallowTargets && opts.RequireProject:
		return "PROJECT or ORG:PROJECT"
	case opts.DisallowTargets && opts.DisallowProject:
		return "ORG or ORG:"
	case opts.DisallowTargets:
		return "PROJECT, ORG:PROJECT, or ORG:"
	case opts.DisallowProject:
		return "/TARGET... or ORG:/TARGET..."
	case opts.RequireProject:
		return "PROJECT/TARGET... or ORG:PROJECT/TARGET..."
	default:
		return targetForms(opts)
	}
}

// targetForms lists the target-bearing shapes, including the bare-target form
// when AllowBareTargets is set.
func targetForms(opts ParseOptions) string {
	bare := ""
	if opts.AllowBareTargets {
		bare = "TARGET, "
	}
	if opts.DisallowOrganization {
		return bare + "/TARGET... or PROJECT/TARGET..."
	}
	return bare + "/TARGET..., PROJECT/TARGET..., ORG:/TARGET..., or ORG:PROJECT/TARGET..."
}

// targetCountError builds the target cardinality error for a mode.
func targetCountError(raw string, opts ParseOptions, got int) error {
	forms := expectedForms(opts)
	switch {
	case opts.MaxTargets == 0:
		return fmt.Errorf("invalid input %q: expected at least %d targets, got %d (expected %s)", raw, opts.MinTargets, got, forms)
	case opts.MinTargets == opts.MaxTargets:
		return fmt.Errorf("invalid input %q: expected exactly %d targets, got %d (expected %s)", raw, opts.MinTargets, got, forms)
	default:
		return fmt.Errorf("invalid input %q: expected %d-%d targets, got %d (expected %s)", raw, opts.MinTargets, opts.MaxTargets, got, forms)
	}
}

// splitScope trims raw input and splits it into an optional explicit
// organization (ORG: prefix), an optional no-project marker, and the remaining
// slash-separated segments. Segments are trimmed individually; empty segments,
// multiple colons, colons not directly following the organization, and an empty
// organization prefix are rejected.
func splitScope(raw string) (org string, hasOrg bool, noProject bool, segments []string, err error) {
	trimmed := strings.TrimSpace(raw)

	if idx := strings.Index(trimmed, ":"); idx >= 0 {
		if strings.Contains(trimmed[idx+1:], ":") {
			return "", false, false, nil, fmt.Errorf("invalid input %q: contains multiple colons", raw)
		}
		if strings.Contains(trimmed[:idx], "/") {
			return "", false, false, nil, fmt.Errorf("invalid input %q: colon must directly follow the organization", raw)
		}
		org = strings.TrimSpace(trimmed[:idx])
		if org == "" {
			return "", false, false, nil, fmt.Errorf("invalid input %q: organization must not be empty", raw)
		}
		hasOrg = true
		trimmed = trimmed[idx+1:]
	}

	noProject = strings.HasPrefix(trimmed, "/")
	if noProject {
		trimmed = strings.TrimPrefix(trimmed, "/")
	}

	if trimmed != "" {
		parts := strings.Split(trimmed, "/")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				return "", false, false, nil, fmt.Errorf("input %q contains empty segment", raw)
			}
			segments = append(segments, part)
		}
	}

	return org, hasOrg, noProject, segments, nil
}

// defaultOrganization resolves the configured default organization. The
// returned error and wrapping behavior are shared by every parser that allows
// an implicit organization.
func defaultOrganization(ctx CmdContext) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("no organization specified and no default organization configured")
	}
	cfg, err := ctx.Config()
	if err != nil {
		return "", err
	}
	org, err := cfg.Authentication().GetDefaultOrganization()
	if err != nil {
		return "", fmt.Errorf("no organization specified and no default organization configured: %w", err)
	}
	org = strings.TrimSpace(org)
	if org == "" {
		return "", fmt.Errorf("no organization specified and no default organization configured")
	}
	return org, nil
}

// ParseScope resolves the organization and optional project from an input
// argument. Accepted forms are PROJECT, ORG:, and ORG:PROJECT; an empty input
// resolves to the default organization from the user configuration. Bare
// segments are project-first, so a bare ORG is parsed as a project.
func ParseScope(ctx CmdContext, scope string) (*Path, error) {
	return Parse(ctx, scope, ParseOptions{
		AllowImplicitOrg: true,
		DisallowTargets:  true,
	})
}

// ParseOrganizationArg resolves the organization from an organization-only
// argument of the form ORG or ORG:. When the input is empty, the default
// organization from the user configuration is returned. This is the only
// wrapper that classifies a bare segment as an organization; structured
// wrappers never do.
func ParseOrganizationArg(ctx CmdContext, arg string) (string, error) {
	org, hasOrg, noProject, segments, err := splitScope(arg)
	if err != nil {
		return "", err
	}
	switch {
	case noProject:
		return "", fmt.Errorf("invalid organization %q: expected ORG or ORG: syntax", arg)
	case hasOrg:
		if len(segments) > 0 {
			return "", fmt.Errorf("invalid organization %q: project scope not allowed for this command", arg)
		}
	case len(segments) > 1:
		return "", fmt.Errorf("invalid organization %q: project scope not allowed for this command", arg)
	case len(segments) == 1:
		org = segments[0]
	}
	if org == "" {
		return defaultOrganization(ctx)
	}
	return org, nil
}

// ParseProjectScope parses arguments of the form PROJECT or ORG:PROJECT. When
// the organization prefix is omitted the default organization from the user's
// configuration is used. Legacy ORGANIZATION/PROJECT inputs are rejected with
// ORG: guidance.
func ParseProjectScope(ctx CmdContext, arg string) (*Path, error) {
	return Parse(ctx, arg, ParseOptions{
		AllowImplicitOrg: true,
		RequireProject:   true,
		DisallowTargets:  true,
	})
}

// ParseTarget validates and parses a target argument of the form ORG:/TARGET or
// ORG:PROJECT/TARGET. An explicit organization is required.
func ParseTarget(target string) (*Path, error) {
	return Parse(nil, target, ParseOptions{
		AllowImplicitOrg: false,
		MinTargets:       1,
		MaxTargets:       1,
	})
}

// ParseTargetWithDefaultOrganization resolves a target that allows an implicit
// organization by falling back to the configured default. Accepted forms are
// /TARGET, PROJECT/TARGET, ORG:/TARGET, and ORG:PROJECT/TARGET. A legacy
// ORG/TARGET input is parsed as a project-first PROJECT/TARGET.
func ParseTargetWithDefaultOrganization(ctx CmdContext, target string) (*Path, error) {
	return Parse(ctx, target, ParseOptions{
		AllowImplicitOrg: true,
		MinTargets:       1,
		MaxTargets:       1,
	})
}

// ParseProjectTargetWithDefaultOrganization resolves targets that must include
// a project segment. Accepted forms are PROJECT/TARGET and ORG:PROJECT/TARGET,
// falling back to the user's default organization when the organization prefix
// is omitted.
func ParseProjectTargetWithDefaultOrganization(ctx CmdContext, target string) (*Path, error) {
	return Parse(ctx, target, ParseOptions{
		AllowImplicitOrg: true,
		RequireProject:   true,
		MinTargets:       1,
		MaxTargets:       1,
	})
}

// ParseProjectPathTargetWithDefaultOrganization resolves a project-scoped path
// target that allows an implicit organization by falling back to the configured
// default. Accepted forms are PROJECT/PATH and ORG:PROJECT/PATH. PATH may
// itself contain '/' and is split into Target segments; callers rejoin them
// with "/" to recover the full path.
func ParseProjectPathTargetWithDefaultOrganization(ctx CmdContext, raw string) (*Path, error) {
	return Parse(ctx, raw, ParseOptions{
		AllowImplicitOrg: true,
		RequireProject:   true,
		MinTargets:       1,
	})
}

// ParsePoolAgentTargetWithDefaultOrganization resolves a pool/agent target that
// allows an implicit organization by falling back to the configured default.
// Accepted forms are /POOL/AGENT and ORG:/POOL/AGENT; the no-project marker is
// required because this mode disallows projects.
func ParsePoolAgentTargetWithDefaultOrganization(ctx CmdContext, raw string) (*Path, error) {
	return Parse(ctx, raw, ParseOptions{
		AllowImplicitOrg: true,
		DisallowProject:  true,
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

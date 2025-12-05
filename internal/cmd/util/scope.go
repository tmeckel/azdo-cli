package util

import (
	"fmt"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
	"github.com/tmeckel/azdo-cli/internal/types"
)

// Scope describes the organization/project resolution for commands that accept optional project scope.
type Scope struct {
	Organization string
	Project      string
}

// ParseScope resolves the organization and optional project from an input argument of the form
// "[ORGANIZATION[/PROJECT]]". When the input is empty, the default organization from the user configuration
// is returned. The function trims whitespace around individual segments and ensures the resulting values are
// non-empty when provided.
func ParseScope(ctx CmdContext, scope string) (*Scope, error) {
	result := &Scope{}

	trimmed := strings.TrimSpace(scope)
	if trimmed == "" {
		cfg, err := ctx.Config()
		if err != nil {
			return nil, err
		}
		org, err := cfg.Authentication().GetDefaultOrganization()
		if err != nil {
			return nil, fmt.Errorf("no organization specified and no default organization configured: %w", err)
		}
		result.Organization = org
		return result, nil
	}

	parts := strings.Split(trimmed, "/")
	switch len(parts) {
	case 1:
		org := strings.TrimSpace(parts[0])
		if org == "" {
			return nil, fmt.Errorf("invalid scope format: %s", scope)
		}
		result.Organization = org
	case 2:
		org := strings.TrimSpace(parts[0])
		project := strings.TrimSpace(parts[1])
		if org == "" || project == "" {
			return nil, fmt.Errorf("invalid scope format: %s", scope)
		}
		result.Organization = org
		result.Project = project
	default:
		return nil, fmt.Errorf("invalid scope format: %s", scope)
	}

	return result, nil
}

// ParseOrganizationArg resolves the organization from an input argument of the form "[ORGANIZATION]".
// When the input is empty, the default organization from the user configuration is returned.
func ParseOrganizationArg(ctx CmdContext, arg string) (string, error) {
	scope, err := ParseScope(ctx, arg)
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
func ParseProjectScope(ctx CmdContext, arg string) (*Scope, error) {
	result := &Scope{}
	parts := strings.Split(strings.TrimSpace(arg), "/")
	switch len(parts) {
	case 1:
		result.Project = strings.TrimSpace(parts[0])
		if result.Project == "" {
			return nil, fmt.Errorf("project argument cannot be empty")
		}
		cfg, err := ctx.Config()
		if err != nil {
			return nil, fmt.Errorf("failed to read configuration: %w", err)
		}
		org, err := cfg.Authentication().GetDefaultOrganization()
		org = strings.TrimSpace(org)
		if err != nil || org == "" {
			return nil, fmt.Errorf("no organization specified and no default organization configured")
		}
		result.Organization = org
		return result, nil
	case 2:
		result.Organization = strings.TrimSpace(parts[0])
		result.Project = strings.TrimSpace(parts[1])
		if result.Organization == "" || result.Project == "" {
			return nil, fmt.Errorf("invalid project argument %q; expected format ORGANIZATION/PROJECT", arg)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("invalid project argument %q; expected format ORGANIZATION/PROJECT", arg)
	}
}

// ResolveScopeDescriptor fetches the descriptor representing the project scope when a project is supplied.
// It returns the descriptor value along with the project ID string to support callers that need to distinguish
// between identically named groups scoped to different projects.
func ResolveScopeDescriptor(ctx CmdContext, organization, project string) (*string, *string, error) {
	if project == "" {
		return nil, nil, nil
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

	var projectID *string
	if projectRef.Id != nil {
		id := projectRef.Id.String()
		projectID = &id
	}

	return descriptor.Value, projectID, nil
}

type Target struct {
	Scope
	Target string
}

// ParseTarget validates and parses a target argument of form ORGANIZATION/TARGET or ORGANIZATION/PROJECT/TARGET.
func ParseTarget(target string) (*Target, error) {
	return parseTarget(nil, target, targetParseOptions{
		allowImplicitOrg: false,
		requireProject:   false,
	})
}

// ParseTargetWithDefaultOrganization resolves a group-oriented target that allows an implicit organization by
// falling back to the configured default. Accepted formats are [ORGANIZATION/]GROUP and
// [ORGANIZATION/]PROJECT/GROUP (used by security membership commands where the middle segment is optional).
func ParseTargetWithDefaultOrganization(ctx CmdContext, target string) (*Target, error) {
	return parseTarget(ctx, target, targetParseOptions{
		allowImplicitOrg: true,
		requireProject:   false,
	})
}

// ParseProjectTargetWithDefaultOrganization resolves targets that must include a project segment. It accepts
// arguments in the form [ORGANIZATION/]PROJECT/TARGET, falling back to the user's default organization when the
// organization segment is omitted.
func ParseProjectTargetWithDefaultOrganization(ctx CmdContext, target string) (*Target, error) {
	return parseTarget(ctx, target, targetParseOptions{
		allowImplicitOrg: true,
		requireProject:   true,
	})
}

type targetParseOptions struct {
	allowImplicitOrg bool
	requireProject   bool
}

func parseTarget(ctx CmdContext, raw string, opts targetParseOptions) (*Target, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("target must not be empty")
	}

	parts := strings.Split(trimmed, "/")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}

	var org, project, targetValue string
	switch len(parts) {
	case 1:
		if !opts.allowImplicitOrg || opts.requireProject {
			return nil, fmt.Errorf("invalid target format: %s", raw)
		}
		targetValue = parts[0]
	case 2:
		if opts.requireProject {
			project = parts[0]
			targetValue = parts[1]
		} else {
			org = parts[0]
			targetValue = parts[1]
		}
	case 3:
		org = parts[0]
		project = parts[1]
		targetValue = parts[2]
	default:
		return nil, fmt.Errorf("invalid target format: %s", raw)
	}

	if targetValue == "" {
		return nil, fmt.Errorf("invalid target format: %s", raw)
	}
	if opts.requireProject && project == "" {
		return nil, fmt.Errorf("invalid target format: %s", raw)
	}

	if org == "" {
		if !opts.allowImplicitOrg {
			return nil, fmt.Errorf("invalid target format: %s", raw)
		}
		if ctx == nil {
			return nil, fmt.Errorf("no organization specified and no default organization configured")
		}
		cfg, err := ctx.Config()
		if err != nil {
			return nil, err
		}
		authCfg := cfg.Authentication()
		org, err = authCfg.GetDefaultOrganization()
		if err != nil {
			return nil, fmt.Errorf("no organization specified and no default organization configured: %w", err)
		}
		org = strings.TrimSpace(org)
		if org == "" {
			return nil, fmt.Errorf("no organization specified and no default organization configured")
		}
	}

	return &Target{
		Scope: Scope{
			Organization: org,
			Project:      project,
		},
		Target: targetValue,
	}, nil
}

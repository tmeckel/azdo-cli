package shared

import (
	"fmt"
	"strings"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

type SubjectTarget struct {
	util.Scope
	Subject string
}

// ParseSubjectTarget parses the target input for permission commands.
// Accepted formats:
//   - (empty)                        → defaults organization from configuration
//   - ORGANIZATION                   → organization scope only
//   - ORGANIZATION/SUBJECT           → organization with explicit subject
//   - ORGANIZATION/PROJECT/SUBJECT   → project-scoped subject
func ParseSubjectTarget(ctx util.CmdContext, input string) (*SubjectTarget, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		scope, err := util.ParseScope(ctx, "")
		if err != nil {
			return nil, err
		}
		return &SubjectTarget{
			Scope: *scope,
		}, nil
	}

	segments := strings.Split(trimmed, "/")
	if len(segments) == 0 || len(segments) > 3 {
		return nil, util.FlagErrorf("invalid target %q", input)
	}

	orgPart := strings.TrimSpace(segments[0])
	if orgPart == "" {
		return nil, util.FlagErrorf("organization must not be empty")
	}

	switch len(segments) {
	case 1:
		scope, err := util.ParseScope(ctx, orgPart)
		if err != nil {
			return nil, err
		}
		return &SubjectTarget{
			Scope:   *scope,
			Subject: "",
		}, nil
	case 2:
		subject := strings.TrimSpace(segments[1])
		if subject == "" {
			return nil, util.FlagErrorf("subject must not be empty")
		}
		scope, err := util.ParseScope(ctx, orgPart)
		if err != nil {
			return nil, err
		}
		return &SubjectTarget{
			Scope:   *scope,
			Subject: subject,
		}, nil
	case 3:
		project := strings.TrimSpace(segments[1])
		subject := strings.TrimSpace(segments[2])
		if project == "" {
			return nil, util.FlagErrorf("project must not be empty")
		}
		if subject == "" {
			return nil, util.FlagErrorf("subject must not be empty")
		}
		scopeInput := fmt.Sprintf("%s/%s", orgPart, project)
		scope, err := util.ParseScope(ctx, scopeInput)
		if err != nil {
			return nil, err
		}
		return &SubjectTarget{
			Scope: *scope,
		}, nil
	default:
		return nil, util.FlagErrorf("invalid target %q", input)
	}
}

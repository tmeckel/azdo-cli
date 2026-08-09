package shared

import (
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

type SubjectTarget struct {
	util.Path
	Subject string
}

// ParseSubjectTarget parses the target input for permission commands using the
// shared structural scope parser.
//
// Accepted forms:
//   - (empty)               → default organization, no subject
//   - ORG:/                 → organization scope only, no subject
//   - /SUBJECT              → organization-level subject in the default organization
//   - ORG:/SUBJECT          → organization-level subject in an explicit organization
//   - PROJECT/SUBJECT       → project-scoped subject in the default organization
//   - ORG:PROJECT/SUBJECT   → project-scoped subject in an explicit organization
//
// A legacy two-segment input such as ORG/SUBJECT is indistinguishable from the
// canonical PROJECT/SUBJECT form, so it is never rejected and never treated as
// an explicit organization: it parses as a project-scoped subject in the
// default organization. Organization-level subjects must use the ORG:/SUBJECT
// or /SUBJECT forms. Bare ORG is likewise parsed as a project, so the
// organization-only form requires the colon prefix (ORG:/).
func ParseSubjectTarget(ctx util.CmdContext, input string) (*SubjectTarget, error) {
	path, err := util.Parse(ctx, input, util.ParseOptions{
		AllowImplicitOrg: true,
		MinTargets:       0,
		MaxTargets:       1,
	})
	if err != nil {
		return nil, err
	}

	target := &SubjectTarget{Path: *path}
	if len(path.Targets) > 0 {
		target.Subject = path.Targets[0]
	}
	return target, nil
}

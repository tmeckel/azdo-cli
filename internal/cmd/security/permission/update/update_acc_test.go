package update

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/security"

	"github.com/tmeckel/azdo-cli/internal/cmd/security/permission/shared"
	inttest "github.com/tmeckel/azdo-cli/internal/test"
)

func TestAccUpdatePermission(t *testing.T) {
	// static values chosen for acceptance run — these are well-known namespace/token patterns
	// repoV2 namespace id (example from docs/data in repo). Adjust as needed for real org.
	namespaceID := "2e9eb7ed-3c0a-47d4-87c1-0ffdd275fd87"
	nsUUID := uuid.MustParse(namespaceID)
	token := "repoV2"

	var groupDescriptor string
	var groupIdentity string
	groupName := fmt.Sprintf("azdo-cli-test-group-%s", uuid.New().String())

	inttest.Test(t, inttest.TestCase{
		Steps: []inttest.Step{
			{
				PreRun: func(ctx inttest.TestContext) error {
					grph, err := ctx.ClientFactory().Graph(ctx.Context(), ctx.Org())
					if err != nil {
						return err
					}
					ext, err := ctx.ClientFactory().Extensions(ctx.Context(), ctx.Org())
					if err != nil {
						return err
					}

					group, err := grph.CreateGroupVsts(ctx.Context(), graph.CreateGroupVstsArgs{
						CreationContext: &graph.GraphGroupVstsCreationContext{
							DisplayName: &groupName,
						},
					})
					if err != nil {
						return fmt.Errorf("failed to create test group: %w", err)
					}
					groupDescriptor = *group.Descriptor

					identity, err := ext.ResolveIdentity(ctx.Context(), groupDescriptor)
					if err != nil {
						return err
					}
					groupIdentity = *identity.Descriptor
					return nil
				},
				Run: func(ctx inttest.TestContext) error {
					o := &opts{
						rawTarget:   fmt.Sprintf("%s/%s", ctx.Org(), groupDescriptor),
						namespaceID: namespaceID,
						token:       token,
						allowBits:   []string{"0x2"},
						denyBits:    []string{},
						merge:       false,
						yes:         true,
					}
					return runCommand(ctx, o)
				},
				Verify: func(ctx inttest.TestContext) error {
					sec, err := ctx.ClientFactory().Security(ctx.Context(), ctx.Org())
					if err != nil {
						return err
					}
					return inttest.Poll(func() error {
						ace, err := shared.FindAccessControlEntry(ctx.Context(), sec, nsUUID, token, groupIdentity)
						if err != nil {
							return err
						}
						if ace == nil {
							return fmt.Errorf("ace for descriptor %q (Identity: %q) not found", groupDescriptor, groupIdentity)
						}
						if ace.Allow == nil {
							return fmt.Errorf("ace allow is nil; expected bit 0x2")
						}
						if *ace.Allow&0x2 == 0 {
							return fmt.Errorf("allow mask %d does not contain expected bit 0x2", *ace.Allow)
						}
						return nil
					}, inttest.PollOptions{
						Tries:   10,
						Timeout: 30 * time.Second,
					})
				},
				PostRun: func(ctx inttest.TestContext) error {
					var errs []error

					sec, err := ctx.ClientFactory().Security(ctx.Context(), ctx.Org())
					if err != nil {
						return err
					}

					isRemoved, err := sec.RemoveAccessControlEntries(ctx.Context(), security.RemoveAccessControlEntriesArgs{
						SecurityNamespaceId: &nsUUID,
						Token:               &token,
						Descriptors:         &groupIdentity,
					})
					if err != nil {
						errs = append(errs, fmt.Errorf("failed to remove ACE: %w", err))
					}
					if !*isRemoved {
						errs = append(errs, fmt.Errorf("failed to remove ACE: status false"))
					}

					grph, err := ctx.ClientFactory().Graph(ctx.Context(), ctx.Org())
					if err != nil {
						return err
					}
					err = grph.DeleteGroup(ctx.Context(), graph.DeleteGroupArgs{
						GroupDescriptor: &groupDescriptor,
					})
					if err != nil {
						errs = append(errs, fmt.Errorf("failed to delete test group: %w", err))
					}

					if len(errs) > 0 {
						return errors.Join(errs...)
					}
					return nil
				},
			},
		},
	})
}

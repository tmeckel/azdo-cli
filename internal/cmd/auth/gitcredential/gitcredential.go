package gitcredential

import (
	"bufio"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

type credentialOptions struct {
	operation string
}

func NewCmdGitCredential(ctx util.CmdContext) *cobra.Command {
	opts := &credentialOptions{}

	cmd := &cobra.Command{
		Use:    "git-credential",
		Args:   cobra.ExactArgs(1),
		Short:  "Implements git credential helper protocol",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.operation = args[0]
			return helperRun(ctx, opts)
		},
	}

	return cmd
}

func helperRun(ctx util.CmdContext, opts *credentialOptions) (err error) {
	if opts.operation == "store" {
		// We pretend to implement the "store" operation, but do nothing since we already have a cached token.
		return nil
	}

	if opts.operation == "erase" {
		// We pretend to implement the "erase" operation, but do nothing since we don't want git to cause user to be logged out.
		return nil
	}

	if opts.operation != "get" {
		return fmt.Errorf("azdo auth git-credential: %q operation not supported", opts.operation)
	}

	cfg, err := ctx.Config()
	if err != nil {
		return util.FlagErrorf("error getting configuration: %w", err)
	}

	iostrms, err := ctx.IOStreams()
	if err != nil {
		return util.FlagErrorf("error getting io streams: %w", err)
	}
	wants := map[string]string{}

	s := bufio.NewScanner(iostrms.In)
	for s.Scan() {
		line := s.Text()
		if line == "" {
			break
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) < 2 {
			continue
		}
		key, value := parts[0], parts[1]
		if key == "url" {
			u, err := url.Parse(value)
			if err != nil {
				return err
			}
			wants["protocol"] = u.Scheme
			wants["host"] = u.Host
			wants["path"] = u.Path
			wants["username"] = u.User.Username()
			wants["password"], _ = u.User.Password()
		} else {
			wants[key] = value
		}
	}

	if err := s.Err(); err != nil {
		return err
	}

	if wants["protocol"] != "https" {
		return fmt.Errorf("procotol %s != https", wants["protocol"])
	}

	var organizationName string
	lookupHost := strings.ToLower(wants["host"])
	if strings.Contains(lookupHost, ".visualstudio.com") {
		organizationName = strings.Split(lookupHost, ".")[0]
	} else if lookupHost == "dev.azure.com" {
		if path, ok := wants["path"]; !ok {
			return fmt.Errorf("authenticating via dev.azure.com host requires path parameter")
		} else {
			organizationName = strings.Split(path, "/")[0]
		}
	} else {
		return fmt.Errorf("not an Azure DevOps host %s", lookupHost)
	}

	if organizationName == "" {
		return fmt.Errorf("unable to get token from host %s or path %s", wants["host"], wants["path"])
	}
	auth := cfg.Authentication()
	gotToken, err := auth.GetToken(organizationName)
	if err != nil || gotToken == "" {
		return fmt.Errorf("unable to get token for organization %s", organizationName)
	}

	fmt.Fprint(iostrms.Out, "protocol=https\n")
	fmt.Fprintf(iostrms.Out, "host=%s\n", wants["host"])
	fmt.Fprintf(iostrms.Out, "username=azdo\n")
	fmt.Fprintf(iostrms.Out, "password=%s\n", gotToken)

	return nil
}

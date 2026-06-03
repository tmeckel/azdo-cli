package gitcredential

import (
	"bufio"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
	cmdutil "github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/util"

	"go.uber.org/zap"
)

type credentialOptions struct {
	operation string
}

func NewCmdGitCredential(ctx cmdutil.CmdContext) *cobra.Command {
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

func helperRun(ctx cmdutil.CmdContext, opts *credentialOptions) (err error) {
	zap.L().Debug("helper started", zap.String("operation", opts.operation))

	if opts.operation == "store" {
		zap.L().Debug("store operation - no action needed")
		// We pretend to implement the "store" operation, but do nothing since we already have a cached token.
		return nil
	}

	if opts.operation == "erase" {
		zap.L().Debug("erase operation - no action needed")
		// We pretend to implement the "erase" operation, but do nothing since we don't want git to cause user to be logged out.
		return nil
	}

	if opts.operation != "get" {
		zap.L().Debug("unsupported operation", zap.String("operation", opts.operation))
		return fmt.Errorf("azdo auth git-credential: %q operation not supported", opts.operation)
	}

	cfg, err := ctx.Config()
	if err != nil {
		zap.L().Debug("config error", zap.Error(err))
		return cmdutil.FlagErrorf("error getting configuration: %w", err)
	}

	iostrms, err := ctx.IOStreams()
	if err != nil {
		zap.L().Debug("io streams error", zap.Error(err))
		return cmdutil.FlagErrorf("error getting io streams: %w", err)
	}
	wants := map[string]string{}

	zap.L().Debug("parsing input")
	s := bufio.NewScanner(iostrms.In)
	lineNum := 0
	for s.Scan() {
		lineNum++
		line := s.Text()
		zap.L().Debug("read line", zap.Int("line", lineNum), zap.String("raw", line))
		if line == "" {
			zap.L().Debug("empty line detected, stopping input parsing")
			break
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) < 2 {
			zap.L().Debug("skipping malformed line", zap.Int("line", lineNum), zap.String("raw", line))
			continue
		}
		key, value := parts[0], parts[1]
		zap.L().Debug("parsed key-value", zap.Int("line", lineNum), zap.String("key", key), zap.String("value", value))
		if key == "url" {
			zap.L().Debug("parsing url", zap.String("url", value))
			u, err := url.Parse(value)
			if err != nil {
				zap.L().Debug("url parse error", zap.Error(err))
				return err
			}
			wants["protocol"] = u.Scheme
			wants["host"] = u.Host
			wants["path"] = u.Path
			wants["username"] = u.User.Username()
			wants["password"], _ = u.User.Password()
			zap.L().Debug("url components extracted",
				zap.String("protocol", u.Scheme),
				zap.String("host", u.Host),
				zap.String("path", u.Path),
				zap.String("username", u.User.Username()))
		} else {
			wants[key] = value
		}
	}

	if err := s.Err(); err != nil {
		zap.L().Debug("scanner error", zap.Error(err))
		return err
	}

	if u, ok := wants["username"]; !ok || len(strings.TrimSpace(u)) == 0 {
		wants["username"] = util.StringBuilderString.MustGenerate(10)
		zap.L().Debug("generated artificial username", zap.String("username", wants["username"]))
	}

	zap.L().Debug("parsed input", zap.Any("wants", wants), zap.Int("total_keys", len(wants)))

	if wants["protocol"] != "https" {
		protocol := wants["protocol"]
		if protocol == "" {
			protocol = "(empty)"
		}
		zap.L().Debug("protocol not https", zap.String("protocol", protocol))
		return fmt.Errorf("protocol %s != https", wants["protocol"])
	}

	var organizationName string
	lookupHost := strings.ToLower(wants["host"])
	zap.L().Debug("detecting organization", zap.String("host", lookupHost), zap.String("path", wants["path"]))
	if strings.Contains(lookupHost, ".visualstudio.com") { //nolint:golint,gocritic
		organizationName = strings.Split(lookupHost, ".")[0]
		zap.L().Debug("organization from visualstudio.com", zap.String("organization", organizationName))
	} else if lookupHost == "dev.azure.com" {
		if path, ok := wants["path"]; !ok {
			zap.L().Debug("dev.azure.com host requires path")
			return fmt.Errorf("authenticating via dev.azure.com host requires path parameter")
		} else { //nolint:golint,revive
			organizationName = strings.Split(path, "/")[0]
			zap.L().Debug("organization from dev.azure.com", zap.String("organization", organizationName))
		}
	} else {
		zap.L().Debug("not an Azure DevOps host", zap.String("host", lookupHost))
		return fmt.Errorf("not an Azure DevOps host %s", lookupHost)
	}

	if organizationName == "" {
		zap.L().Debug("unable to extract organization", zap.String("host", wants["host"]), zap.String("path", wants["path"]))
		return fmt.Errorf("unable to get token from host %s or path %s", wants["host"], wants["path"])
	}
	auth := cfg.Authentication()
	gotToken, err := auth.GetToken(organizationName)
	if err != nil || gotToken == "" {
		zap.L().Debug("token retrieval failed", zap.String("organization", organizationName), zap.Error(err))
		return fmt.Errorf("unable to get token for organization %s", organizationName)
	}

	zap.L().Debug("outputting credentials", zap.String("host", wants["host"]))
	fmt.Fprint(iostrms.Out, "protocol=https\n")
	fmt.Fprintf(iostrms.Out, "host=%s\n", wants["host"])
	fmt.Fprintf(iostrms.Out, "username=%s\n", wants["username"])
	fmt.Fprintf(iostrms.Out, "password=%s\n", gotToken)

	return nil
}

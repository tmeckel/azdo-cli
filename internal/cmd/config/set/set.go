package set

import (
	"errors"
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/pterm/pterm"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/config"
)

type setOptions struct {
	key              string
	value            string
	organizationName string
	remove           bool
}

func NewCmdConfigSet(ctx util.CmdContext) *cobra.Command {
	opts := &setOptions{}

	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Update configuration with a value for the given key",
		Example: heredoc.Doc(`
			$ azdo config set editor vim
			$ azdo config set editor "code --wait"
			$ azdo config set git_protocol ssh --organization myorg
			$ azdo config set prompt disabled
			$ azdo config set -r -o myorg git_protocol
		`),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("remove") {
				if !cmd.Flags().Changed("organization") {
					return errors.New("configration values can only be removed for organizations. Please specify the organization via -o")
				}
				if len(args) != 1 {
					return fmt.Errorf("accepts %d arg(s), received %d", 1, len(args))
				}
			} else if len(args) != 2 {
				return fmt.Errorf("accepts %d arg(s), received %d", 2, len(args))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.key = args[0]
			if !opts.remove {
				opts.value = args[1]
			}

			return setRun(ctx, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.organizationName, "organization", "o", "", "Set per-organization setting")
	cmd.Flags().BoolVarP(&opts.remove, "remove", "r", false, "Remove config item for an organization, so that the default value will be in effect again")

	return cmd
}

func setRun(ctx util.CmdContext, opts *setOptions) (err error) {
	cfg, err := ctx.Config()
	if err != nil {
		return util.FlagErrorf("error getting io configuration: %w", err)
	}
	iostrms, err := ctx.IOStreams()
	if err != nil {
		return util.FlagErrorf("error getting io streams: %w", err)
	}

	err = validateKey(opts.key)
	if err != nil {
		fmt.Fprintf(iostrms.ErrOut, pterm.Warning.Sprintf("%q is not a known configuration key\n", opts.key))
	}

	if opts.organizationName != "" {
		if !lo.Contains(cfg.Authentication().GetOrganizations(), opts.organizationName) {
			fmt.Fprintf(
				iostrms.ErrOut,
				"You are not logged the Azure DevOps organization %q. Run %s to authenticate.\n",
				opts.organizationName, pterm.Bold.Sprint("azdo auth login"),
			)
			return util.ErrSilent
		}
	}

	if opts.remove {
		err = cfg.Remove([]string{config.Organizations, opts.organizationName, opts.key})
		if err != nil {
			if !errors.Is(err, &config.KeyNotFoundError{}) {
				return err
			}
			return nil // no need to write configuration because it didn't change
		}
	} else {
		err = validateValue(opts.key, opts.value)
		if err != nil {
			var invalidValue InvalidValueError
			if errors.As(err, &invalidValue) {
				var values []string
				for _, v := range invalidValue.ValidValues {
					values = append(values, fmt.Sprintf("'%s'", v))
				}
				return fmt.Errorf("failed to set %q to %q: valid values are %v", opts.key, opts.value, strings.Join(values, ", "))
			}
		}

		if opts.organizationName != "" {
			cfg.Set([]string{config.Organizations, opts.organizationName, opts.key}, opts.value)
		} else {
			cfg.Set([]string{opts.key}, opts.value)
		}
	}

	err = cfg.Write()
	if err != nil {
		return fmt.Errorf("failed to write config to disk: %w", err)
	}
	return
}

func validateKey(key string) error {
	for _, configKey := range config.Options() {
		if key == configKey.Key {
			return nil
		}
	}

	return fmt.Errorf("invalid key")
}

type InvalidValueError struct {
	ValidValues []string
}

func (e InvalidValueError) Error() string {
	return "invalid value"
}

func validateValue(key, value string) error {
	var validValues []string

	for _, v := range config.Options() {
		if v.Key == key {
			validValues = v.AllowedValues
			break
		}
	}

	if validValues == nil {
		return nil
	}

	for _, v := range validValues {
		if v == value {
			return nil
		}
	}

	return InvalidValueError{ValidValues: validValues}
}

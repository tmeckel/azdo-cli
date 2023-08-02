package set

import (
	"errors"
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/config"
)

type setOptions struct {
	Key              string
	Value            string
	OrganizationName string
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
		`),
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Key = args[0]
			opts.Value = args[1]

			return setRun(ctx, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.OrganizationName, "organization", "o", "", "Set per-organization setting")

	return cmd
}

func setRun(ctx util.CmdContext, opts *setOptions) (err error) {
	cfg, err := ctx.Config()
	if err != nil {
		return util.FlagErrorf("error getting io configuration: %w", err)
	}
	iostreams, err := ctx.IOStreams()
	if err != nil {
		return util.FlagErrorf("error getting io streams: %w", err)
	}

	err = validateKey(opts.Key)
	if err != nil {
		warningIcon := iostreams.ColorScheme().WarningIcon()
		fmt.Fprintf(iostreams.ErrOut, "%s warning: '%s' is not a known configuration key\n", warningIcon, opts.Key)
	}

	err = validateValue(opts.Key, opts.Value)
	if err != nil {
		var invalidValue InvalidValueError
		if errors.As(err, &invalidValue) {
			var values []string
			for _, v := range invalidValue.ValidValues {
				values = append(values, fmt.Sprintf("'%s'", v))
			}
			return fmt.Errorf("failed to set %q to %q: valid values are %v", opts.Key, opts.Value, strings.Join(values, ", "))
		}
	}

	if opts.OrganizationName != "" {
		cfg.Set([]string{config.Organizations, opts.OrganizationName, opts.Key}, opts.Value)
	} else {
		cfg.Set([]string{opts.Key}, opts.Value)
	}

	err = cfg.Write()
	if err != nil {
		return fmt.Errorf("failed to write config to disk: %w", err)
	}
	return
}

func validateKey(key string) error {
	for _, configKey := range config.ConfigOptions() {
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

	for _, v := range config.ConfigOptions() {
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

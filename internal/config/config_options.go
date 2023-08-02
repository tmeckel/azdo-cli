package config

import (
	"os"
	"path/filepath"
)

type ConfigOption struct {
	Key           string
	Description   string
	DefaultValue  string
	AllowedValues []string
}

var configOptions = []ConfigOption{
	{
		Key:           "git_protocol",
		Description:   "the protocol to use for git clone and push operations",
		DefaultValue:  "https",
		AllowedValues: []string{"https", "ssh"},
	},
	{
		Key:          "editor",
		Description:  "the text editor program to use for authoring text",
		DefaultValue: "",
	},
	{
		Key:           "prompt",
		Description:   "toggle interactive prompting in the terminal",
		DefaultValue:  "enabled",
		AllowedValues: []string{"enabled", "disabled"},
	},
	{
		Key:          "pager",
		Description:  "the terminal pager program to send standard output to",
		DefaultValue: "",
	},
	{
		Key:          "http_unix_socket",
		Description:  "the path to a Unix socket through which to make an HTTP connection",
		DefaultValue: "",
	},
	{
		Key:          "browser",
		Description:  "the web browser to use for opening URLs",
		DefaultValue: "",
	},
	{
		Key:          "default_organization",
		Description:  "the default Azure DevOps organization to use, if no organization is specified",
		DefaultValue: "",
	},
}

func ConfigOptions() []ConfigOption {
	return configOptions
}

func HomeDirPath(subdir string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	newPath := filepath.Join(homeDir, subdir)
	return newPath, nil
}

func defaultFor(key string) string {
	for _, co := range configOptions {
		if co.Key == key {
			return co.DefaultValue
		}
	}
	return ""
}

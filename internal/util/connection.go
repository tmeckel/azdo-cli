package util

import (
	"fmt"
	"regexp"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
)

var rxOrgURL = regexp.MustCompile(`//(dev\.azure\.com/(?P<organization>[^/]+)|(?P<organization>[^.]+)\.visualstudio\.com)`)

func GetOrganizationFromConnection(conn *azuredevops.Connection) (string, error) {
	match := rxOrgURL.FindStringSubmatch(conn.BaseUrl)
	if len(match) == 0 {
		return "", fmt.Errorf("invalid Azure DevOps URL %s", conn.BaseUrl)
	}
	return match[2] + match[3], nil
}

package shared

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
)

// TeamProjectField is the work item field that identifies the parent project.
const TeamProjectField = "System.TeamProject"

// BelongsToProject reports whether the work item's System.TeamProject field
// matches the given project. The comparison is case-insensitive, consistent
// with Azure DevOps project name behavior. Used to verify project ownership
// before mutating or deleting work items.
func BelongsToProject(item *workitemtracking.WorkItem, project string) bool {
	if item == nil || item.Fields == nil {
		return false
	}
	got, ok := (*item.Fields)[TeamProjectField]
	return ok && strings.EqualFold(fmt.Sprint(got), project)
}

// OpenURL opens a URL in the default browser: $BROWSER if set, otherwise the
// platform opener (xdg-open/open/rundll32). Empty URLs are ignored.
func OpenURL(raw string) error {
	if raw == "" {
		return nil
	}
	if browser := os.Getenv("BROWSER"); browser != "" {
		parts := strings.Fields(browser)
		cmd := exec.Command(parts[0], append(parts[1:], raw)...) //nolint:gosec // BROWSER env is an explicit user-chosen command
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", raw).Run()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", raw).Run()
	default:
		return exec.Command("xdg-open", raw).Run()
	}
}

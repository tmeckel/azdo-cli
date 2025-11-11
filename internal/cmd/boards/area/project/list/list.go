package list

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/tmeckel/azdo-cli/internal/cmd/boards/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type listOptions struct {
	scopeArg string
	path     string
	depth    int
	exporter util.Exporter
}

type workItemClassificationNode struct {
	ID          *int                         `json:"id,omitempty"`
	Identifier  *string                      `json:"identifier,omitempty"`
	Name        *string                      `json:"name,omitempty"`
	Path        *string                      `json:"path,omitempty"`
	Structure   *string                      `json:"structureType,omitempty"`
	HasChildren *bool                        `json:"hasChildren,omitempty"`
	Children    []workItemClassificationNode `json:"children,omitempty"`
	Attributes  map[string]any               `json:"attributes,omitempty"`
}

type areaNode struct {
	ID          int    `json:"id"`
	Identifier  string `json:"identifier,omitempty"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	HasChildren bool   `json:"hasChildren"`
	ParentPath  string `json:"parentPath,omitempty"`
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &listOptions{
		depth: 1,
	}

	cmd := &cobra.Command{
		Use:   "list [ORGANIZATION/]PROJECT",
		Short: "List area paths defined for a project.",
		Long: heredoc.Doc(`
			List Azure Boards area paths for a project. The project argument accepts the form
			[ORGANIZATION/]PROJECT. When the organization segment is omitted, the default
			organization from configuration is used.
		`),
		Example: heredoc.Doc(`
			# List the top-level area paths for Fabrikam using the default organization
			azdo boards area project list Fabrikam

			# List the full area tree for a project in a specific organization
			azdo boards area project list myorg/Fabrikam --depth 5

			# List the sub-tree under a specific area path (relative paths are resolved under <project>/Area)
			azdo boards area project list myorg/Fabrikam --path Payments --depth 3
		`),
		Aliases: []string{"ls", "l"},
		Args:    util.ExactArgs(1, "project argument required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.scopeArg = args[0]
			return runList(ctx, opts)
		},
	}

	cmd.Flags().StringVar(&opts.path, "path", "", "Restrict results to a specific area path (relative paths like \"Payments\" or \"Payments/Sub\" are resolved under <project>/Area).")
	cmd.Flags().IntVar(&opts.depth, "depth", 1, "Depth of child nodes to include (use 0 to omit child nodes).")
	util.AddJSONFlags(cmd, &opts.exporter, []string{"id", "identifier", "name", "path", "hasChildren", "parentPath"})

	return cmd
}

func runList(ctx util.CmdContext, opts *listOptions) error {
	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	if opts.depth < 0 {
		return util.FlagErrorf("--depth must be greater than or equal to 0")
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	scope, err := util.ParseProjectScope(ctx, opts.scopeArg)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	cfg, err := ctx.Config()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	orgURL, err := cfg.Authentication().GetURL(scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to resolve organization URL: %w", err)
	}

	conn, err := ctx.ConnectionFactory().Connection(scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create connection: %w", err)
	}

	endpoint, err := buildAreaEndpoint(strings.TrimRight(orgURL, "/"), scope.Project, opts.path)
	if err != nil {
		return err
	}

	reqURL, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("failed to parse request URL: %w", err)
	}
	if opts.depth > 0 {
		query := reqURL.Query()
		query.Set("$depth", strconv.Itoa(opts.depth))
		reqURL.RawQuery = query.Encode()
	}

	client := conn.GetClientByUrl(strings.TrimRight(orgURL, "/"))
	req, err := client.CreateRequestMessage(ctx.Context(), http.MethodGet, reqURL.String(), "7.1", nil, "", azuredevops.MediaTypeApplicationJson, nil)
	if err != nil {
		return fmt.Errorf("failed to construct request: %w", err)
	}

	zap.L().Debug("listing project area paths",
		zap.String("organization", scope.Organization),
		zap.String("project", scope.Project),
		zap.String("path", strings.TrimSpace(opts.path)),
		zap.Int("depth", opts.depth),
		zap.String("endpoint", reqURL.String()),
	)

	resp, err := client.SendRequest(req)
	if err != nil {
		return fmt.Errorf("failed to fetch area paths: %w", err)
	}
	defer resp.Body.Close()

	var root workItemClassificationNode
	if err := client.UnmarshalBody(resp, &root); err != nil {
		return fmt.Errorf("failed to decode area paths response: %w", err)
	}

	var nodes []areaNode
	flattenAreaNodes(&root, "", &nodes)

	if len(nodes) == 0 {
		return util.NewNoResultsError("no area paths found")
	}

	sort.Slice(nodes, func(i, j int) bool {
		return strings.ToLower(nodes[i].Path) < strings.ToLower(nodes[j].Path)
	})

	if opts.exporter != nil {
		ios.StopProgressIndicator()
		return opts.exporter.Write(ios, nodes)
	}

	tp, err := ctx.Printer("table")
	if err != nil {
		return err
	}

	tp.AddColumns("Name", "Path", "HasChildren")
	for _, n := range nodes {
		tp.AddField(n.Name)
		tp.AddField(n.Path)
		if n.HasChildren {
			tp.AddField("true")
		} else {
			tp.AddField("false")
		}
		tp.EndRow()
	}

	ios.StopProgressIndicator()

	return tp.Render()
}

func buildAreaEndpoint(baseURL, project, path string) (string, error) {
	projectSegment := url.PathEscape(project)
	endpoint := fmt.Sprintf("%s/%s/_apis/wit/classificationnodes/Areas", baseURL, projectSegment)

	normalized, err := shared.BuildClassificationPath(project, true, "Area", path)
	if err != nil {
		return "", util.FlagErrorf("invalid path: %w", err)
	}
	if normalized != "" {
		endpoint = endpoint + "/" + normalized
	}
	return endpoint, nil
}

func flattenAreaNodes(node *workItemClassificationNode, parent string, rows *[]areaNode) {
	if node == nil {
		return
	}

	name := types.GetValue(node.Name, "")
	rawPath := types.GetValue(node.Path, "")
	path := shared.NormalizeClassificationPath(rawPath)
	id := types.GetValue(node.ID, 0)
	identifier := types.GetValue(node.Identifier, "")
	hasChildren := types.GetValue(node.HasChildren, false)

	row := areaNode{
		ID:          id,
		Identifier:  identifier,
		Name:        name,
		Path:        path,
		HasChildren: hasChildren,
	}
	if parent != "" {
		row.ParentPath = shared.NormalizeClassificationPath(parent)
	}
	*rows = append(*rows, row)

	if len(node.Children) == 0 {
		return
	}
	for i := range node.Children {
		child := &node.Children[i]
		flattenAreaNodes(child, rawPath, rows)
	}
}

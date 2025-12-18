package create

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	u "unicode"
	"unicode/utf8"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"

	"github.com/tmeckel/azdo-cli/internal/cmd/serviceendpoint/create/azurerm"
	"github.com/tmeckel/azdo-cli/internal/cmd/serviceendpoint/create/github"
	"github.com/tmeckel/azdo-cli/internal/cmd/serviceendpoint/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type fromFileOptions struct {
	scope    string
	fromFile string
	encoding string
	exporter util.Exporter
}

var encodingAliases = map[string]string{
	"utf-8":    "utf-8",
	"utf8":     "utf-8",
	"ascii":    "ascii",
	"utf-16be": "utf-16be",
	"utf16be":  "utf-16be",
	"utf-16le": "utf-16le",
	"utf16le":  "utf-16le",
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &fromFileOptions{
		encoding: "utf-8",
	}

	cmd := &cobra.Command{
		Use:   "create [ORGANIZATION/]PROJECT --from-file <path> [flags]",
		Short: "Create service connections",
		Long: heredoc.Doc(`
			Create Azure DevOps service endpoints (service connections) from a JSON definition file.

			The project scope accepts the form [ORGANIZATION/]PROJECT. When the organization segment
			is omitted the default organization from configuration is used.

			Check the available subcommands to create service connections of specific well-known types.
		`),
		Example: heredoc.Doc(`
			# Create a service endpoint from a UTF-8 JSON file
			azdo service-endpoint create my-org/my-project --from-file ./endpoint.json

			# Read the definition from stdin using UTF-16LE encoding
			cat endpoint.json | azdo service-endpoint create my-org/my-project --from-file - --encoding utf-16le
		`),
		Aliases: []string{
			"import",
		},
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.scope = args[0]
			return runCreateFromFile(ctx, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.fromFile, "from-file", "f", "", "Path to the JSON service endpoint definition or '-' for stdin.")
	cmd.Flags().StringVarP(&opts.encoding, "encoding", "e", opts.encoding, "File encoding (utf-8, ascii, utf-16be, utf-16le).")
	util.AddJSONFlags(cmd, &opts.exporter, []string{
		"administratorsGroup",
		"authorization",
		"createdBy",
		"data",
		"description",
		"groupScopeId",
		"id",
		"isReady",
		"isShared",
		"name",
		"operationStatus",
		"owner",
		"readersGroup",
		"serviceEndpointProjectReferences",
		"type",
		"url",
	})

	_ = cmd.MarkFlagRequired("from-file")

	cmd.AddCommand(azurerm.NewCmd(ctx))

	cmd.AddCommand(github.NewCmd(ctx))

	return cmd
}

func runCreateFromFile(ctx util.CmdContext, opts *fromFileOptions) error {
	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	scope, err := util.ParseProjectScope(ctx, opts.scope)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	encodingValue, err := normalizeEncoding(opts.encoding)
	if err != nil {
		return util.FlagErrorWrap(err)
	}
	opts.encoding = encodingValue

	projectRef, err := shared.ResolveProjectReference(ctx, scope)
	if err != nil {
		return err
	}

	zap.L().Debug("Creating service endpoint from file",
		zap.String("organization", scope.Organization),
		zap.String("project", scope.Project),
		zap.String("input", describeInput(opts.fromFile)),
		zap.String("encoding", opts.encoding),
	)

	content, err := util.ReadFile(opts.fromFile, ios.In)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", describeInput(opts.fromFile), err)
	}

	decoded, err := decodeContent(content, opts.encoding)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	var endpoint serviceendpoint.ServiceEndpoint
	if err := json.Unmarshal(decoded, &endpoint); err != nil {
		return util.FlagErrorf("failed to parse service endpoint JSON: %w", err)
	}

	if err := validateEndpointPayload(&endpoint); err != nil {
		return util.FlagErrorWrap(err)
	}

	if projectRef != nil {
		refs := []serviceendpoint.ServiceEndpointProjectReference{
			{
				ProjectReference: projectRef,
				Name:             endpoint.Name,
				Description:      endpoint.Description,
			},
		}
		endpoint.ServiceEndpointProjectReferences = &refs
	}

	client, err := ctx.ClientFactory().ServiceEndpoint(ctx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create service endpoint client: %w", err)
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	createdEndpoint, err := client.CreateServiceEndpoint(ctx.Context(), serviceendpoint.CreateServiceEndpointArgs{
		Endpoint: &endpoint,
	})
	if err != nil {
		return fmt.Errorf("failed to create service endpoint: %w", err)
	}

	ios.StopProgressIndicator()

	if opts.exporter != nil {
		return opts.exporter.Write(ios, createdEndpoint)
	}

	tp, err := ctx.Printer("list")
	if err != nil {
		return err
	}
	tp.AddColumns("ID", "Name", "Type", "URL")
	tp.EndRow()
	tp.AddField(types.GetValue(createdEndpoint.Id, uuid.Nil).String())
	tp.AddField(types.GetValue(createdEndpoint.Name, ""))
	tp.AddField(types.GetValue(createdEndpoint.Type, ""))
	tp.AddField(types.GetValue(createdEndpoint.Url, ""))
	tp.EndRow()
	tp.Render()

	return nil
}

func normalizeEncoding(value string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return "utf-8", nil
	}
	if normalized, ok := encodingAliases[trimmed]; ok {
		return normalized, nil
	}
	return "", fmt.Errorf("unsupported encoding %q; supported values: utf-8, ascii, utf-16be, utf-16le", value)
}

func describeInput(path string) string {
	if path == "-" {
		return "stdin"
	}
	return path
}

func decodeContent(raw []byte, encodingName string) ([]byte, error) {
	var dec *encoding.Decoder
	switch encodingName {
	case "utf-8":
		if !utf8.Valid(raw) {
			return nil, fmt.Errorf("input is not valid UTF-8")
		}
		return raw, nil
	case "ascii":
		for i, b := range raw {
			if b > u.MaxASCII {
				return nil, fmt.Errorf("input contains non-ASCII byte at offset %d", i)
			}
		}
		return raw, nil
	case "utf-16le":
		if err := validateUTF16(raw, binary.LittleEndian); err != nil {
			return nil, err
		}
		dec = unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder()
	case "utf-16be":
		if err := validateUTF16(raw, binary.BigEndian); err != nil {
			return nil, err
		}
		dec = unicode.UTF16(unicode.BigEndian, unicode.UseBOM).NewDecoder()
	default:
		return nil, fmt.Errorf("unsupported encoding %q", encodingName)
	}
	raw, err := dec.Bytes(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to decode input: %w", err)
	}
	return raw, nil
}

func validateEndpointPayload(endpoint *serviceendpoint.ServiceEndpoint) error {
	if endpoint == nil {
		return errors.New("service endpoint payload is empty")
	}
	name := strings.TrimSpace(types.GetValue(endpoint.Name, ""))
	if name == "" {
		return fmt.Errorf("service endpoint JSON missing 'name'")
	}
	typeValue := strings.TrimSpace(types.GetValue(endpoint.Type, ""))
	if typeValue == "" {
		return fmt.Errorf("service endpoint JSON missing 'type'")
	}
	urlValue := strings.TrimSpace(types.GetValue(endpoint.Url, ""))
	if urlValue == "" {
		return fmt.Errorf("service endpoint JSON missing 'url'")
	}
	endpoint.Name = types.ToPtr(name)
	endpoint.Type = types.ToPtr(typeValue)
	endpoint.Url = types.ToPtr(urlValue)
	endpoint.Id = nil
	return nil
}

func validateUTF16(raw []byte, order binary.ByteOrder) error {
	if len(raw)%2 != 0 {
		return fmt.Errorf("failed to decode input: invalid UTF-16 sequence length")
	}
	if len(raw) == 0 {
		return nil
	}
	units := make([]uint16, len(raw)/2)
	for i := range units {
		units[i] = order.Uint16(raw[2*i:])
	}
	start := 0
	if units[0] == 0xFEFF {
		start = 1
	}
	for i := start; i < len(units); {
		val := units[i]
		switch {
		case val >= 0xD800 && val <= 0xDBFF:
			if i+1 >= len(units) {
				return fmt.Errorf("failed to decode input: invalid UTF-16 surrogate pair")
			}
			next := units[i+1]
			if next < 0xDC00 || next > 0xDFFF {
				return fmt.Errorf("failed to decode input: invalid UTF-16 surrogate pair")
			}
			i += 2
		case val >= 0xDC00 && val <= 0xDFFF:
			return fmt.Errorf("failed to decode input: invalid UTF-16 surrogate pair")
		default:
			i++
		}
	}
	return nil
}

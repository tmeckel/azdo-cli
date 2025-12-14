package util

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/jq"
	"github.com/tmeckel/azdo-cli/internal/jsoncolor"
	"github.com/tmeckel/azdo-cli/internal/template"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type JSONFlagError struct {
	error
}

func AddJSONFlags(cmd *cobra.Command, exportTarget *Exporter, fields []string) {
	f := cmd.Flags()
	f.StringSlice("json", nil, "Output JSON with the specified `fields`. Prefix a field with '-' to exclude it.")
	f.StringP("jq", "q", "", "Filter JSON output using a jq `expression`")
	f.StringP("template", "t", "", "Format JSON output using a Go template; see \"azdo help formatting\"")

	f.Lookup("json").NoOptDefVal = jsonSelectAllSentinel

	_ = cmd.RegisterFlagCompletionFunc("json", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		var results []string
		var prefix string
		if idx := strings.LastIndexByte(toComplete, ','); idx >= 0 {
			prefix = toComplete[:idx+1]
			toComplete = toComplete[idx+1:]
		}
		toComplete = strings.ToLower(toComplete)
		for _, f := range fields {
			if strings.HasPrefix(strings.ToLower(f), toComplete) {
				results = append(results, prefix+f)
			}
		}
		sort.Strings(results)
		return results, cobra.ShellCompDirectiveNoSpace
	})

	oldPreRun := cmd.PreRunE
	cmd.PreRunE = func(c *cobra.Command, args []string) error {
		if oldPreRun != nil {
			if err := oldPreRun(c, args); err != nil {
				return err
			}
		}
		if export, err := checkJSONFlags(c); err == nil {
			if export == nil {
				*exportTarget = nil
			} else {
				resolved, err := resolveJSONSelection(export.Fields(), fields)
				if err != nil {
					return err
				}
				export.SetFields(resolved)
				*exportTarget = export
			}
		} else {
			return err
		}
		return nil
	}

	cmd.SetFlagErrorFunc(func(c *cobra.Command, e error) error {
		if cmd.HasParent() {
			return cmd.Parent().FlagErrorFunc()(c, e)
		}
		return e
	})

	if len(fields) == 0 {
		return
	}

	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations["help:json-fields"] = strings.Join(fields, ",")
}

const jsonSelectAllSentinel = "*"

func resolveJSONSelection(raw []string, allowed []string) ([]string, error) {
	if len(allowed) == 0 {
		return nil, JSONFlagError{fmt.Errorf("no JSON fields are defined for this command")}
	}

	result := slices.Clone(allowed)
	haveExplicitInclude := false

	allowedSet := types.NewStringSet()
	allowedSet.AddValues(allowed)

	for _, item := range raw {
		if item == "" || item == jsonSelectAllSentinel {
			// an empty entry or sentinel means "use defaults"
			continue
		}

		remove := false
		if strings.HasPrefix(item, "-") {
			remove = true
			item = strings.TrimPrefix(item, "-")
		}
		if item == "" {
			return nil, JSONFlagError{fmt.Errorf("invalid JSON field selector \"-\"")}
		}
		if !allowedSet.Contains(item) {
			sorted := slices.Clone(allowed)
			sort.Strings(sorted)
			return nil, JSONFlagError{fmt.Errorf("unknown JSON field: %q\navailable fields:\n  %s", item, strings.Join(sorted, "\n  "))}
		}

		if remove {
			result = removeJSONField(result, item)
			continue
		}

		if !haveExplicitInclude {
			result = result[:0]
			haveExplicitInclude = true
		}

		if !containsJSONField(result, item) {
			result = append(result, item)
		}
	}

	if !haveExplicitInclude {
		// Ensure exclusions above are reflected while preserving original order.
		result = filterAllowedBySelection(allowed, result)
	}

	if len(result) == 0 {
		return nil, JSONFlagError{fmt.Errorf("no JSON fields selected; all columns were excluded")}
	}

	return result, nil
}

func removeJSONField(fields []string, target string) []string {
	idx := slices.Index(fields, target)
	if idx == -1 {
		return fields
	}
	return append(fields[:idx], fields[idx+1:]...)
}

func containsJSONField(fields []string, target string) bool {
	return slices.Contains(fields, target)
}

func filterAllowedBySelection(allowed, selection []string) []string {
	if len(selection) == len(allowed) {
		return selection
	}

	selected := types.NewStringSet()
	selected.AddValues(selection)

	result := make([]string, 0, len(selection))
	for _, name := range allowed {
		if selected.Contains(name) {
			result = append(result, name)
		}
	}
	return result
}

func checkJSONFlags(cmd *cobra.Command) (*jsonExporter, error) {
	f := cmd.Flags()
	jsonFlag := f.Lookup("json")
	jqFlag := f.Lookup("jq")
	tplFlag := f.Lookup("template")
	webFlag := f.Lookup("web")

	if jsonFlag.Changed {
		if webFlag != nil && webFlag.Changed {
			return nil, errors.New("cannot use `--web` with `--json`")
		}
		jv := jsonFlag.Value.(pflag.SliceValue)
		return &jsonExporter{
			fields:   jv.GetSlice(),
			filter:   jqFlag.Value.String(),
			template: tplFlag.Value.String(),
		}, nil
	} else if jqFlag.Changed {
		return nil, errors.New("cannot use `--jq` without specifying `--json`")
	} else if tplFlag.Changed {
		return nil, errors.New("cannot use `--template` without specifying `--json`")
	}
	return nil, nil
}

func AddFormatFlags(cmd *cobra.Command, exportTarget *Exporter) {
	var format string
	StringEnumFlag(cmd, &format, "format", "", "", []string{"json"}, "Output format")
	f := cmd.Flags()
	f.StringP("jq", "q", "", "Filter JSON output using a jq `expression`")
	f.StringP("template", "t", "", "Format JSON output using a Go template; see \"azdo help formatting\"")

	oldPreRun := cmd.PreRunE
	cmd.PreRunE = func(c *cobra.Command, args []string) error {
		if oldPreRun != nil {
			if err := oldPreRun(c, args); err != nil {
				return err
			}
		}

		if export, err := checkFormatFlags(c); err == nil {
			if export == nil {
				*exportTarget = nil
			} else {
				*exportTarget = export
			}
		} else {
			return err
		}
		return nil
	}
}

func checkFormatFlags(cmd *cobra.Command) (*jsonExporter, error) {
	f := cmd.Flags()
	formatFlag := f.Lookup("format")
	formatValue := formatFlag.Value.String()
	jqFlag := f.Lookup("jq")
	tplFlag := f.Lookup("template")
	webFlag := f.Lookup("web")

	if formatFlag.Changed {
		if webFlag != nil && webFlag.Changed {
			return nil, errors.New("cannot use `--web` with `--format`")
		}
		return &jsonExporter{
			filter:   jqFlag.Value.String(),
			template: tplFlag.Value.String(),
		}, nil
	} else if jqFlag.Changed && formatValue != "json" {
		return nil, errors.New("cannot use `--jq` without specifying `--format json`")
	} else if tplFlag.Changed && formatValue != "json" {
		return nil, errors.New("cannot use `--template` without specifying `--format json`")
	}
	return nil, nil
}

type Exporter interface {
	Fields() []string
	Write(io *iostreams.IOStreams, data any) error
}

type jsonExporter struct {
	fields   []string // only print fields matching names from this array
	strict   bool     // use the struct field names instead of the names from the fields array
	filter   string   // jq expression
	template string   // GO template
}

// NewJSONExporter returns an Exporter to emit JSON.
func NewJSONExporter() *jsonExporter {
	return &jsonExporter{}
}

func (e *jsonExporter) Fields() []string {
	return e.fields
}

func (e *jsonExporter) SetFields(fields []string) {
	e.fields = fields
}

// Write serializes data into JSON output written to w. If the object passed as data implements exportable,
// or if data is a map or slice of exportable object, ExportData() will be called on each object to obtain
// raw data for serialization.
func (e *jsonExporter) Write(ios *iostreams.IOStreams, data any) error {
	data = e.exportData(reflect.ValueOf(data))

	buf := bytes.Buffer{}
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(data); err != nil {
		return err
	}

	if e.filter != "" {
		jsonData, err := io.ReadAll(&buf)
		if err != nil {
			return err
		}

		err = json.Unmarshal(jsonData, &data)
		if err != nil {
			return err
		}

		data, err = jq.EvaluateData(e.filter, data)
		if err != nil {
			return err
		}
		if err := encoder.Encode(data); err != nil {
			return err
		}
	}

	w := ios.Out
	if e.template != "" {
		t := template.New(w, ios.TerminalWidth(), ios.ColorEnabled()).
			WithTheme(ios.TerminalTheme())
		if err := t.Parse(e.template); err != nil {
			return err
		}
		if err := t.Execute(&buf); err != nil {
			return err
		}
		return t.Flush()
	} else if ios.ColorEnabled() {
		return jsoncolor.Write(w, &buf, "  ")
	}

	_, err := io.Copy(w, &buf)
	return err
}

func (e *jsonExporter) exportData(v reflect.Value) any {
	switch v.Kind() { //nolint:exhaustive
	case reflect.Ptr, reflect.Interface:
		if !v.IsNil() {
			return e.exportData(v.Elem())
		}
	case reflect.Slice:
		a := make([]any, v.Len())
		for i := 0; i < v.Len(); i++ {
			a[i] = e.exportData(v.Index(i))
		}
		return a
	case reflect.Map:
		t := reflect.MapOf(v.Type().Key(), emptyInterfaceType)
		m := reflect.MakeMapWithSize(t, v.Len())
		iter := v.MapRange()
		for iter.Next() {
			if len(e.fields) > 0 {
				kv := iter.Key()
				switch iter.Key().Kind() { //nolint:exhaustive
				case reflect.Interface:
					fallthrough
				case reflect.Pointer:
					kv = iter.Key().Elem()
				}
				if !kv.CanConvert(reflect.TypeFor[string]()) {
					continue
				}
				szKeyValue := kv.Convert(reflect.TypeFor[string]()).String()
				if !slices.ContainsFunc(e.fields, func(v string) bool {
					return strings.EqualFold(v, szKeyValue)
				}) {
					continue
				}
			}
			ve := reflect.ValueOf(e.exportData(iter.Value()))
			m.SetMapIndex(iter.Key(), ve)
		}
		return m.Interface()
	case reflect.Struct:
		if v.CanAddr() && reflect.PointerTo(v.Type()).Implements(exportableType) {
			ve := v.Addr().Interface().(exportable)
			return ve.ExportData(e.fields)
		} else if v.Type().Implements(exportableType) {
			ve := v.Interface().(exportable)
			return ve.ExportData(e.fields)
		} else {
			if len(e.fields) == 0 {
				return v.Interface()
			}
			return structExportData(v, e.fields, e.strict)
		}
	}
	return v.Interface()
}

type exportable interface {
	ExportData([]string) map[string]any
}

var (
	exportableType        = reflect.TypeOf((*exportable)(nil)).Elem()
	sliceOfEmptyInterface []any
	emptyInterfaceType    = reflect.TypeOf(sliceOfEmptyInterface).Elem()
)

func structExportData(v reflect.Value, fields []string, strict bool) map[string]any {
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		// If s is not a struct or pointer to a struct return nil.
		return nil
	}

	fieldList, fieldIndex := flattenStructFields(v)
	if len(fieldList) == 0 {
		return nil
	}

	allocate := len(fields)
	if allocate == 0 {
		allocate = len(fieldList)
	}
	data := make(map[string]any, allocate)

	emitField := func(fi structFieldInfo, nameOverride string) {
		sf := fi.value
		if !sf.IsValid() || !sf.CanInterface() {
			return
		}
		if fi.omitEmpty && sf.IsZero() {
			return
		}

		fieldName := fi.jsonName
		if fieldName == "" {
			fieldName = fi.structField.Name
		}
		if nameOverride != "" {
			fieldName = nameOverride
		}

		data[fieldName] = sf.Interface()
	}

	if len(fields) > 0 {
		for _, f := range fields {
			idx, ok := fieldIndex[strings.ToLower(f)]
			if !ok {
				continue
			}
			fi := fieldList[idx]
			name := ""
			if strict {
				name = fi.structField.Name
			} else {
				name = f
			}
			emitField(fi, name)
		}
		return data
	}

	for _, fi := range fieldList {
		emitField(fi, "")
	}

	return data
}

type structFieldInfo struct {
	value       reflect.Value
	structField reflect.StructField
	jsonName    string
	omitEmpty   bool
}

func flattenStructFields(v reflect.Value) ([]structFieldInfo, map[string]int) {
	fields := make([]structFieldInfo, 0)
	index := make(map[string]int)

	var walk func(reflect.Value)
	walk = func(val reflect.Value) {
		if val.Kind() == reflect.Ptr {
			if val.IsNil() {
				return
			}
			val = val.Elem()
		}
		if val.Kind() != reflect.Struct {
			return
		}

		t := val.Type()
		for i := 0; i < t.NumField(); i++ {
			sf := t.Field(i)

			fv := val.Field(i)

			if sf.Anonymous {
				if fv.Kind() == reflect.Ptr && fv.IsNil() {
					continue
				}
				if fv.Kind() == reflect.Ptr || fv.Kind() == reflect.Interface {
					fv = fv.Elem()
				}
				if fv.Kind() == reflect.Struct {
					walk(fv)
					continue
				}
			} else if sf.PkgPath != "" {
				continue
			}

			jsonTag := sf.Tag.Get("json")
			if jsonTag == "-" {
				continue
			}

			jsonName := ""
			omitEmpty := false
			if jsonTag != "" {
				parts := strings.Split(jsonTag, ",")
				if len(parts) > 0 {
					jsonName = parts[0]
				}
				for _, p := range parts[1:] {
					if p == "omitempty" {
						omitEmpty = true
						break
					}
				}
			}

			key := strings.ToLower(sf.Name)
			if jsonName != "" {
				key = strings.ToLower(jsonName)
			}

			if _, exists := index[key]; exists {
				continue
			}

			info := structFieldInfo{
				value:       fv,
				structField: sf,
				jsonName:    jsonName,
				omitEmpty:   omitEmpty,
			}
			index[key] = len(fields)
			fields = append(fields, info)
		}
	}

	walk(v)
	return fields, index
}

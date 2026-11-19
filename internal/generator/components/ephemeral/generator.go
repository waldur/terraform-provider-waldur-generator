// Package ephemeral generates Terraform ephemeral resources: a value that one
// API operation returns on every run and that Terraform never writes to the plan
// or the state. It suits secrets an operation reveals, such as an API key.
package ephemeral

import (
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/waldur/terraform-provider-waldur-generator/internal/config"
	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/common"
	"github.com/waldur/terraform-provider-waldur-generator/internal/openapi"
)

// PackageSuffix keeps an ephemeral resource in a package of its own. It often
// shares its name with a data source (the key and the revealed key), and the two
// would otherwise collide on their generated model and SDK types.
const PackageSuffix = "_ephemeral"

// PrepareData builds the ResourceData of an ephemeral resource from its
// operation: the response becomes computed attributes, and each path parameter
// becomes a required argument.
func PrepareData(parser *openapi.Parser, eph *config.EphemeralResource, schemaCfg common.SchemaConfig) (*common.ResourceData, error) {
	op, path, method, err := parser.GetOperation(eph.OperationID)
	if err != nil {
		return nil, fmt.Errorf("ephemeral resource %s: %w", eph.Name, err)
	}
	method = strings.ToUpper(method)
	if method != http.MethodGet && method != http.MethodPost {
		return nil, fmt.Errorf("ephemeral resource %s: %s is a %s operation; only GET and POST are supported", eph.Name, eph.OperationID, method)
	}
	if op.RequestBody != nil {
		return nil, fmt.Errorf("ephemeral resource %s: %s takes a request body, which ephemeral resources do not send", eph.Name, eph.OperationID)
	}

	responseSchema, err := parser.GetOperationResponseSchema(eph.OperationID)
	if err != nil {
		return nil, fmt.Errorf("ephemeral resource %s: %w", eph.Name, err)
	}
	responseFields, err := common.ExtractFields(schemaCfg, responseSchema, true)
	if err != nil {
		return nil, fmt.Errorf("ephemeral resource %s: %w", eph.Name, err)
	}
	common.ApplySchemaSkipRecursive(schemaCfg, responseFields, nil)
	markComputed(responseFields)
	sort.Slice(responseFields, func(i, j int) bool { return responseFields[i].Name < responseFields[j].Name })

	if err := markSensitive(responseFields, eph.SensitiveFields); err != nil {
		return nil, fmt.Errorf("ephemeral resource %s: %w", eph.Name, err)
	}

	pathParams, err := bindPathParams(op.Parameters, eph.PathParams, responseFields)
	if err != nil {
		return nil, fmt.Errorf("ephemeral resource %s: %s: %w", eph.Name, eph.OperationID, err)
	}

	modelFields := make([]common.FieldInfo, len(responseFields))
	for i, f := range responseFields {
		modelFields[i] = f.Clone()
	}

	service, cleanName := common.SplitResourceName(eph.Name)
	return &common.ResourceData{
		Name:                eph.Name,
		Service:             service,
		CleanName:           cleanName + PackageSuffix,
		ResponseFields:      responseFields,
		ModelFields:         modelFields,
		IsEphemeral:         true,
		EphemeralTypeName:   eph.Name,
		EphemeralMethod:     method,
		EphemeralPathParams: pathParams,
		APIPaths:            map[string]string{"Ephemeral": path},
	}, nil
}

// bindPathParams pairs every path parameter of the operation with the argument
// configured for it, failing on a parameter left unbound or a binding to no
// parameter: either would build a request to the wrong URL.
func bindPathParams(params openapi3.Parameters, bindings map[string]string, responseFields []common.FieldInfo) ([]common.EphemeralPathParam, error) {
	responseNames := make(map[string]bool, len(responseFields))
	for _, f := range responseFields {
		responseNames[f.Name] = true
	}

	inPath := make(map[string]bool)
	var out []common.EphemeralPathParam
	for _, ref := range params {
		if ref == nil || ref.Value == nil || ref.Value.In != "path" {
			continue
		}
		name := ref.Value.Name
		inPath[name] = true
		attribute, ok := bindings[name]
		if !ok || attribute == "" {
			return nil, fmt.Errorf("path parameter %q has no argument; add it to path_params", name)
		}
		if attribute != "id" && responseNames[attribute] {
			return nil, fmt.Errorf("path parameter %q: argument %q is also a response attribute; bind it to id or another name", name, attribute)
		}
		out = append(out, common.EphemeralPathParam{
			Param:     name,
			Attribute: attribute,
			FieldName: modelFieldName(attribute),
			ArgName:   goArgName(name),
			InModel:   attribute == "id",
		})
	}

	bound := make([]string, 0, len(bindings))
	for name := range bindings {
		bound = append(bound, name)
	}
	sort.Strings(bound)
	for _, name := range bound {
		if !inPath[name] {
			return nil, fmt.Errorf("path_params names %q, which is not a path parameter", name)
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Param < out[j].Param })
	return out, nil
}

// modelFieldName is the Go field holding an argument in the generated model.
// The model already carries the object's UUID as UUID, under the `id` attribute.
func modelFieldName(attribute string) string {
	if attribute == "id" {
		return "UUID"
	}
	return common.ToTitle(attribute)
}

// goArgName turns a path parameter into a Go parameter name: resource_uuid
// becomes resourceUuidParam, which cannot collide with a keyword or an import.
func goArgName(param string) string {
	title := common.ToTitle(param)
	return strings.ToLower(title[:1]) + title[1:] + "Param"
}

// markComputed makes every response attribute computed, as in a data source.
func markComputed(fields []common.FieldInfo) {
	for i := range fields {
		fields[i].IsDataSource = true
		fields[i].Required = false
		fields[i].ReadOnly = true
		if fields[i].ItemSchema != nil {
			fields[i].ItemSchema.IsDataSource = true
			fields[i].ItemSchema.Required = false
			fields[i].ItemSchema.ReadOnly = true
			markComputed(fields[i].ItemSchema.Properties)
		}
		markComputed(fields[i].Properties)
	}
}

// markSensitive flags the configured top-level response attributes as
// sensitive. A name that matches no attribute is an error rather than a secret
// silently left in the clear.
func markSensitive(fields []common.FieldInfo, names []string) error {
	for _, name := range names {
		found := false
		for i := range fields {
			if fields[i].Name == name && !fields[i].SchemaSkip {
				fields[i].Sensitive = true
				found = true
			}
		}
		if !found {
			return fmt.Errorf("sensitive field %q is not a response attribute", name)
		}
	}
	return nil
}

// GenerateImplementation renders the ephemeral resource.
func GenerateImplementation(cfg *config.Config, renderer common.Renderer, rd *common.ResourceData) error {
	return renderer.RenderTemplate(
		"ephemeral.go.tmpl",
		[]string{"templates/shared/*.tmpl", "components/ephemeral/ephemeral.go.tmpl"},
		rd,
		filepath.Join(cfg.Generator.OutputDir, "services", rd.Service, rd.CleanName),
		"ephemeral.go",
	)
}

package common

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func queryParam(name string, schema *openapi3.Schema) *openapi3.ParameterRef {
	return &openapi3.ParameterRef{Value: &openapi3.Parameter{
		In: "query", Name: name, Schema: &openapi3.SchemaRef{Value: schema},
	}}
}

// A multiple-choice filter is an array of scalars; it is offered as a
// single-valued filter of the item type, with the item's enum.
func TestExtractFilterParams_ArrayOfScalars(t *testing.T) {
	op := &openapi3.Operation{Parameters: openapi3.Parameters{
		queryParam("state", &openapi3.Schema{
			Type: &openapi3.Types{"array"},
			Items: &openapi3.SchemaRef{Value: &openapi3.Schema{
				Type: &openapi3.Types{"string"},
				Enum: []interface{}{"OK", "Erred"},
			}},
		}),
		queryParam("resource_uuid", &openapi3.Schema{Type: &openapi3.Types{"string"}, Format: "uuid"}),
		// Arrays of objects and the ordering parameter stay out.
		queryParam("tags", &openapi3.Schema{
			Type:  &openapi3.Types{"array"},
			Items: &openapi3.SchemaRef{Value: &openapi3.Schema{Type: &openapi3.Types{"object"}}},
		}),
		queryParam("o", &openapi3.Schema{
			Type:  &openapi3.Types{"array"},
			Items: &openapi3.SchemaRef{Value: &openapi3.Schema{Type: &openapi3.Types{"string"}}},
		}),
	}}

	params := ExtractFilterParams(op, "")
	if len(params) != 2 {
		t.Fatalf("expected resource_uuid and state, got %+v", params)
	}
	if params[0].Name != "resource_uuid" || params[0].Type != "String" {
		t.Errorf("unexpected first filter %+v", params[0])
	}
	state := params[1]
	if state.Name != "state" || state.Type != "String" {
		t.Errorf("expected a String state filter, got %+v", state)
	}
	if len(state.Enum) != 2 || state.Enum[0] != "OK" || state.Enum[1] != "Erred" {
		t.Errorf("expected the item enum on the state filter, got %v", state.Enum)
	}
}

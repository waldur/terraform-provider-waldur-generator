package generator

import (
	"testing"

	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/common"
)

func renderGoTypeFunc(t *testing.T) func(common.FieldInfo, string, string, string) string {
	t.Helper()
	fn, ok := GetFuncMap()["renderGoType"].(func(common.FieldInfo, string, string, string) string)
	if !ok {
		t.Fatal("renderGoType has an unexpected signature")
	}
	return fn
}

// Maps whose values are objects or arrays decode into map[string]interface{}, which the
// Plugin Framework cannot reflect into the generated map(string) attribute. Response
// structs use JSONStringMap instead; request structs are left alone.
func TestRenderGoType_AnyMap(t *testing.T) {
	renderGoType := renderGoTypeFunc(t)

	anyMap := common.FieldInfo{
		Name:     "options",
		Type:     common.OpenAPITypeObject,
		GoType:   common.TFTypeMap,
		ItemType: common.OpenAPITypeObject,
	}
	common.CalculateSDKType(&anyMap)

	stringMap := common.FieldInfo{
		Name:     "labels",
		Type:     common.OpenAPITypeObject,
		GoType:   common.TFTypeMap,
		ItemType: common.OpenAPITypeString,
	}
	common.CalculateSDKType(&stringMap)

	tests := []struct {
		name     string
		field    common.FieldInfo
		pkgName  string
		suffix   string
		expected string
	}{
		{
			name:     "any map in a service response struct",
			field:    anyMap,
			pkgName:  "offering",
			suffix:   "Response",
			expected: "common.JSONStringMap",
		},
		{
			name:     "any map in a shared common struct is left alone",
			field:    anyMap,
			pkgName:  "common",
			suffix:   "",
			expected: "map[string]interface{}",
		},
		{
			name:     "any map in a request struct is left alone",
			field:    anyMap,
			pkgName:  "offering",
			suffix:   "Request",
			expected: "map[string]interface{}",
		},
		{
			name:     "string map is unaffected",
			field:    stringMap,
			pkgName:  "offering",
			suffix:   "Response",
			expected: "map[string]string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderGoType(tt.field, tt.pkgName, "MarketplaceOffering", tt.suffix)
			if got != tt.expected {
				t.Errorf("renderGoType() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// marketplace_order flattens `attributes` to map(string) for the Terraform
// schema and the create request, but the API leaves the field unconstrained and
// real orders carry numbers, booleans and lists. The response struct therefore
// has to decode leniently even though its ItemType now says "string".
func TestRenderGoType_LenientStringMap(t *testing.T) {
	renderGoType := renderGoTypeFunc(t)

	flattened := common.FieldInfo{
		Name:     "attributes",
		Type:     common.OpenAPITypeObject,
		GoType:   common.TFTypeMap,
		ItemType: common.OpenAPITypeString,
	}
	common.CalculateSDKType(&flattened)

	// Sanity: without the marker this is an ordinary string map.
	if common.IsAnyMap(flattened) {
		t.Fatal("a string map must not be treated as an any-map")
	}
	if got := renderGoType(flattened, "order", "MarketplaceOrder", "Response"); got != "map[string]string" {
		t.Fatalf("unmarked string map = %q, want map[string]string", got)
	}

	lenient := flattened
	lenient.LenientStringMap = true

	if got := renderGoType(lenient, "order", "MarketplaceOrder", "Response"); got != "common.JSONStringMap" {
		t.Errorf("response = %q, want common.JSONStringMap", got)
	}
	// The write path must keep taking a plain string map.
	if got := renderGoType(lenient, "order", "MarketplaceOrder", "Request"); got != "map[string]string" {
		t.Errorf("request = %q, want map[string]string", got)
	}
}

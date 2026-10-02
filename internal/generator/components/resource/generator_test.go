package resource

import (
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/common"
)

func TestCheckActionRequestKey(t *testing.T) {
	extendBody := &openapi3.Schema{
		Type:       &openapi3.Types{"object"},
		Properties: openapi3.Schemas{"disk_size": &openapi3.SchemaRef{Value: openapi3.NewIntegerSchema()}},
	}
	arrayBody := &openapi3.Schema{Type: &openapi3.Types{"array"}}

	tests := []struct {
		name    string
		action  common.UpdateAction
		body    *openapi3.Schema
		wantErr bool
	}{
		{"key matches", common.UpdateAction{Name: "extend", Param: "size", RequestParam: "disk_size"}, extendBody, false},
		{"key missing", common.UpdateAction{Name: "extend", Operation: "openstack_volumes_extend", Param: "size", RequestParam: "size"}, extendBody, true},
		{"bare array body", common.UpdateAction{Name: "set_rules", Param: "rules", RequestParam: "rules"}, arrayBody, false},
		{"no schema", common.UpdateAction{Name: "pull", Param: "x", RequestParam: "x"}, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkActionRequestKey("openstack_volume", tt.action, tt.body)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), "disk_size") {
				t.Errorf("error should name the request body's properties: %v", err)
			}
		})
	}
}

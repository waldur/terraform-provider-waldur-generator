package generator

import (
	"testing"

	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/common"
)

func TestManifestModeMatchesSchemaLifecycle(t *testing.T) {
	tests := []struct {
		name                         string
		field                        common.FieldInfo
		dataSource                   bool
		required, optional, computed bool
	}{
		{"read-only", common.FieldInfo{ReadOnly: true}, false, false, false, true},
		{"read-only wins over required", common.FieldInfo{ReadOnly: true, Required: true}, false, false, false, true},
		{"required", common.FieldInfo{Required: true}, false, true, false, false},
		{"optional", common.FieldInfo{}, false, false, true, false},
		{"optional with server default", common.FieldInfo{ServerComputed: true}, false, false, true, true},
		{"data source", common.FieldInfo{Required: true}, true, false, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, o, c := manifestMode(tt.field, tt.dataSource)
			if r != tt.required || o != tt.optional || c != tt.computed {
				t.Errorf("got required=%v optional=%v computed=%v, want %v %v %v", r, o, c, tt.required, tt.optional, tt.computed)
			}
		})
	}
}

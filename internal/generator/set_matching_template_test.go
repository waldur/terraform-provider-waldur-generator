package generator

import (
	"bytes"
	"strings"
	"testing"
	"text/template"

	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/common"
)

func TestAttrPlanModifiers_RendersSetElementMatching(t *testing.T) {
	tmpl, err := template.New("schema").Funcs(GetFuncMap()).ParseFS(templates, "templates/shared/schema.tmpl")
	if err != nil {
		t.Fatalf("parsing templates: %v", err)
	}
	field := common.FieldInfo{
		Name:         "security_groups",
		SetMatchKeys: []string{"ip_address", "url"},
		TypeMeta:     common.TypeMeta{PlanModImport: "setplanmodifier", PlanModType: "planmodifier.Set"},
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "attr_plan_modifiers", field); err != nil {
		t.Fatalf("executing: %v", err)
	}
	out := buf.String()
	want := `common.SetElementsFromState{Keys: []string{"ip_address", "url"}},`
	if !strings.Contains(out, want) || !strings.Contains(out, "PlanModifiers: []planmodifier.Set{") {
		t.Errorf("rendered modifiers missing %q:\n%s", want, out)
	}
}

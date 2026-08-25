package generator

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"text/template"

	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/common"
)

// renderDefine renders a single named template from the embedded FS, using the
// same file set the corresponding plugin builder passes to RenderTemplate.
func renderDefine(t *testing.T, define string, files []string, data *common.ResourceData) string {
	t.Helper()
	tmpl, err := template.New("resource.go.tmpl").Funcs(GetFuncMap()).ParseFS(templates, files...)
	if err != nil {
		t.Fatalf("parsing templates: %v", err)
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, define, data); err != nil {
		t.Fatalf("executing %q: %v", define, err)
	}
	return buf.String()
}

func standardFiles(extra string) []string {
	return []string{"templates/shared/*.tmpl", "components/resource/resource.go.tmpl", extra}
}

// block is a brace-balanced region of rendered Go source.
type block struct {
	Body string
	End  int
}

// blocksAfter returns every brace-balanced block opened by one of the given
// headers at or after `from`, in source order.
func blocksAfter(src string, from int, headers ...string) []block {
	var out []block
	for i := from; i < len(src); i++ {
		matched := false
		for _, h := range headers {
			if strings.HasPrefix(src[i:], h) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		depth, start := 0, -1
		for j := i; j < len(src); j++ {
			switch src[j] {
			case '{':
				if depth == 0 {
					start = j
				}
				depth++
			case '}':
				depth--
				if depth == 0 && start >= 0 {
					out = append(out, block{Body: src[start : j+1], End: j + 1})
					i = j
					goto next
				}
			}
		}
	next:
	}
	return out
}

// remoteObjectExistsFrom returns the offset at which the remote object is known
// to exist: past the create call and past the create call's own error check.
// Everything after that point runs with an object already created remotely.
func remoteObjectExistsFrom(t *testing.T, body, createCall string) int {
	t.Helper()
	created := strings.Index(body, createCall)
	if created < 0 {
		t.Fatalf("create call %q not found in rendered body:\n%s", createCall, body)
	}
	own := blocksAfter(body, created+len(createCall), "if err != nil {")
	if len(own) == 0 {
		t.Fatalf("create call %q has no error check", createCall)
	}
	return own[0].End
}

// A resource whose remote object was created but whose completion wait or
// follow-up read failed must still be written to state. Returning an error
// diagnostic without State.Set makes the framework drop the resource, orphaning
// the created object: it exists remotely, Terraform stops tracking it, and the
// next apply creates a duplicate. For marketplace orders that is a duplicate
// billable order per retry.
func TestCreate_PersistsStateOnPostCreateFailure(t *testing.T) {
	tests := []struct {
		name       string
		data       *common.ResourceData
		files      []string
		createCall string
	}{
		{
			name: "standard resource",
			data: &common.ResourceData{
				Name:     "openstack_network",
				APIPaths: map[string]string{"Create": "/api/openstack-networks/"},
			},
			files:      standardFiles("plugins/standard/resource.tmpl"),
			createCall: "r.client.Create(ctx",
		},
		{
			// marketplace_order takes a dedicated branch in the standard plugin
			// that waits on the order rather than on the resource.
			name: "marketplace_order",
			data: &common.ResourceData{
				Name:     "marketplace_order",
				APIPaths: map[string]string{"Create": "/api/marketplace-orders/"},
			},
			files:      standardFiles("plugins/standard/resource.tmpl"),
			createCall: "r.client.Create(ctx",
		},
		{
			name: "order plugin",
			data: &common.ResourceData{
				Name:     "openstack_tenant",
				Plugin:   "order",
				IsOrder:  true,
				APIPaths: map[string]string{"Create": "/api/openstack-tenants/"},
			},
			files:      standardFiles("plugins/order/resource.tmpl"),
			createCall: "r.client.CreateOrder(ctx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := renderDefine(t, "resource_create", tt.files, tt.data)
			after := remoteObjectExistsFrom(t, body, tt.createCall)

			blocks := blocksAfter(body, after, "if err != nil {", "} else {")
			checked := 0
			for _, b := range blocks {
				if !strings.Contains(b.Body, "resp.Diagnostics.AddError(") || !strings.Contains(b.Body, "return") {
					continue
				}
				checked++
				if !strings.Contains(b.Body, "resp.State.Set(ctx, &data)") {
					t.Errorf("error path after create returns without persisting state:\n%s", b.Body)
				}
			}
			if checked == 0 {
				t.Fatal("no post-create error paths found; the test is not exercising anything")
			}
			t.Logf("verified %d post-create error paths", checked)
		})
	}
}

// The pre-create failure path must NOT set state: nothing exists remotely yet,
// so writing state there would invent a resource.
func TestCreate_DoesNotPersistStateBeforeRemoteCreate(t *testing.T) {
	data := &common.ResourceData{
		Name:     "openstack_network",
		APIPaths: map[string]string{"Create": "/api/openstack-networks/"},
	}
	body := renderDefine(t, "resource_create", standardFiles("plugins/standard/resource.tmpl"), data)

	created := strings.Index(body, "r.client.Create(ctx")
	if created < 0 {
		t.Fatal("create call not found")
	}
	if strings.Contains(body[:created], "resp.State.Set(ctx, &data)") {
		t.Error("state is written before the remote object is created")
	}

	// The create call's own error check runs when the remote create failed, so
	// there is nothing to track; writing state there would invent a resource.
	own := blocksAfter(body, created+len("r.client.Create(ctx"), "if err != nil {")
	if len(own) == 0 {
		t.Fatal("create call has no error check")
	}
	if strings.Contains(own[0].Body, "resp.State.Set(ctx, &data)") {
		t.Errorf("the failed-create path writes state:\n%s", own[0].Body)
	}
}

// A zero-valued timeouts.Value carries no attribute types, so it converts to
// tftypes.Object[] while the schema declares an object with create/update/delete.
// Import builds a fresh model and writes it straight to state, so without an
// explicit initialisation it fails the framework type check with a Value
// Conversion Error.
func TestImport_InitialisesTimeouts(t *testing.T) {
	for _, tt := range []struct {
		name  string
		data  *common.ResourceData
		files []string
	}{
		{
			name: "standard",
			data: &common.ResourceData{
				Name:     "openstack_network",
				APIPaths: map[string]string{"Create": "/api/openstack-networks/"},
			},
			files: standardFiles("plugins/standard/resource.tmpl"),
		},
		{
			name: "order plugin",
			data: &common.ResourceData{
				Name:     "openstack_tenant",
				Plugin:   "order",
				IsOrder:  true,
				APIPaths: map[string]string{"Create": "/api/openstack-tenants/"},
			},
			files: standardFiles("plugins/order/resource.tmpl"),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body := renderDefine(t, "resource_import", tt.files, tt.data)

			set := strings.Index(body, "resp.State.Set(ctx, &data)")
			if set < 0 {
				t.Fatalf("import does not write state:\n%s", body)
			}
			init := strings.Index(body, "data.Timeouts = timeouts.Value{")
			if init < 0 {
				t.Fatalf("import does not initialise Timeouts:\n%s", body)
			}
			if init > set {
				t.Error("Timeouts is initialised after the state write, so the write still fails")
			}
		})
	}
}

// The null object's attribute types have to match the schema's timeouts block.
// If the block ever gains or drops an option, a stale null_timeouts produces the
// same Value Conversion Error it was added to prevent.
func TestNullTimeouts_MatchesSchemaBlock(t *testing.T) {
	data := &common.ResourceData{
		Name:     "openstack_network",
		APIPaths: map[string]string{"Create": "/api/openstack-networks/"},
	}
	rendered := renderDefine(t, "null_timeouts", standardFiles("plugins/standard/resource.tmpl"), data)

	keys := map[string]bool{}
	for _, m := range regexp.MustCompile(`"(create|update|delete)":\s*types\.StringType`).FindAllStringSubmatch(rendered, -1) {
		keys[m[1]] = true
	}

	schemaTmpl, err := templates.ReadFile("components/resource/resource.go.tmpl")
	if err != nil {
		t.Fatalf("reading resource template: %v", err)
	}
	block := regexp.MustCompile(`timeouts\.Block\(ctx, timeouts\.Opts\{([^}]*)\}`).FindSubmatch(schemaTmpl)
	if block == nil {
		t.Fatal("timeouts.Block not found in the resource schema template")
	}
	opts := map[string]bool{}
	for _, m := range regexp.MustCompile(`(Create|Update|Delete):\s*true`).FindAllSubmatch(block[1], -1) {
		opts[strings.ToLower(string(m[1]))] = true
	}

	if len(opts) == 0 {
		t.Fatal("no enabled timeouts options parsed; the test is not exercising anything")
	}
	for opt := range opts {
		if !keys[opt] {
			t.Errorf("schema enables the %q timeout but null_timeouts omits it", opt)
		}
	}
	for key := range keys {
		if !opts[key] {
			t.Errorf("null_timeouts declares %q but the schema block does not enable it", key)
		}
	}
}

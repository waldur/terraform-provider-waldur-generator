package generator

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/waldur/terraform-provider-waldur-generator/internal/config"
	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/common"
	ephgen "github.com/waldur/terraform-provider-waldur-generator/internal/generator/components/ephemeral"
	"github.com/waldur/terraform-provider-waldur-generator/internal/openapi"
)

const revealSchema = `openapi: 3.0.3
info:
  title: test
  version: "1"
paths:
  /api/keys/{uuid}/reveal/:
    post:
      operationId: keys_reveal
      parameters:
      - in: path
        name: uuid
        required: true
        schema:
          type: string
      responses:
        '200':
          description: ''
          content:
            application/json:
              schema:
                type: object
                properties:
                  uuid:
                    type: string
                  client_id:
                    type: string
                  api_key:
                    type: string
                    readOnly: true
`

// renderFile executes a whole template file, as RenderTemplate does, without
// writing it out.
func renderFile(t *testing.T, name string, files []string, data interface{}) string {
	t.Helper()
	tmpl, err := template.New(name).Funcs(GetFuncMap()).ParseFS(templates, files...)
	if err != nil {
		t.Fatalf("parsing %s: %v", name, err)
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		t.Fatalf("executing %s: %v", name, err)
	}
	return buf.String()
}

func revealEntity(t *testing.T) *common.ResourceData {
	t.Helper()
	path := filepath.Join(t.TempDir(), "schema.yaml")
	if err := os.WriteFile(path, []byte(revealSchema), 0o644); err != nil {
		t.Fatal(err)
	}
	parser, err := openapi.NewParser(path)
	if err != nil {
		t.Fatal(err)
	}
	rd, err := ephgen.PrepareData(parser, &config.EphemeralResource{
		Name:            "marketplace_resource_api_key",
		OperationID:     "keys_reveal",
		PathParams:      map[string]string{"uuid": "id"},
		SensitiveFields: []string{"api_key"},
	}, common.SchemaConfig{})
	if err != nil {
		t.Fatalf("PrepareData: %v", err)
	}
	return rd
}

// assertContains checks for each snippet ignoring whitespace: templates render
// unformatted code, and goimports formats it only once it is written out.
func assertContains(t *testing.T, src string, wants ...string) {
	t.Helper()
	compact := strings.Join(strings.Fields(src), "")
	for _, want := range wants {
		if !strings.Contains(compact, strings.Join(strings.Fields(want), "")) {
			t.Errorf("rendered code lacks %q", want)
		}
	}
}

func TestEphemeralTemplate(t *testing.T) {
	src := renderFile(t, "ephemeral.go.tmpl",
		[]string{"templates/shared/*.tmpl", "components/ephemeral/ephemeral.go.tmpl"}, revealEntity(t))

	assertContains(t, src,
		"package resource_api_key_ephemeral",
		"var _ ephemeral.EphemeralResource = &MarketplaceResourceApiKeyEphemeralResource{}",
		`resp.TypeName = req.ProviderTypeName + "_marketplace_resource_api_key"`,
		// The path parameter is a required argument; the secret is computed and sensitive.
		`"id": schema.StringAttribute{ Required: true,`,
		`"api_key": schema.StringAttribute{ Computed: true, MarkdownDescription: "Api Key", Sensitive: true,`,
		"apiResp, err := e.client.Open(ctx, data.UUID.ValueString())",
		"data.UUID = configured.UUID",
		"resp.Result.Set(ctx, &data)",
	)
	if strings.Contains(src, "resp.State") {
		t.Errorf("an ephemeral resource must not touch state")
	}
	if strings.Count(src, `"id":`) != 1 {
		t.Errorf("id must be declared once, as the bound argument")
	}
}

func TestEphemeralClientOpen(t *testing.T) {
	rd := revealEntity(t)
	src := renderFile(t, "sdk_client.go.tmpl",
		[]string{"templates/shared/*.tmpl", "templates/sdk_client.go.tmpl"},
		map[string]interface{}{"Resources": []common.ResourceData{*rd}, "Package": rd.CleanName})

	assertContains(t, src,
		"func (c *MarketplaceResourceApiKeyClient) Open(ctx context.Context, uuidParam string) (*MarketplaceResourceApiKeyResponse, error)",
		`path := "/api/keys/{uuid}/reveal/"`,
		`path = strings.Replace(path, "{uuid}", uuidParam, 1)`,
		"err := c.Client.Post(ctx, path, nil, &apiResp)",
	)
	// The entity has no list or retrieve operation of its own.
	for _, absent := range []string{") Get(ctx", ") List(ctx"} {
		if strings.Contains(src, absent) {
			t.Errorf("ephemeral client must not render %q", absent)
		}
	}
}

func TestServiceRegistrationOfEphemeralAndPluralDataSource(t *testing.T) {
	ephemeral := revealEntity(t)
	dataSource := &common.ResourceData{
		Name:                 "marketplace_resource_api_key",
		CleanName:            "resource_api_key",
		IsDatasourceOnly:     true,
		HasDataSource:        true,
		PluralDataSourceName: "marketplace_resource_api_keys",
	}
	src := renderFile(t, "service_register.go.tmpl",
		[]string{"templates/service_register.go.tmpl"},
		map[string]interface{}{
			"Service":      "marketplace",
			"Resources":    []*common.ResourceData{dataSource, ephemeral},
			"ProviderName": "waldur",
		})

	assertContains(t, src,
		"pkg_resource_api_key.NewMarketplaceResourceApiKeyDataSource,",
		"pkg_resource_api_key.NewMarketplaceResourceApiKeyListDataSource,",
		"func GetEphemeralResources() []func() ephemeral.EphemeralResource",
		"pkg_resource_api_key_ephemeral.NewMarketplaceResourceApiKeyEphemeralResource,",
	)
	for _, absent := range []string{
		"pkg_resource_api_key_ephemeral.NewMarketplaceResourceApiKeyResource",
		"pkg_resource_api_key_ephemeral.NewMarketplaceResourceApiKeyList,",
		"pkg_resource_api_key_ephemeral.NewMarketplaceResourceApiKeyDataSource",
	} {
		if strings.Contains(src, absent) {
			t.Errorf("an ephemeral resource must not register %q", absent)
		}
	}
}

func TestProviderServesEphemeralResources(t *testing.T) {
	src := renderFile(t, "provider.go.tmpl", []string{"templates/provider.go.tmpl"},
		map[string]interface{}{"ProviderName": "waldur", "Services": []string{"marketplace"}})
	assertContains(t, src,
		"var _ provider.ProviderWithEphemeralResources = &waldurProvider{}",
		"resp.EphemeralResourceData = apiClient",
		"er = append(er, marketplace.GetEphemeralResources()...)",
	)
}

func TestPluralDataSourceTemplate(t *testing.T) {
	data := map[string]interface{}{
		"Name":           "marketplace_resource_api_key",
		"PluralName":     "marketplace_resource_api_keys",
		"CleanName":      "resource_api_key",
		"FilterParams":   []common.FilterParam{{Name: "state", Type: "String"}},
		"ResponseFields": []common.FieldInfo{},
	}
	src := renderFile(t, "datasource_list.go.tmpl",
		[]string{"templates/shared/*.tmpl", "components/datasource/datasource_list.go.tmpl"}, data)
	assertContains(t, src,
		`resp.TypeName = req.ProviderTypeName + "_marketplace_resource_api_keys"`,
		`Items []MarketplaceResourceApiKeyModel `+"`tfsdk:\"items\"`",
		`"items": schema.ListNestedAttribute{ Computed: true,`,
		"results, err := d.client.ListAll(ctx, filters)",
		"resp.Diagnostics.Append(data.Items[i].CopyFrom(ctx, results[i])...)",
	)
}

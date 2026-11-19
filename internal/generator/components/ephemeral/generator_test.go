package ephemeral

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/waldur/terraform-provider-waldur-generator/internal/config"
	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/common"
	"github.com/waldur/terraform-provider-waldur-generator/internal/openapi"
)

// testSchema has a reveal-style operation (POST, path parameter, no body, a
// secret in the response), a GET with two path parameters, and a POST with a
// request body, which ephemeral resources do not support.
const testSchema = `openapi: 3.0.3
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
          format: uuid
      responses:
        '200':
          description: ''
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/RevealedKey'
  /api/resources/{resource_uuid}/keys/{key}/:
    get:
      operationId: resources_key_retrieve
      parameters:
      - in: path
        name: resource_uuid
        required: true
        schema:
          type: string
      - in: path
        name: key
        required: true
        schema:
          type: string
      responses:
        '200':
          description: ''
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/RevealedKey'
  /api/keys/{uuid}/rotate/:
    post:
      operationId: keys_rotate
      parameters:
      - in: path
        name: uuid
        required: true
        schema:
          type: string
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                reason:
                  type: string
      responses:
        '200':
          description: ''
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/RevealedKey'
components:
  schemas:
    RevealedKey:
      type: object
      properties:
        uuid:
          type: string
          format: uuid
          readOnly: true
        client_id:
          type: string
        api_key:
          type: string
          readOnly: true
        created:
          type: string
          format: date-time
          readOnly: true
`

func testParser(t *testing.T) *openapi.Parser {
	t.Helper()
	path := filepath.Join(t.TempDir(), "schema.yaml")
	if err := os.WriteFile(path, []byte(testSchema), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := openapi.NewParser(path)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func schemaCfg() common.SchemaConfig {
	return common.SchemaConfig{ExcludedFields: map[string]bool{"created": true}}
}

func field(t *testing.T, fields []common.FieldInfo, name string) common.FieldInfo {
	t.Helper()
	for _, f := range fields {
		if f.Name == name {
			return f
		}
	}
	t.Fatalf("no field %q", name)
	return common.FieldInfo{}
}

func TestPrepareData_Reveal(t *testing.T) {
	rd, err := PrepareData(testParser(t), &config.EphemeralResource{
		Name:            "marketplace_resource_api_key",
		OperationID:     "keys_reveal",
		PathParams:      map[string]string{"uuid": "id"},
		SensitiveFields: []string{"api_key"},
	}, schemaCfg())
	if err != nil {
		t.Fatalf("PrepareData: %v", err)
	}

	if !rd.IsEphemeral || rd.IsDatasourceOnly {
		t.Errorf("expected an ephemeral, non-data-source entity, got IsEphemeral=%v IsDatasourceOnly=%v", rd.IsEphemeral, rd.IsDatasourceOnly)
	}
	if rd.Service != "marketplace" || rd.CleanName != "resource_api_key_ephemeral" {
		t.Errorf("expected package marketplace/resource_api_key_ephemeral, got %s/%s", rd.Service, rd.CleanName)
	}
	if rd.EphemeralTypeName != "marketplace_resource_api_key" {
		t.Errorf("unexpected type name %q", rd.EphemeralTypeName)
	}
	if rd.EphemeralMethod != "POST" || rd.APIPaths["Ephemeral"] != "/api/keys/{uuid}/reveal/" {
		t.Errorf("unexpected operation %s %s", rd.EphemeralMethod, rd.APIPaths["Ephemeral"])
	}

	if len(rd.EphemeralPathParams) != 1 {
		t.Fatalf("expected one path parameter, got %+v", rd.EphemeralPathParams)
	}
	p := rd.EphemeralPathParams[0]
	if p.Param != "uuid" || p.Attribute != "id" || p.FieldName != "UUID" || !p.InModel || p.ArgName != "uuidParam" {
		t.Errorf("unexpected binding %+v", p)
	}

	apiKey := field(t, rd.ResponseFields, "api_key")
	if !apiKey.Sensitive || !apiKey.ReadOnly || !apiKey.IsDataSource {
		t.Errorf("api_key should be sensitive and computed: %+v", apiKey)
	}
	if clientID := field(t, rd.ResponseFields, "client_id"); clientID.Sensitive || !clientID.ReadOnly {
		t.Errorf("client_id should be computed and not sensitive: %+v", clientID)
	}
	for _, f := range rd.ResponseFields {
		if f.Name == "uuid" {
			t.Errorf("uuid is the id attribute and must not be a response attribute")
		}
		if f.Name == "created" {
			t.Errorf("globally excluded fields must be left out")
		}
	}
}

func TestPrepareData_TwoPathParamsSortedAndNamed(t *testing.T) {
	rd, err := PrepareData(testParser(t), &config.EphemeralResource{
		Name:        "marketplace_resource_key",
		OperationID: "resources_key_retrieve",
		PathParams:  map[string]string{"resource_uuid": "resource", "key": "key_name"},
	}, schemaCfg())
	if err != nil {
		t.Fatalf("PrepareData: %v", err)
	}
	if rd.EphemeralMethod != "GET" {
		t.Errorf("expected GET, got %s", rd.EphemeralMethod)
	}
	var got []string
	for _, p := range rd.EphemeralPathParams {
		got = append(got, p.Param+"="+p.Attribute+"/"+p.FieldName+"/"+p.ArgName)
		if p.InModel {
			t.Errorf("%s is not the id and must get a field of its own", p.Attribute)
		}
	}
	want := "key=key_name/KeyName/keyParam,resource_uuid=resource/Resource/resourceUuidParam"
	if strings.Join(got, ",") != want {
		t.Errorf("expected %s, got %s", want, strings.Join(got, ","))
	}
}

func TestPrepareData_Errors(t *testing.T) {
	cases := []struct {
		name string
		eph  config.EphemeralResource
		want string
	}{
		{"unknown operation", config.EphemeralResource{Name: "x_y", OperationID: "nope"}, "operation not found"},
		{"unbound path parameter", config.EphemeralResource{Name: "x_y", OperationID: "keys_reveal"}, `path parameter "uuid" has no argument`},
		{"binding to no parameter", config.EphemeralResource{Name: "x_y", OperationID: "keys_reveal", PathParams: map[string]string{"uuid": "id", "other": "x"}}, `"other", which is not a path parameter`},
		{"argument shadows a response attribute", config.EphemeralResource{Name: "x_y", OperationID: "keys_reveal", PathParams: map[string]string{"uuid": "client_id"}}, "also a response attribute"},
		{"request body", config.EphemeralResource{Name: "x_y", OperationID: "keys_rotate", PathParams: map[string]string{"uuid": "id"}}, "takes a request body"},
		{"unknown sensitive field", config.EphemeralResource{Name: "x_y", OperationID: "keys_reveal", PathParams: map[string]string{"uuid": "id"}, SensitiveFields: []string{"secret"}}, `sensitive field "secret"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := PrepareData(testParser(t), &tc.eph, schemaCfg())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("expected an error containing %q, got %v", tc.want, err)
			}
		})
	}
}

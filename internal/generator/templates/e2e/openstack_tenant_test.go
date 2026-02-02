package e2e_test

import (
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/waldur/terraform-provider-waldur/internal/provider"
	"github.com/waldur/terraform-provider-waldur/internal/testhelpers"
)

func TestOpenstackTenant_Create(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Skipping acceptance test")
	}

	rec, cleanup := testhelpers.SetupVCR(t, "openstack_tenant_create")
	defer cleanup()

	httpClient := &http.Client{Transport: rec}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"waldur": providerserver.NewProtocol6WithError(
				provider.NewWithHTTPClient("test", httpClient)(),
			),
		},
		Steps: []resource.TestStep{
			{
				Config: testAccOpenstackTenantConfig_basic(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("waldur_openstack_tenant.test", "name", "tf-test-tenant-v2"),
					resource.TestCheckResourceAttrSet("waldur_openstack_tenant.test", "id"),
					resource.TestCheckResourceAttrSet("waldur_openstack_tenant.test", "project"),
					resource.TestCheckResourceAttrSet("waldur_openstack_tenant.test", "offering"),
				),
			},
		},
	})
}

func getProviderConfigTenant() string {
	endpoint := os.Getenv("WALDUR_API_URL")
	if endpoint == "" {
		endpoint = "https://api.waldur.example.com"
	}
	token := os.Getenv("WALDUR_ACCESS_TOKEN")
	if token == "" {
		token = "test-token-sanitized"
	}
	return fmt.Sprintf(`provider "waldur" {
  endpoint = %q
  token    = %q
}
`, endpoint, token)
}

func testAccOpenstackTenantConfig_basic() string {
	return getProviderConfigTenant() + `
data "waldur_structure_project" "test" {
  filters = {
    name_exact = "Naked azanide as an aminating agent and a superbase"
  }
}

data "waldur_marketplace_offering" "test" {
  filters = {
    name = "MockOpenStack4"
    project_uuid = data.waldur_structure_project.test.id
  }
}

resource "waldur_openstack_tenant" "test" {
  name     = "tf-test-tenant-v2"
  project  = data.waldur_structure_project.test.url
  offering = data.waldur_marketplace_offering.test.url
  plan     = data.waldur_marketplace_offering.test.plans[0].url
  limits = {
    cores   = 10
    ram     = 10240
    storage = 100
  }
  skip_connection_extnet          = true
  skip_creation_of_default_router = true
}
`
}

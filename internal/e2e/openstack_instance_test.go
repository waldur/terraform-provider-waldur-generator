package e2e_test

import (
	"net/http"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/waldur/terraform-waldur-provider/internal/provider"
	"github.com/waldur/terraform-waldur-provider/internal/testhelpers"
)

func TestOpenstackInstance_CRUD(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Skipping acceptance test")
	}

	rec, cleanup := testhelpers.SetupVCR(t, "openstack_instance_crud")
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
				Config: testAccOpenstackInstanceConfig_basic(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("waldur_openstack_instance.test", "name", "test-instance"),
					resource.TestCheckResourceAttrSet("waldur_openstack_instance.test", "id"),
				),
			},

			{
				Config: testAccOpenstackInstanceConfig_updated(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("waldur_openstack_instance.test", "name", "test-instance-updated"),
				),
			},
		},
	})
}

func testAccOpenstackInstanceConfig_basic() string {
	return `provider "waldur" {
  endpoint = "https://api.waldur.example.com"
  token    = "test-token-sanitized"
}

resource "waldur_openstack_instance" "test" {
  name    = "test-instance"
  flavor  = "m1.small"
  image   = "ubuntu-20.04"
  project = "abc123-project-uuid"
  offering = "offering-uuid"
  system_volume_size = 1024
  ports = []
}
`
}

func testAccOpenstackInstanceConfig_updated() string {
	return `provider "waldur" {
  endpoint = "https://api.waldur.example.com"
  token    = "test-token-sanitized"
}

resource "waldur_openstack_instance" "test" {
  name    = "test-instance-updated"
  flavor  = "m1.small"
  image   = "ubuntu-20.04"
  project = "abc123-project-uuid"
  offering = "offering-uuid"
  system_volume_size = 1024
  ports = []
}
`
}

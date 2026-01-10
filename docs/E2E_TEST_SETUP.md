# E2E VCR Testing Guide for Generated Providers

This guide explains how to set up and run End-to-End (E2E) tests for your generated Terraform provider using VCR (Go-VCR). VCR records HTTP interactions to cassette files, allowing you to run deterministic tests without a live API connection.

## 1. Setup & Prerequisites

Before writing tests, set up the generated provider environment:

### A. Copy Test Helpers

The generator includes VCR test helpers that need to be copied to your generated provider:

```bash
# From the generator project root:
cp -r internal/testhelpers testdata/output/internal/
```

### B. Create Fixtures Directory

Create the directory where VCR cassettes will be stored:

```bash
mkdir -p testdata/output/testdata/fixtures
```

### C. Install Dependencies

Navigate to your generated provider directory and install necessary testing modules:

```bash
cd testdata/output
go get github.com/hashicorp/terraform-plugin-testing
go get github.com/hashicorp/terraform-plugin-framework
go get github.com/hashicorp/terraform-plugin-go
go mod tidy
```

## 2. Writing an E2E Test

Create a new test file, e.g., `e2e_test/openstack_instance_test.go`.

**Key Components:**

* **VCR Setup**: Use `testhelpers.SetupVCR(t, "cassette_name")`.
* **HTTP Client Injection**: Create an `http.Client` using the VCR recorder transport.
* **Provider Factory**: Use `provider.NewWithHTTPClient` (generated automatically) to inject the VCR client.

### Example Test Code

```go
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
 // 1. Skip if not running acceptance tests
 if os.Getenv("TF_ACC") == "" {
  t.Skip("Skipping acceptance test - set TF_ACC=1 to run")
 }

 // 2. Setup VCR Recorder
 //    This will look for fixtures in testdata/fixtures/openstack_instance_crud.yaml
 rec, cleanup := testhelpers.SetupVCR(t, "openstack_instance_crud")
 defer cleanup()

 // 3. Create HTTP Client with VCR Transport
 httpClient := &http.Client{Transport: rec}

 // 4. Run Terraform Test
 resource.Test(t, resource.TestCase{
  ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
   "waldur": providerserver.NewProtocol6WithError(
    // Inject the VCR-enabled client into the provider
    provider.NewWithHTTPClient("test", httpClient)(),
   ),
  },
  Steps: []resource.TestStep{
   // Step 1: CREATE
   {
    Config: testAccConfig_Basic(),
    Check: resource.ComposeTestCheckFunc(
     resource.TestCheckResourceAttr("waldur_openstack_instance.test", "name", "test-instance"),
     resource.TestCheckResourceAttrSet("waldur_openstack_instance.test", "id"),
    ),
   },
   // Step 2: UPDATE
   {
    Config: testAccConfig_Updated(),
    Check: resource.ComposeTestCheckFunc(
     resource.TestCheckResourceAttr("waldur_openstack_instance.test", "name", "test-instance-updated"),
    ),
   },
   // DELETE is handled automatically
  },
 })
}

// Helper functions for Terraform configuration
func testAccConfig_Basic() string {
 return `
provider "waldur" {
  endpoint = "https://api.waldur.example.com" // Implementation ignored by VCR replay
  token    = "test-token"                     // Sanitized token
}

resource "waldur_openstack_instance" "test" {
  name    = "test-instance"
  # ... other required fields ...
}
`
}

func testAccConfig_Updated() string {
 return `
provider "waldur" {
  endpoint = "https://api.waldur.example.com"
  token    = "test-token"
}

resource "waldur_openstack_instance" "test" {
  name    = "test-instance-updated"
  # ... other required fields ...
}
`
}
```

## 3. Recording Cassettes

To record a new test scenario, you need access to a live API.

1. **Configure Environment**:

    ```bash
    export WALDUR_API_URL="https://actual-api.waldur.com/api/"
    export WALDUR_API_TOKEN="your-real-token"
    export VCR_RECORD=true
    export TF_ACC=1
    ```

2. **Run the Test**:

    ```bash
    go test ./e2e_test -v -run TestOpenstackInstance_CRUD
    ```

    This will execute the real API calls and check for success.

3. **Sanitize the Cassette**:
    The recording will be saved to `testdata/fixtures/openstack_instance_crud.yaml`.
    **IMPORTANT**: Open this file and replace any sensitive tokens or real backend IDs with placeholders (e.g., replace your real token with `test-token-sanitized`).

4. **Commit**: Add the sanitized YAML file to git.

## 4. Running Tests (Replay Mode)

Once the cassette is recorded and sanitized, anyone can run the test without API access.

```bash
# Just enable acceptance tests
export TF_ACC=1

# Run tests
go test ./e2e_test -v
```

The test will use the recorded responses from the YAML file. It is fast, deterministic, and requires no network.

## 5. CI Integration

To run these tests in GitHub Actions or other CI:

```yaml
- name: Run E2E Tests
  working-directory: testdata/output
  run: go test ./e2e_test -v
  env:
    TF_ACC: "1"
    VCR_RECORD: "false" # Explicitly ensure replay mode
```

## 6. Troubleshooting & Best Practices

* **"Cassette not found"**: You are in replay mode but the YAML file doesn't exist. You must record it first.
* **"Interaction not found"**: The test made a request that isn't in the recording. Did you change the test logic? You may need to re-record.
* **One Cassette Per Test**: Use a unique name in `SetupVCR` for each test function (e.g., "instance_crud", "instance_error_case") to avoid conflicts.
* **Sanitization**: Never commit real credentials. Go-VCR has some built-in sanitization, but manual verification is recommended.

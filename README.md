# Terraform Waldur Provider Generator

A Go-based code generator that creates a Terraform provider plugin for managing hybrid cloud computing infrastructure using the [Waldur](https://waldur.com/) REST API.

## Overview

This generator reads a Waldur OpenAPI schema and a YAML configuration file to automatically generate a complete Terraform provider using the modern [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework). The generated provider can be tested, built, and published to the Terraform Registry.

## Features

- ✅ **Convention-based configuration**: Minimal YAML config using `base_operation_id` for automatic operation inference
- ✅ **Modern Terraform Plugin Framework**: Uses the latest Plugin Framework (protocol 6.0)
- ✅ **OpenAPI schema parsing**: Automatically infers schemas and operations from OpenAPI definitions
- ✅ **Multi-platform builds**: Generates providers for Linux, macOS, and Windows
- ✅ **Registry-ready**: Includes GoReleaser config and GitHub Actions for automated publishing
- ✅ **Modular resource naming**: Supports module-prefixed resources (e.g., `structure_project`, `openstack_instance`)

## Installation

### Prerequisites

- Go 1.24 or later
- Access to Waldur OpenAPI schema file

### Install from source

```bash
git clone https://github.com/waldur/terraform-waldur-provider-generator.git
cd terraform-waldur-provider-generator
go install
```

Or install directly:

```bash
go install github.com/waldur/terraform-waldur-provider-generator@latest
```

## Usage

### 1. Create Configuration File

Create a `config.yaml` file that defines the resources and data sources you want to generate:

```yaml
generator:
  openapi_schema: "path/to/waldur-openapi.yaml"
  output_dir: "./output/terraform-waldur-provider"
  provider_name: "waldur"

resources:
  - name: "structure_project"
    base_operation_id: "projects"
    
  - name: "openstack_instance"
    base_operation_id: "openstack_instances"
    
data_sources:
  - name: "structure_project"
    base_operation_id: "projects"
```

**Convention-based Operation Inference:**

For each `base_operation_id`, the generator automatically looks for these operations in the OpenAPI schema:

- `{base}_list` - List/read all resources (GET)
- `{base}_create` - Create resource (POST)
- `{base}_retrieve` - Read single resource (GET with ID)
- `{base}_partial_update` - Update resource (PATCH)
- `{base}_destroy` - Delete resource (DELETE)

### 2. Run the Generator

```bash
./terraform-waldur-provider-generator -config config.yaml
```

Or if running from source:

```bash
go run main.go -config config.yaml
```

### 3. Build the Generated Provider

```bash
cd ./output/terraform-waldur-provider
go mod tidy
go build
```

### 4. Test the Provider

```bash
# Run unit tests
cd ./output/terraform-waldur-provider
go test ./... -v

# Run acceptance tests (requires TF_ACC=1)
TF_ACC=1 go test ./... -v
```

### 5. E2E Testing with go-VCR

The generator includes support for End-to-End (E2E) testing using [go-VCR](https://github.com/dnaeon/go-vcr), which records and replays HTTP interactions with the Waldur API.

This allows for:

- ✅ **Testing without live API dependencies** (Replay mode)
- ✅ **Deterministic and fast CI execution**
- ✅ **Verification of full CRUD lifecycles**

**Quick Start in Replay Mode:**

```bash
cd output/terraform-waldur-provider
TF_ACC=1 go test ./internal/e2e -v
```

For detailed instructions on setup, recording new cassettes, and writing tests, please refer to the **[E2E VCR Testing Guide](docs/E2E_TEST_SETUP.md)**.

### 5. Test Locally Without Publishing

You can test the built provider locally without publishing it to the Terraform Registry using Terraform's [development overrides](https://developer.hashicorp.com/terraform/cli/config/config-file#development-overrides-for-provider-developers).

#### Step 1: Build and install the provider binary

```bash
cd ./output/terraform-waldur-provider
go build -o terraform-provider-waldur

# Create the local plugin directory
mkdir -p ~/.terraform.d/plugins/registry.terraform.io/waldur/waldur/1.0.0/linux_amd64

# Copy the provider binary with the correct naming convention
cp terraform-provider-waldur ~/.terraform.d/plugins/registry.terraform.io/waldur/waldur/1.0.0/linux_amd64/terraform-provider-waldur_v1.0.0
```

**Note:** Adjust the platform directory (`linux_amd64`) based on your OS and architecture:

- Linux AMD64: `linux_amd64`
- macOS AMD64: `darwin_amd64`
- macOS ARM64 (M1/M2): `darwin_arm64`
- Windows AMD64: `windows_amd64`

#### Step 2: Create or edit `~/.terraformrc`

Create a Terraform CLI configuration file at `~/.terraformrc` (or `%APPDATA%/terraform.rc` on Windows) with the following content:

```hcl
provider_installation {
  dev_overrides {
    "registry.terraform.io/waldur/waldur" = "/home/your-username/.terraform.d/plugins/registry.terraform.io/waldur/waldur/1.0.0/linux_amd64"
  }

  # For all other providers, install them directly as normal.
  direct {}
}
```

**Important:**

- Replace `/home/your-username/` with your actual home directory path
- Adjust the platform directory to match your OS (see Step 1)

#### Step 3: Use the provider in your Terraform configuration

Create a test Terraform configuration (e.g., `test.tf`):

```hcl
terraform {
  required_providers {
    waldur = {
      source = "registry.terraform.io/waldur/waldur"
    }
  }
}

provider "waldur" {
  endpoint     = "https://your-waldur-instance.com/api/"
  token = "your-api-token"
}

# Test with a data source
data "waldur_structure_project" "test" {
  name = "My Project"
}

output "project_uuid" {
  value = data.waldur_structure_project.test.id
}
```

#### Step 4: Run Terraform

**Important:** When using `dev_overrides`, **skip `terraform init`** and go directly to `terraform plan`:

```bash
terraform plan
terraform apply
```

When using `dev_overrides`, Terraform will display a warning:

```text
Warning: Provider development overrides are in effect
```

This is expected and confirms that Terraform is using your locally built provider instead of downloading from the registry.

#### Step 5: Clean up (optional)

When you're done testing, remove or comment out the `dev_overrides` section from `~/.terraformrc` to return to normal provider installation behavior.

## Generated Provider Structure

The generator creates a complete provider with the following structure:

```text
output/terraform-waldur-provider/
├── main.go                          # Provider entry point
├── go.mod                           # Go module
├── internal/
│   ├── provider/                   # Provider configuration
│   ├── resources/                  # Generated resources
│   ├── datasources/                # Generated data sources
│   └── client/                     # Waldur API client
├── .goreleaser.yml                 # Multi-platform build config
├── terraform-registry-manifest.json # Registry metadata
└── .github/workflows/
    └── release.yml                 # Automated release workflow
```

## Publishing to Terraform Registry

The generated provider includes all necessary files for publishing to the Terraform Registry:

### 1. Generate GPG Signing Key

```bash
gpg --full-generate-key
# Select: RSA and RSA
# Key size: 4096
# Follow prompts
```

### 2. Add GPG Key to Terraform Registry

1. Sign in to [Terraform Registry](https://registry.terraform.io/)
2. Go to User Settings → Signing Keys
3. Add your public GPG key:

   ```bash
   gpg --armor --export your.email@example.com
   ```

### 3. Configure GitHub Secrets

Add these secrets to your GitHub repository:

- `GPG_PRIVATE_KEY`: Your private GPG key (`gpg --armor --export-secret-keys your.email@example.com`)
- `PASSPHRASE`: Your GPG key passphrase

### 4. Create a Release

```bash
git tag v1.0.0
git push origin v1.0.0
```

The GitHub Actions workflow will automatically:

- Build binaries for all platforms
- Create checksums
- Sign the release with your GPG key
- Publish to GitHub Releases
- Make it available on Terraform Registry

## Configuration Reference

### Generator Section

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `openapi_schema` | string | Yes | Path to Waldur OpenAPI schema file |
| `output_dir` | string | No | Output directory (default: `./output/terraform-waldur-provider`) |
| `provider_name` | string | Yes | Provider name (e.g., `waldur`) |

### Resources and Data Sources

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Resource/data source name (with module prefix) |
| `base_operation_id` | string | Yes | Base operation ID for convention-based inference |

## Development

### Project Structure

```text
terraform-waldur-provider-generator/
├── main.go                      # CLI entry point
├── internal/
│   ├── config/                 # Configuration parsing
│   ├── openapi/                # OpenAPI schema parsing
│   └── generator/              # Code generation logic
│       └── templates/          # Template files
├── testdata/                   # Test data
├── examples/                   # Example configurations
└── config.yaml                 # Example config
```

### Running Tests

```bash
go test ./... -v
```

### Building

```bash
go build -o terraform-waldur-provider-generator
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Links

- [Waldur](https://waldur.com/)
- [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework)
- [Terraform Registry](https://registry.terraform.io/)
- [OpenAPI Specification](https://spec.openapis.org/oas/v3.1.0)

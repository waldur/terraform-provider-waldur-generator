# Configuration Guide: Waldur Terraform Provider Generator

The generator is controlled by a `config.yaml` file. This guide explains every available configuration option to help you tune the generated provider.

## Global Generator Settings

```yaml
generator:
  openapi_schema: "waldur_api.yaml"  # Path to the OpenAPI spec
  output_dir: "output"               # Where to generate code
  provider_name: "waldur"            # Name of the provider
  
  # Fields to exclude globally from all resources and data sources
  excluded_fields:
    - "created"
    - "modified"
    
  # Fields to force as types.Set instead of types.List (globally)
  set_fields:
    - "tags"
```

## Resource Configuration

Resources are defined in the `resources` list.

### 1. Basic Resource

```yaml
- name: "structure_project"
  base_operation_id: "projects"
```

Uses convention-based lookup for `projects_list`, `projects_create`, etc.

### 2. Custom Operations

If your resource doesn't follow the `{base}_{op}` naming convention:

```yaml
- name: "openstack_security_group"
  base_operation_id: "openstack_security_groups"
  create_operation:
    operation_id: "openstack_tenants_create_security_group"
    path_params:
      uuid: "tenant"  # Maps the resource ID or a field to a path param
```

### 3. Plugins

Plugins switch the internal logic of the resource.

* **`order`**: For Waldur Marketplace resources.

    ```yaml
    - name: "openstack_instance"
      plugin: order
      offering_type: OpenStack.Instance
    ```

* **`link`**: For relationship resources (join tables).

    ```yaml
    - name: "openstack_volume_attachment"
      link_op: "openstack_volumes_attach"
      unlink_op: "openstack_volumes_detach"
    ```

### 4. Specialized Update Actions

If a resource has "Action" endpoints (POST to a sub-resource) that should be mapped to Terraform fields:

```yaml
update_actions:
  update_limits:
    operation: "marketplace_resources_update_limits"
    param: "limits"       # The field in Terraform
    compare_key: "limits" # Used to detect drift
```

### 5. Standalone Actions

Custom POST operations that don't map to fields (exposed as `terraform apply` triggers or just handled internally):

```yaml
actions:
  - "start"
  - "stop"
  - "pull"
```

### 6. Fine-grained Field Overrides

You can override specific attribute properties:

```yaml
set_fields:
  rules.ethertype:
    computed: true
    unknown_if_null: true # Forces (Unknown) if API returns null, preventing drift
```

### 7. Termination Attributes

For resources that require extra parameters during deletion:

```yaml
termination_attributes:
  - name: delete_volumes
    type: boolean
```

## Data Source Configuration

Data sources are simpler and usually only require the `base_operation_id`.

```yaml
data_sources:
  - name: "openstack_flavor"
    base_operation_id: "openstack_flavors"
```

## Tips for Best Results

1. **Iterative Generation**: Start with a minimal config, run the generator, check the `output/`, and then add overrides as needed.
2. **Use `excluded_fields`**: Always exclude metadata fields like `created`/`modified` to avoid noisy Terraform diffs.
3. **Check `unknown_if_null`**: If Terraform keeps showing a diff for a field even when it hasn't changed, it might be because the API returns `null` but Terraform expects an empty value. Use `unknown_if_null` to resolve this.

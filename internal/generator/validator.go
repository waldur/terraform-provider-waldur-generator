package generator

import (
	"fmt"
)

// validateOperations checks that all referenced operations exist in the OpenAPI schema
func (g *Generator) validateOperations() error {
	for _, resource := range g.config.Resources {
		ops := resource.OperationIDs()

		// Build a set of operations to skip
		skipOps := make(map[string]bool)
		for _, op := range resource.SkipOperations {
			skipOps[op] = true
		}

		// For order resources, create and destroy operations don't exist
		// (they use marketplace-orders API instead)
		operationsToCheck := map[string]string{
			"list":           ops.List,
			"retrieve":       ops.Retrieve,
			"partial_update": ops.PartialUpdate,
		}
		if resource.LinkOp != "" {
			// Link Plugin validation
			operationsToCheck["link"] = resource.LinkOp
			operationsToCheck["unlink"] = resource.UnlinkOp
			if resource.Source != nil && resource.Source.RetrieveOp != "" {
				operationsToCheck["source_retrieve"] = resource.Source.RetrieveOp
			}
			// Don't validate standard CRUD for link resources
			delete(operationsToCheck, "list")
			delete(operationsToCheck, "retrieve")
			delete(operationsToCheck, "partial_update")
		} else if resource.Plugin != "order" {
			// Use custom create operation if specified
			if resource.CreateOperation != nil && resource.CreateOperation.OperationID != "" {
				operationsToCheck["create"] = resource.CreateOperation.OperationID
			} else {
				operationsToCheck["create"] = ops.Create
			}
			operationsToCheck["destroy"] = ops.Destroy
		}

		for opName, opID := range operationsToCheck {
			// Skip if this operation is in the skip list
			if skipOps[opName] {
				continue
			}
			if err := g.parser.ValidateOperationExists(opID); err != nil {
				return fmt.Errorf("resource %s: %w", resource.Name, err)
			}
		}
	}

	for _, dataSource := range g.config.DataSources {
		ops := dataSource.OperationIDs()
		if err := g.parser.ValidateOperationExists(ops.List); err != nil {
			return fmt.Errorf("data source %s: %w", dataSource.Name, err)
		}
	}

	return nil
}

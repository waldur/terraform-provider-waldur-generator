package generator

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// cleanupImports runs goimports or gofmt on generated Go files to clean up formatting and imports
func (g *Generator) cleanupImports() error {
	// Try goimports first (removes unused imports + formats)
	toolPath, err := exec.LookPath("goimports")
	if err != nil {
		// Fall back to gofmt (only formats, doesn't remove unused imports)
		toolPath, err = exec.LookPath("gofmt")
		if err != nil {
			// Neither tool available, skip cleanup
			return nil
		}
	}

	// Clean up datasources
	datasourcesDir := filepath.Join(g.config.Generator.OutputDir, "internal", "datasources")
	cmd := exec.Command(toolPath, "-w", datasourcesDir)
	if err := cmd.Run(); err != nil {
		// Don't fail hard on formatting errors, just log and continue
		fmt.Printf("Warning: failed to format datasources: %v\n", err)
	}

	// Clean up resources
	resourcesDir := filepath.Join(g.config.Generator.OutputDir, "internal", "resources")
	cmd = exec.Command(toolPath, "-w", resourcesDir)
	if err := cmd.Run(); err != nil {
		fmt.Printf("Warning: failed to format resources: %v\n", err)
	}

	// Clean up services
	servicesDir := filepath.Join(g.config.Generator.OutputDir, "services")
	cmd = exec.Command(toolPath, "-w", servicesDir)
	if err := cmd.Run(); err != nil {
		fmt.Printf("Warning: failed to format services: %v\n", err)
	}

	return nil
}

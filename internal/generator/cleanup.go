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

	// Clean up internal
	commonDir := filepath.Join(g.config.Generator.OutputDir, "internal")
	cmd := exec.Command(toolPath, "-w", commonDir)
	if err := cmd.Run(); err != nil {
		fmt.Printf("Warning: failed to format internal: %v\n", err)
	}

	// Clean up services (includes all resources and datasources)
	servicesDir := filepath.Join(g.config.Generator.OutputDir, "services")
	cmd = exec.Command(toolPath, "-w", servicesDir)
	if err := cmd.Run(); err != nil {
		fmt.Printf("Warning: failed to format services: %v\n", err)
	}

	return nil
}

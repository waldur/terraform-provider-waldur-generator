package generator

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// cleanupImports runs goimports or gofmt on generated Go files to clean up formatting and imports
func (g *Generator) cleanupImports() error {
	// Templates import terraform-plugin-framework/types unconditionally and rely
	// on this pass to drop it where it is unused, which is what keeps them
	// structural. That makes goimports a requirement rather than a nicety: gofmt
	// formats but leaves unused imports, so falling back to it silently yields
	// output that does not compile. Say so instead of letting the caller
	// rediscover it as a confusing build error.
	toolPath, err := exec.LookPath("goimports")
	if err != nil {
		toolPath, err = exec.LookPath("gofmt")
		if err != nil {
			fmt.Println(
				"Warning: neither goimports nor gofmt found; generated code is unformatted " +
					"and will not compile. Install goimports: " +
					"go install golang.org/x/tools/cmd/goimports@latest",
			)
			return nil
		}
		fmt.Println(
			"Warning: goimports not found, falling back to gofmt. Unused imports will " +
				"remain and the generated code will not compile. Install goimports: " +
				"go install golang.org/x/tools/cmd/goimports@latest",
		)
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

	// Clean up e2e_test
	e2eDir := filepath.Join(g.config.Generator.OutputDir, "e2e_test")
	cmd = exec.Command(toolPath, "-w", e2eDir)
	if err := cmd.Run(); err != nil {
		fmt.Printf("Warning: failed to format e2e_test: %v\n", err)
	}

	return nil
}

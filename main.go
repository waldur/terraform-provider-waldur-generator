package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/waldur/terraform-waldur-provider-generator/internal/config"
	"github.com/waldur/terraform-waldur-provider-generator/internal/generator"
	"github.com/waldur/terraform-waldur-provider-generator/internal/openapi"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Parse OpenAPI schema
	parser, err := openapi.NewParser(cfg.Generator.OpenAPISchema)
	if err != nil {
		log.Fatalf("Error parsing OpenAPI schema: %v", err)
	}

	// Create generator
	gen := generator.New(cfg, parser)

	// Generate provider
	fmt.Printf("Generating Terraform provider for %s...\n", cfg.Generator.ProviderName)
	fmt.Printf("Output directory: %s\n", cfg.Generator.OutputDir)
	fmt.Printf("Resources: %d\n", len(cfg.Resources))
	fmt.Printf("Data sources: %d\n", len(cfg.DataSources))

	if err := gen.Generate(); err != nil {
		log.Fatalf("Error generating provider: %v", err)
	}

	fmt.Printf("\n✅ Provider generated successfully at %s\n", cfg.Generator.OutputDir)
	fmt.Println("\nNext steps:")
	fmt.Printf("  1. cd %s\n", cfg.Generator.OutputDir)
	fmt.Println("  2. go mod tidy")
	fmt.Println("  3. go build")
}

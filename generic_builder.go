package rubyext

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GenericBuilder provides a configurable builder for any language that
// can compile to shared libraries.
//
// This builder is designed to support modern systems languages like Crystal,
// Zig, Nim, Odin, D, V, Swift, and others without requiring a new Go file
// for each language.
//
// # Configuration
//
// GenericBuilder is configured with:
//   - File patterns to detect (e.g., "*.cr", "build.zig")
//   - Required tools and alternatives
//   - Build command template
//   - Output file patterns
//
// # Example: Crystal
//
//	crystal := NewGenericBuilder(GenericBuilderConfig{
//	    Name:     "Crystal",
//	    Patterns: []string{"*.cr", "shard.yml"},
//	    Tools: []ToolRequirement{
//	        {Name: "crystal", Purpose: "Crystal compiler"},
//	    },
//	    BuildCommand: []string{
//	        "crystal", "build", "--single-module",
//	        "--link-flags=-shared", "-o", "{{output}}", "{{input}}",
//	    },
//	    OutputPatterns: []string{"*.so", "*.dylib", "*.dll"},
//	})
type GenericBuilder struct {
	name           string
	patterns       []string
	tools          []ToolRequirement
	buildCommand   []string
	cleanCommand   []string
	outputPatterns []string
}

// GenericBuilderConfig defines configuration for a GenericBuilder.
type GenericBuilderConfig struct {
	// Name is the human-readable builder name (e.g., "Crystal", "Zig")
	Name string

	// Patterns are file patterns to match (e.g., "*.cr", "build.zig")
	Patterns []string

	// Tools are the required build tools
	Tools []ToolRequirement

	// BuildCommand is the command template to build the extension.
	// Supports placeholders:
	//   {{input}}  - The input file (e.g., extension.cr)
	//   {{output}} - The output file (e.g., extension.so)
	//   {{dir}}    - The extension directory
	BuildCommand []string

	// CleanCommand is an optional command to clean build artifacts
	CleanCommand []string

	// OutputPatterns are glob patterns to find built extensions
	// (e.g., "*.so", "*.dylib", "zig-out/lib/*.so")
	OutputPatterns []string
}

// NewGenericBuilder creates a new GenericBuilder from configuration.
func NewGenericBuilder(config *GenericBuilderConfig) *GenericBuilder {
	return &GenericBuilder{
		name:           config.Name,
		patterns:       config.Patterns,
		tools:          config.Tools,
		buildCommand:   config.BuildCommand,
		cleanCommand:   config.CleanCommand,
		outputPatterns: config.OutputPatterns,
	}
}

// Name returns the builder name
func (b *GenericBuilder) Name() string {
	return b.name
}

// RequiredTools returns the tools needed for this builder
func (b *GenericBuilder) RequiredTools() []ToolRequirement {
	return b.tools
}

// CheckTools verifies that all required tools are available
func (b *GenericBuilder) CheckTools() error {
	return CheckRequiredTools(b.RequiredTools())
}

// CanBuild checks if this builder can handle the extension file
func (b *GenericBuilder) CanBuild(extensionFile string) bool {
	filename := strings.ToLower(filepath.Base(extensionFile))

	for _, pattern := range b.patterns {
		// Support both exact matches and glob patterns
		if matched, _ := filepath.Match(strings.ToLower(pattern), filename); matched {
			return true
		}
	}

	return false
}

// Build compiles the extension using the configured build command
func (b *GenericBuilder) Build(ctx context.Context, config *BuildConfig, extensionFile string) (*BuildResult, error) {
	return runCommonBuild(ctx, config, extensionFile, CommonBuildSteps{
		ConfigureFunc: b.noConfigure,
		BuildFunc:     b.runBuild,
		FindFunc:      b.findBuiltExtensions,
	})
}

// Clean removes build artifacts using the configured clean command
func (b *GenericBuilder) Clean(ctx context.Context, config *BuildConfig, extensionFile string) error {
	if len(b.cleanCommand) == 0 {
		return nil // No clean command configured
	}

	extensionPath := filepath.Join(config.GemDir, extensionFile)
	extensionDir := filepath.Dir(extensionPath)

	// Execute clean command
	//nolint:gosec // Command is from trusted builder configuration
	cmd := exec.CommandContext(ctx, b.cleanCommand[0], b.cleanCommand[1:]...)
	cmd.Dir = extensionDir

	// Ignore errors - clean may not be necessary
	_ = cmd.Run()
	return nil
}

// noConfigure is a no-op since generic builders don't need configuration
func (b *GenericBuilder) noConfigure(ctx context.Context, config *BuildConfig, extensionDir string, result *BuildResult) error {
	if config.Verbose {
		result.Output = append(result.Output, fmt.Sprintf("%s builder, no configuration needed", b.name))
	}
	return nil
}

// runBuild executes the configured build command
func (b *GenericBuilder) runBuild(ctx context.Context, config *BuildConfig, extensionDir string, result *BuildResult) error {
	if len(b.buildCommand) == 0 {
		return fmt.Errorf("no build command configured for %s builder", b.name)
	}

	// Prepare command with placeholder substitution
	inputFile := filepath.Base(extensionDir) // Default input
	outputFile := "extension.so"             // Default output

	// If dest path specified, place output there
	if config.DestPath != "" {
		outputFile = filepath.Join(config.DestPath, outputFile)
	}

	// Replace placeholders in build command
	args := make([]string, len(b.buildCommand))
	for i, arg := range b.buildCommand {
		arg = strings.ReplaceAll(arg, "{{input}}", inputFile)
		arg = strings.ReplaceAll(arg, "{{output}}", outputFile)
		arg = strings.ReplaceAll(arg, "{{dir}}", extensionDir)
		args[i] = arg
	}

	// Add any additional build args from config
	args = append(args, config.BuildArgs...)

	// Execute build command
	//nolint:gosec // Command is from trusted builder configuration
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = extensionDir

	// Set environment variables
	cmd.Env = os.Environ()
	for key, value := range config.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	output, err := cmd.CombinedOutput()
	outputLines := strings.Split(string(output), "\n")
	result.Output = append(result.Output, outputLines...)

	if config.Verbose {
		result.Output = append(result.Output,
			fmt.Sprintf("Running: %s", strings.Join(args, " ")),
			fmt.Sprintf("Working directory: %s", extensionDir))
	}

	if err != nil {
		return BuildError(b.name, result.Output, err)
	}

	return nil
}

// findBuiltExtensions locates compiled extension files using configured patterns
func (b *GenericBuilder) findBuiltExtensions(extensionDir string) ([]string, error) {
	var extensions []string

	for _, pattern := range b.outputPatterns {
		matches, err := filepath.Glob(filepath.Join(extensionDir, pattern))
		if err != nil {
			return nil, fmt.Errorf("failed to glob pattern %s in %s: %v", pattern, extensionDir, err)
		}

		for _, match := range matches {
			// Convert to relative path
			relPath, err := filepath.Rel(extensionDir, match)
			if err == nil {
				extensions = append(extensions, relPath)
			}
		}
	}

	return extensions, nil
}

// Predefined language configurations for common languages

// NewCrystalBuilder creates a builder for Crystal extensions.
func NewCrystalBuilder() *GenericBuilder {
	return NewGenericBuilder(&GenericBuilderConfig{
		Name:     "Crystal",
		Patterns: []string{"*.cr", "shard.yml"},
		Tools: []ToolRequirement{
			{Name: "crystal", Purpose: "Crystal compiler"},
		},
		BuildCommand: []string{
			"crystal", "build", "--single-module",
			"--link-flags=-shared", "-o", "{{output}}", "{{input}}",
		},
		OutputPatterns: []string{"*.so", "*.dylib", "*.dll"},
	})
}

// NewZigBuilder creates a builder for Zig extensions.
func NewZigBuilder() *GenericBuilder {
	return NewGenericBuilder(&GenericBuilderConfig{
		Name:     "Zig",
		Patterns: []string{"build.zig", "*.zig"},
		Tools: []ToolRequirement{
			{Name: "zig", Purpose: "Zig compiler and build system"},
		},
		BuildCommand: []string{
			"zig", "build-lib", "-dynamic",
			"-O", "ReleaseFast", "{{input}}",
		},
		OutputPatterns: []string{"*.so", "*.dylib", "*.dll", "zig-out/lib/*.so"},
	})
}

// NewSwiftBuilder creates a builder for Swift extensions.
func NewSwiftBuilder() *GenericBuilder {
	return NewGenericBuilder(&GenericBuilderConfig{
		Name:     "Swift",
		Patterns: []string{"*.swift", "Package.swift"},
		Tools: []ToolRequirement{
			{Name: "swiftc", Purpose: "Swift compiler"},
		},
		BuildCommand: []string{
			"swiftc", "-emit-library", "-o", "{{output}}", "{{input}}",
		},
		OutputPatterns: []string{"*.so", "*.dylib", "*.dll"},
	})
}

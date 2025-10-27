package rubyext

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GoBuilder handles Go-based builds using CGO to create shared libraries.
//
// This builder compiles Go code into shared libraries (.so/.dll/.dylib)
// that can be loaded by Ruby using FFI or similar mechanisms.
//
// Common use cases:
//   - High-performance extensions written in Go
//   - Reusing existing Go libraries in Ruby
//   - Cross-platform extensions leveraging Go's portability
//
// Build command:
//
//	go build -buildmode=c-shared -o extension.so
type GoBuilder struct{}

// Name returns the builder name
func (b *GoBuilder) Name() string {
	return "Go"
}

// RequiredTools returns the tools needed for Go builds
func (b *GoBuilder) RequiredTools() []ToolRequirement {
	return []ToolRequirement{
		{
			Name:    "go",
			Purpose: "Go compiler and toolchain",
		},
		{
			Name:         "gcc",
			Alternatives: []string{"clang", "cc"},
			Purpose:      "C compiler (required for CGO)",
		},
	}
}

// CheckTools verifies that Go toolchain is available
func (b *GoBuilder) CheckTools() error {
	return CheckRequiredTools(b.RequiredTools())
}

// CanBuild checks if this builder can handle the extension file
func (b *GoBuilder) CanBuild(extensionFile string) bool {
	// Look for .go files or go.mod
	ext := strings.ToLower(filepath.Ext(extensionFile))
	base := strings.ToLower(filepath.Base(extensionFile))
	return ext == ".go" || base == "go.mod"
}

// Build compiles the Go extension into a shared library
func (b *GoBuilder) Build(ctx context.Context, config *BuildConfig, extensionFile string) (*BuildResult, error) {
	return runCommonBuild(ctx, config, extensionFile, CommonBuildSteps{
		ConfigureFunc: b.noConfigure,
		BuildFunc:     b.runGoBuild,
		FindFunc:      b.findBuiltExtensions,
	})
}

// Clean removes build artifacts
func (b *GoBuilder) Clean(ctx context.Context, config *BuildConfig, extensionFile string) error {
	extensionPath := filepath.Join(config.GemDir, extensionFile)
	extensionDir := filepath.Dir(extensionPath)

	cleanCmd := exec.CommandContext(ctx, "go", "clean")
	cleanCmd.Dir = extensionDir

	// Ignore errors - clean may not be necessary
	_ = cleanCmd.Run()
	return nil
}

// noConfigure is a no-op since Go doesn't need configuration
func (b *GoBuilder) noConfigure(ctx context.Context, config *BuildConfig, extensionDir string, result *BuildResult) error {
	if config.Verbose {
		result.Output = append(result.Output, "Go modules, no configuration needed")
	}
	return nil
}

const (
	defaultExtensionName = "extension.so"
)

// runGoBuild executes go build to compile the shared library
func (b *GoBuilder) runGoBuild(ctx context.Context, config *BuildConfig, extensionDir string, result *BuildResult) error {
	// Determine output filename
	outputName := defaultExtensionName
	if config.DestPath != "" {
		outputName = filepath.Join(config.DestPath, outputName)
	}

	// Build go build arguments
	args := []string{"build", "-buildmode=c-shared", "-o", outputName}

	// Add any additional build args
	args = append(args, config.BuildArgs...)

	// Run go build
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = extensionDir

	// Set environment variables
	cmd.Env = os.Environ()
	for key, value := range config.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	// Enable CGO
	cmd.Env = append(cmd.Env, "CGO_ENABLED=1")

	output, err := cmd.CombinedOutput()
	outputLines := strings.Split(string(output), "\n")
	result.Output = append(result.Output, outputLines...)

	if config.Verbose {
		result.Output = append(result.Output,
			fmt.Sprintf("Running: go %s", strings.Join(args, " ")),
			fmt.Sprintf("Working directory: %s", extensionDir))
	}

	if err != nil {
		return BuildError("Go", result.Output, err)
	}

	return nil
}

// findBuiltExtensions locates the compiled shared library files
func (b *GoBuilder) findBuiltExtensions(extensionDir string) ([]string, error) {
	var extensions []string

	// Go builds produce .so, .dll, or .dylib depending on platform
	patterns := []string{
		"*.so",    // Linux
		"*.dylib", // macOS
		"*.dll",   // Windows
	}

	for _, pattern := range patterns {
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

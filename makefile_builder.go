package rubyext

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// MakefileBuilder handles plain Makefile-based builds.
//
// This builder handles gems that provide a Makefile directly without
// using extconf.rb, configure scripts, or other build systems.
//
// Common in:
//   - Simple C extensions with handwritten Makefiles
//   - Legacy gems that predate extconf.rb conventions
//   - Cross-platform extensions with custom build logic
type MakefileBuilder struct{}

// Name returns the builder name
func (b *MakefileBuilder) Name() string {
	return "Makefile"
}

// RequiredTools returns the tools needed for Makefile builds
func (b *MakefileBuilder) RequiredTools() []ToolRequirement {
	return []ToolRequirement{
		{
			Name:         "make",
			Alternatives: []string{"gmake", "nmake"},
			Purpose:      "Build automation tool",
		},
		{
			Name:         "gcc",
			Alternatives: []string{"clang", "cc", "cl"},
			Purpose:      "C/C++ compiler",
		},
	}
}

// CheckTools verifies that make and compiler are available
func (b *MakefileBuilder) CheckTools() error {
	return CheckRequiredTools(b.RequiredTools())
}

// CanBuild checks if this builder can handle the extension file
func (b *MakefileBuilder) CanBuild(extensionFile string) bool {
	filename := strings.ToLower(filepath.Base(extensionFile))
	// Match Makefile, makefile, GNUmakefile
	return filename == "makefile" || filename == "gnumakefile"
}

// Build compiles the extension using make
func (b *MakefileBuilder) Build(ctx context.Context, config *BuildConfig, extensionFile string) (*BuildResult, error) {
	return runCommonBuild(ctx, config, extensionFile, CommonBuildSteps{
		ConfigureFunc: b.noConfigure,
		BuildFunc:     b.runMake,
		FindFunc:      b.findBuiltExtensions,
	})
}

// Clean removes build artifacts
func (b *MakefileBuilder) Clean(ctx context.Context, config *BuildConfig, extensionFile string) error {
	extensionPath := filepath.Join(config.GemDir, extensionFile)
	extensionDir := filepath.Dir(extensionPath)

	makeProgram := b.getMakeProgram()
	cleanCmd := exec.CommandContext(ctx, makeProgram, "clean")
	cleanCmd.Dir = extensionDir

	// Ignore errors - clean target may not exist
	_ = cleanCmd.Run()
	return nil
}

// noConfigure is a no-op since Makefile doesn't need configuration
func (b *MakefileBuilder) noConfigure(ctx context.Context, config *BuildConfig, extensionDir string, result *BuildResult) error {
	if config.Verbose {
		result.Output = append(result.Output, "Using existing Makefile, no configuration needed")
	}
	return nil
}

// runMake executes make to compile the extension
//
//nolint:dupl // Similar to extconf_builder but with different context
func (b *MakefileBuilder) runMake(ctx context.Context, config *BuildConfig, extensionDir string, result *BuildResult) error {
	makeProgram := b.getMakeProgram()

	// Build make arguments
	args := []string{}

	// Add parallel jobs if specified
	if config.Parallel > 0 {
		args = append(args, fmt.Sprintf("-j%d", config.Parallel))
	}

	// Clean first if requested
	if config.CleanFirst {
		cleanCmd := exec.CommandContext(ctx, makeProgram, "clean")
		cleanCmd.Dir = extensionDir
		cleanOutput, _ := cleanCmd.CombinedOutput()
		result.Output = append(result.Output, strings.Split(string(cleanOutput), "\n")...)
	}

	// Run make
	cmd := exec.CommandContext(ctx, makeProgram, args...)
	cmd.Dir = extensionDir

	// Set environment variables
	cmd.Env = os.Environ()
	for key, value := range config.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	// Set DESTDIR if dest path is specified
	if config.DestPath != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("DESTDIR=%s", config.DestPath))
	}

	output, err := cmd.CombinedOutput()
	outputLines := strings.Split(string(output), "\n")
	result.Output = append(result.Output, outputLines...)

	if config.Verbose {
		result.Output = append(result.Output,
			fmt.Sprintf("Running: %s %s", makeProgram, strings.Join(args, " ")),
			fmt.Sprintf("Working directory: %s", extensionDir))
	}

	if err != nil {
		return BuildError("Make", result.Output, err)
	}

	// Run make install if dest path is specified
	if config.DestPath != "" {
		installCmd := exec.CommandContext(ctx, makeProgram, "install")
		installCmd.Dir = extensionDir
		installCmd.Env = cmd.Env

		installOutput, err := installCmd.CombinedOutput()
		installLines := strings.Split(string(installOutput), "\n")
		result.Output = append(result.Output, installLines...)

		if err != nil {
			return BuildError("Make Install", result.Output, err)
		}
	}

	return nil
}

// findBuiltExtensions locates the compiled extension files
func (b *MakefileBuilder) findBuiltExtensions(extensionDir string) ([]string, error) {
	var extensions []string

	// Common extension file patterns
	patterns := []string{
		"*.so",     // Linux/Unix shared libraries
		"*.bundle", // macOS bundles
		"*.dll",    // Windows dynamic libraries
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

// getMakeProgram returns the appropriate make program for the platform
func (b *MakefileBuilder) getMakeProgram() string {
	// Check environment variable first
	if makeEnv := os.Getenv("MAKE"); makeEnv != "" {
		return makeEnv
	}

	// Platform-specific defaults
	switch runtime.GOOS {
	case platformWindows:
		return nmakeProgram
	default:
		return makeProgram
	}
}

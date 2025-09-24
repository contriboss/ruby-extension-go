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

// Build tool constants
const (
	unixMakefiles = "Unix Makefiles"
	nmakeProgram  = "nmake"
	makeProgram   = "make"
)

// CmakeBuilder handles CMake-based builds
type CmakeBuilder struct{}

// Name returns the builder name
func (b *CmakeBuilder) Name() string {
	return "CMake"
}

// CanBuild checks if this builder can handle the extension file
func (b *CmakeBuilder) CanBuild(extensionFile string) bool {
	return MatchesPattern(extensionFile, `CMakeLists\.txt$`)
}

// Build compiles the extension using the cmake → make workflow
func (b *CmakeBuilder) Build(ctx context.Context, config *BuildConfig, extensionFile string) (*BuildResult, error) {
	return runCommonBuild(ctx, config, extensionFile, CommonBuildSteps{
		ConfigureFunc: b.runCmake,
		BuildFunc:     b.runBuild,
		FindFunc:      b.findBuiltExtensions,
	})
}

// Clean removes build artifacts
func (b *CmakeBuilder) Clean(ctx context.Context, config *BuildConfig, extensionFile string) error {
	extensionPath := filepath.Join(config.GemDir, extensionFile)
	extensionDir := filepath.Dir(extensionPath)

	// Try cmake --build . --target clean first
	cleanCmd := exec.CommandContext(ctx, "cmake", "--build", ".", "--target", "clean")
	cleanCmd.Dir = extensionDir
	if err := cleanCmd.Run(); err != nil {
		// Fall back to make clean if available
		makefilePath := filepath.Join(extensionDir, "Makefile")
		if _, err := os.Stat(makefilePath); err == nil {
			makeProgram := b.getMakeProgram()
			makeCmd := exec.CommandContext(ctx, makeProgram, "clean")
			makeCmd.Dir = extensionDir
			return makeCmd.Run()
		}
	}

	return nil
}

// runCmake executes cmake to configure the build
func (b *CmakeBuilder) runCmake(ctx context.Context, config *BuildConfig, extensionDir string, result *BuildResult) error {
	// Build cmake arguments
	args := []string{"."}

	// Set install prefix if dest path is specified
	if config.DestPath != "" {
		args = append(args, fmt.Sprintf("-DCMAKE_INSTALL_PREFIX=%s", config.DestPath))
	}

	// Set build type to Release by default
	args = append(args, "-DCMAKE_BUILD_TYPE=Release")

	// Platform-specific generator selection
	generator := b.getGenerator()
	if generator != "" {
		args = append(args, "-G", generator)
	}

	// Add any custom build args
	args = append(args, config.BuildArgs...)

	cmd := exec.CommandContext(ctx, "cmake", args...)
	cmd.Dir = extensionDir

	// Set environment variables
	cmd.Env = os.Environ()
	for key, value := range config.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	// Set Ruby-related CMake variables
	if config.RubyPath != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("Ruby_EXECUTABLE=%s", config.RubyPath))
	}

	output, err := cmd.CombinedOutput()
	outputLines := strings.Split(string(output), "\n")
	result.Output = append(result.Output, outputLines...)

	if config.Verbose {
		result.Output = append(result.Output,
			fmt.Sprintf("Running: cmake %s", strings.Join(args, " ")),
			fmt.Sprintf("Working directory: %s", extensionDir))
	}

	if err != nil {
		return BuildError("CMake", result.Output, err)
	}

	return nil
}

// runBuild executes the build command
func (b *CmakeBuilder) runBuild(ctx context.Context, config *BuildConfig, extensionDir string, result *BuildResult) error {
	// Use cmake --build for cross-platform building
	args := []string{"--build", "."}

	// Add parallel jobs if specified
	if config.Parallel > 0 {
		args = append(args, "--parallel", fmt.Sprintf("%d", config.Parallel))
	}

	// Clean first if requested
	if config.CleanFirst {
		cleanArgs := []string{"--build", ".", "--target", "clean"}
		cleanCmd := exec.CommandContext(ctx, "cmake", cleanArgs...)
		cleanCmd.Dir = extensionDir
		cleanOutput, _ := cleanCmd.CombinedOutput()
		result.Output = append(result.Output, strings.Split(string(cleanOutput), "\n")...)
	}

	// Build configuration (Release by default)
	args = append(args, "--config", "Release")

	cmd := exec.CommandContext(ctx, "cmake", args...)
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
			fmt.Sprintf("Running: cmake %s", strings.Join(args, " ")),
			fmt.Sprintf("Working directory: %s", extensionDir))
	}

	if err != nil {
		return BuildError("CMake Build", result.Output, err)
	}

	// Run install if dest path is specified
	if config.DestPath != "" {
		installArgs := []string{"--install", "."}
		installCmd := exec.CommandContext(ctx, "cmake", installArgs...)
		installCmd.Dir = extensionDir
		installCmd.Env = cmd.Env

		installOutput, err := installCmd.CombinedOutput()
		installLines := strings.Split(string(installOutput), "\n")
		result.Output = append(result.Output, installLines...)

		if err != nil {
			return BuildError("CMake Install", result.Output, err)
		}
	}

	return nil
}

// findBuiltExtensions locates the compiled extension files
func (b *CmakeBuilder) findBuiltExtensions(extensionDir string) ([]string, error) {
	var extensions []string

	// CMake can output to various directories depending on configuration
	searchDirs := []string{
		".",       // Current directory
		"Release", // Release build directory
		"Debug",   // Debug build directory
		"lib",     // Common library output
		"bin",     // Common binary output
		"build",   // Common build directory
		"_builds", // Some CMake setups use this
	}

	// Common extension file patterns
	patterns := []string{
		"*.so",     // Linux/Unix shared libraries
		"*.bundle", // macOS bundles
		"*.dll",    // Windows dynamic libraries
		"*.dylib",  // macOS dynamic libraries
	}

	for _, searchDir := range searchDirs {
		fullSearchDir := filepath.Join(extensionDir, searchDir)
		if _, err := os.Stat(fullSearchDir); os.IsNotExist(err) {
			continue
		}

		for _, pattern := range patterns {
			matches, err := filepath.Glob(filepath.Join(fullSearchDir, pattern))
			if err != nil {
				return nil, fmt.Errorf("failed to glob pattern %s in %s: %v", pattern, fullSearchDir, err)
			}

			for _, match := range matches {
				// Convert to relative path from extension directory
				relPath, err := filepath.Rel(extensionDir, match)
				if err == nil {
					extensions = append(extensions, relPath)
				}
			}
		}
	}

	return extensions, nil
}

// getGenerator returns the appropriate CMake generator for the platform
func (b *CmakeBuilder) getGenerator() string {
	// Check environment variable first
	if generator := os.Getenv("CMAKE_GENERATOR"); generator != "" {
		return generator
	}

	// Platform-specific defaults
	switch runtime.GOOS {
	case platformWindows:
		// Prefer Visual Studio if available, otherwise MinGW
		return "Visual Studio 16 2019" // Modern default
	case "darwin":
		return unixMakefiles // Xcode also available
	default:
		return unixMakefiles
	}
}

// getMakeProgram returns the appropriate make program for the platform
func (b *CmakeBuilder) getMakeProgram() string {
	// Check environment variable first
	if makeProgram := os.Getenv("MAKE"); makeProgram != "" {
		return makeProgram
	}

	// Platform-specific defaults
	switch runtime.GOOS {
	case platformWindows:
		return nmakeProgram
	default:
		return makeProgram
	}
}

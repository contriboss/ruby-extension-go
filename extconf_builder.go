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

// ExtConfBuilder handles extconf.rb files - the most common Ruby extension build system
type ExtConfBuilder struct{}

// Name returns the builder name
func (b *ExtConfBuilder) Name() string {
	return "ExtConf"
}

// CanBuild checks if this builder can handle the extension file
func (b *ExtConfBuilder) CanBuild(extensionFile string) bool {
	return MatchesPattern(extensionFile, `extconf\.rb$`)
}

// Build compiles the extension using the extconf.rb → make workflow
func (b *ExtConfBuilder) Build(ctx context.Context, config *BuildConfig, extensionFile string) (*BuildResult, error) {
	result := &BuildResult{
		Success: false,
		Output:  []string{},
	}

	extensionPath := filepath.Join(config.GemDir, extensionFile)
	extensionDir := filepath.Dir(extensionPath)
	
	// Step 1: Run ruby extconf.rb to generate Makefile
	if err := b.runExtConf(ctx, config, extensionDir, result); err != nil {
		result.Error = err
		return result, err
	}

	// Step 2: Run make to compile the extension
	if err := b.runMake(ctx, config, extensionDir, result); err != nil {
		result.Error = err
		return result, err
	}

	// Step 3: Find built extensions
	extensions, err := b.findBuiltExtensions(extensionDir)
	if err != nil {
		result.Error = err
		return result, err
	}
	
	result.Extensions = extensions
	result.Success = true
	return result, nil
}

// Clean removes build artifacts
func (b *ExtConfBuilder) Clean(ctx context.Context, config *BuildConfig, extensionFile string) error {
	extensionPath := filepath.Join(config.GemDir, extensionFile)
	extensionDir := filepath.Dir(extensionPath)
	
	makefilePath := filepath.Join(extensionDir, "Makefile")
	if _, err := os.Stat(makefilePath); os.IsNotExist(err) {
		return nil // Nothing to clean
	}
	
	makeProgram := b.getMakeProgram()
	cmd := exec.CommandContext(ctx, makeProgram, "clean")
	cmd.Dir = extensionDir
	
	return cmd.Run()
}

// runExtConf executes ruby extconf.rb to generate the Makefile
func (b *ExtConfBuilder) runExtConf(ctx context.Context, config *BuildConfig, extensionDir string, result *BuildResult) error {
	rubyPath := config.RubyPath
	if rubyPath == "" {
		rubyPath = "ruby"
	}
	
	args := []string{"extconf.rb"}
	args = append(args, config.BuildArgs...)
	
	cmd := exec.CommandContext(ctx, rubyPath, args...)
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
		result.Output = append(result.Output, fmt.Sprintf("Running: %s %s", rubyPath, strings.Join(args, " ")))
		result.Output = append(result.Output, fmt.Sprintf("Working directory: %s", extensionDir))
	}
	
	if err != nil {
		return BuildError("ExtConf", result.Output, err)
	}
	
	// Verify Makefile was created
	makefilePath := filepath.Join(extensionDir, "Makefile")
	if _, err := os.Stat(makefilePath); os.IsNotExist(err) {
		return BuildError("ExtConf", result.Output, fmt.Errorf("Makefile not generated"))
	}
	
	return nil
}

// runMake executes make to compile the extension
func (b *ExtConfBuilder) runMake(ctx context.Context, config *BuildConfig, extensionDir string, result *BuildResult) error {
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
		result.Output = append(result.Output, fmt.Sprintf("Running: %s %s", makeProgram, strings.Join(args, " ")))
		result.Output = append(result.Output, fmt.Sprintf("Working directory: %s", extensionDir))
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
func (b *ExtConfBuilder) findBuiltExtensions(extensionDir string) ([]string, error) {
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
			continue
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
func (b *ExtConfBuilder) getMakeProgram() string {
	// Check environment variable first
	if makeProgram := os.Getenv("MAKE"); makeProgram != "" {
		return makeProgram
	}
	
	// Platform-specific defaults
	switch runtime.GOOS {
	case "windows":
		return "nmake" // Visual Studio's make
	default:
		return "make"
	}
}
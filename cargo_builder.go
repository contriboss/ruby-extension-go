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

// Platform constants
const (
	platformWindows = "windows"
	platformDarwin  = "darwin"
)

// CargoBuilder handles Rust-based builds using Cargo
type CargoBuilder struct{}

// Name returns the builder name
func (b *CargoBuilder) Name() string {
	return "Cargo"
}

// CanBuild checks if this builder can handle the extension file
func (b *CargoBuilder) CanBuild(extensionFile string) bool {
	return MatchesPattern(extensionFile, `Cargo\.toml$`)
}

// Build compiles the extension using cargo
func (b *CargoBuilder) Build(ctx context.Context, config *BuildConfig, extensionFile string) (*BuildResult, error) {
	result := &BuildResult{
		Success: false,
		Output:  []string{},
	}

	extensionPath := filepath.Join(config.GemDir, extensionFile)
	extensionDir := filepath.Dir(extensionPath)

	// Step 1: Run cargo to build the Rust extension
	if err := b.runCargo(ctx, config, extensionDir, result); err != nil {
		result.Error = err
		return result, err
	}

	// Step 2: Find and rename built extensions to Ruby's expected format
	if err := b.processBuiltExtensions(ctx, config, extensionDir, result); err != nil {
		result.Error = err
		return result, err
	}

	result.Success = true
	return result, nil
}

// Clean removes build artifacts
func (b *CargoBuilder) Clean(ctx context.Context, config *BuildConfig, extensionFile string) error {
	extensionPath := filepath.Join(config.GemDir, extensionFile)
	extensionDir := filepath.Dir(extensionPath)

	cmd := exec.CommandContext(ctx, "cargo", "clean")
	cmd.Dir = extensionDir

	return cmd.Run()
}

// runCargo executes cargo to build the Rust extension
func (b *CargoBuilder) runCargo(ctx context.Context, config *BuildConfig, extensionDir string, result *BuildResult) error {
	cargoPath := b.getCargoPath()

	// Build cargo arguments
	args := []string{"rustc", "--release", "--crate-type", "cdylib"}

	// Add target if specified
	if target := os.Getenv("CARGO_BUILD_TARGET"); target != "" {
		args = append(args, "--target", target)
	}

	// Use locked dependencies if Cargo.lock exists
	lockPath := filepath.Join(extensionDir, "Cargo.lock")
	if _, err := os.Stat(lockPath); err == nil {
		args = append(args, "--locked")
	}

	// Add parallel jobs if specified
	if config.Parallel > 0 {
		args = append(args, "--jobs", fmt.Sprintf("%d", config.Parallel))
	}

	// Clean first if requested
	if config.CleanFirst {
		cleanCmd := exec.CommandContext(ctx, cargoPath, "clean")
		cleanCmd.Dir = extensionDir
		cleanOutput, _ := cleanCmd.CombinedOutput()
		result.Output = append(result.Output, strings.Split(string(cleanOutput), "\n")...)
	}

	// Add any custom build args
	args = append(args, config.BuildArgs...)

	// Add rustc-specific arguments for Ruby integration
	args = append(args, "--")
	args = append(args, b.getRustcArgs(config)...)

	cmd := exec.CommandContext(ctx, cargoPath, args...)
	cmd.Dir = extensionDir

	// Set environment variables for Rust/Ruby integration
	cmd.Env = os.Environ()
	for key, value := range config.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	// Set Ruby-specific environment variables
	cmd.Env = append(cmd.Env, b.getRubyEnv(config)...)

	output, err := cmd.CombinedOutput()
	outputLines := strings.Split(string(output), "\n")
	result.Output = append(result.Output, outputLines...)

	if config.Verbose {
		result.Output = append(result.Output,
			fmt.Sprintf("Running: %s %s", cargoPath, strings.Join(args, " ")),
			fmt.Sprintf("Working directory: %s", extensionDir))
	}

	if err != nil {
		return BuildError("Cargo", result.Output, err)
	}

	return nil
}

// processBuiltExtensions finds built Rust libraries and renames them for Ruby
func (b *CargoBuilder) processBuiltExtensions(_ context.Context, config *BuildConfig, extensionDir string, result *BuildResult) error {
	// Find the target directory
	targetDir := filepath.Join(extensionDir, "target")
	if target := os.Getenv("CARGO_BUILD_TARGET"); target != "" {
		targetDir = filepath.Join(targetDir, target)
	}
	targetDir = filepath.Join(targetDir, "release")

	// Find built dynamic libraries
	builtLibs, err := b.findCargoOutputs(targetDir)
	if err != nil {
		return BuildError("Cargo", result.Output, fmt.Errorf("failed to find cargo outputs: %v", err))
	}

	if len(builtLibs) == 0 {
		return BuildError("Cargo", result.Output, fmt.Errorf("no dynamic libraries found in %s", targetDir))
	}

	// Process each built library
	for _, lib := range builtLibs {
		// Convert Rust library name to Ruby extension name
		rubyExtName := b.getRubyExtensionName(lib)
		rubyExtPath := filepath.Join(extensionDir, rubyExtName)

		// Copy the library to the expected location
		if err := b.copyFile(lib, rubyExtPath); err != nil {
			return BuildError("Cargo", result.Output, fmt.Errorf("failed to copy %s to %s: %v", lib, rubyExtPath, err))
		}

		// Add to results
		relPath, _ := filepath.Rel(extensionDir, rubyExtPath)
		result.Extensions = append(result.Extensions, relPath)

		if config.Verbose {
			result.Output = append(result.Output, fmt.Sprintf("Copied %s -> %s", lib, rubyExtPath))
		}
	}

	return nil
}

// findCargoOutputs locates built dynamic libraries
func (b *CargoBuilder) findCargoOutputs(targetDir string) ([]string, error) {
	var outputs []string

	// Platform-specific library patterns
	var patterns []string
	switch runtime.GOOS {
	case platformWindows:
		patterns = []string{"*.dll"}
	case platformDarwin:
		patterns = []string{"*.dylib", "lib*.dylib"}
	default:
		patterns = []string{"*.so", "lib*.so"}
	}

	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(targetDir, pattern))
		if err != nil {
			return nil, fmt.Errorf("failed to glob pattern %s: %v", pattern, err)
		}
		outputs = append(outputs, matches...)
	}

	return outputs, nil
}

// getRubyExtensionName converts a Rust library name to Ruby extension format
func (b *CargoBuilder) getRubyExtensionName(libPath string) string {
	filename := filepath.Base(libPath)
	ext := filepath.Ext(filename)

	// Remove lib prefix if present
	filename = strings.TrimPrefix(filename, "lib")

	// Remove original extension and add Ruby's expected extension
	name := strings.TrimSuffix(filename, ext)

	// Ruby expects specific extensions based on platform
	switch runtime.GOOS {
	case platformDarwin:
		return name + ".bundle"
	case platformWindows:
		return name + ".dll"
	default:
		return name + ".so"
	}
}

// getRustcArgs returns rustc arguments for Ruby integration
func (b *CargoBuilder) getRustcArgs(_ *BuildConfig) []string {
	var args []string

	// Platform-specific linking arguments
	switch runtime.GOOS {
	case platformDarwin:
		args = append(args, "-C", "link-arg=-Wl,-undefined,dynamic_lookup")
	case platformWindows:
		// Windows-specific linking
		args = append(args, "-C", "link-arg=-Wl,--dynamicbase", "-C", "link-arg=-Wl,--disable-auto-image-base", "-C", "link-arg=-static-libgcc")
	}

	return args
}

// getRubyEnv returns Ruby-specific environment variables for Cargo
func (b *CargoBuilder) getRubyEnv(config *BuildConfig) []string {
	var env []string

	// Set RUSTFLAGS for Ruby gem configuration
	rustFlags := os.Getenv("RUSTFLAGS")
	rubyFlags := "--cfg=rb_sys_gem --cfg=rubygems"

	if rustFlags != "" {
		rustFlags = fmt.Sprintf("%s %s", rustFlags, rubyFlags)
	} else {
		rustFlags = rubyFlags
	}

	env = append(env, fmt.Sprintf("RUSTFLAGS=%s", rustFlags))

	// Set Ruby-specific variables if available
	if config.RubyPath != "" {
		env = append(env, fmt.Sprintf("RUBY=%s", config.RubyPath))
	}
	if config.RubyVersion != "" {
		env = append(env, fmt.Sprintf("RUBY_VERSION=%s", config.RubyVersion))
	}
	if config.RubyEngine != "" {
		env = append(env, fmt.Sprintf("RUBY_ENGINE=%s", config.RubyEngine))
	}

	return env
}

// getCargoPath returns the path to the cargo executable
func (b *CargoBuilder) getCargoPath() string {
	if cargoPath := os.Getenv("CARGO"); cargoPath != "" {
		return cargoPath
	}
	return "cargo"
}

// copyFile copies a file from src to dst
func (b *CargoBuilder) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// Create destination directory if needed
	if mkdirErr := os.MkdirAll(filepath.Dir(dst), 0755); mkdirErr != nil {
		return mkdirErr
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = destFile.ReadFrom(sourceFile)
	return err
}

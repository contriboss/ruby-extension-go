// Package rubyext provides native extension compilation support for Ruby gems.
// It supports multiple build systems (extconf.rb, Rakefile, CMake, Cargo, etc.)
// Ruby equivalent: Gem::Ext::Builder
package rubyext

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// BuildResult contains the output and status of a build operation
type BuildResult struct {
	Success    bool
	Output     []string
	Extensions []string // List of built extension files (.so/.bundle)
	Error      error
}

// CommonBuildSteps defines the standard 3-step build pattern used by multiple builders
type CommonBuildSteps struct {
	ConfigureFunc func(ctx context.Context, config *BuildConfig, extensionDir string, result *BuildResult) error
	BuildFunc     func(ctx context.Context, config *BuildConfig, extensionDir string, result *BuildResult) error
	FindFunc      func(extensionDir string) ([]string, error)
}

// BuildConfig contains configuration for the build process
type BuildConfig struct {
	// Source paths
	GemDir       string // Root directory of the extracted gem
	ExtensionDir string // Directory containing the extension files
	DestPath     string // Destination for compiled extensions
	LibDir       string // Optional lib directory for extension installation

	// Build arguments
	BuildArgs []string          // Additional build arguments
	Env       map[string]string // Environment variables for build

	// Ruby configuration
	RubyEngine  string // Ruby engine (ruby, jruby, truffleruby)
	RubyVersion string // Ruby version (3.4.0, etc.)
	RubyPath    string // Path to Ruby executable

	// Build options
	Verbose    bool // Enable verbose output
	CleanFirst bool // Run clean before build
	Parallel   int  // Number of parallel jobs (for make -j)
}

// Builder interface defines the contract for all extension builders
type Builder interface {
	// Name returns the human-readable name of this builder
	Name() string

	// CanBuild checks if this builder can handle the given extension file
	CanBuild(extensionFile string) bool

	// Build compiles the extension and returns the result
	Build(ctx context.Context, config *BuildConfig, extensionFile string) (*BuildResult, error)

	// Clean removes build artifacts (optional, some builders may not support this)
	Clean(ctx context.Context, config *BuildConfig, extensionFile string) error
}

// BuilderFactory manages the registration and creation of extension builders
type BuilderFactory struct {
	builders []Builder
}

// NewBuilderFactory creates a new factory with all standard builders registered
func NewBuilderFactory() *BuilderFactory {
	factory := &BuilderFactory{}

	// Register all standard builders
	factory.Register(&ExtConfBuilder{})
	factory.Register(&ConfigureBuilder{})
	factory.Register(&RakeBuilder{})
	factory.Register(&CmakeBuilder{})
	factory.Register(&CargoBuilder{})

	return factory
}

// Register adds a new builder to the factory
func (f *BuilderFactory) Register(builder Builder) {
	f.builders = append(f.builders, builder)
}

// BuilderFor returns the appropriate builder for the given extension file
func (f *BuilderFactory) BuilderFor(extensionFile string) (Builder, error) {
	filename := filepath.Base(extensionFile)

	for _, builder := range f.builders {
		if builder.CanBuild(filename) {
			return builder, nil
		}
	}

	return nil, fmt.Errorf("no builder found for extension file: %s", filename)
}

// ListBuilders returns all registered builders
func (f *BuilderFactory) ListBuilders() []Builder {
	return append([]Builder{}, f.builders...)
}

// BuildAllExtensions builds all extensions found in the gem specification
func (f *BuilderFactory) BuildAllExtensions(ctx context.Context, config *BuildConfig, extensions []string) ([]*BuildResult, error) {
	if len(extensions) == 0 {
		return nil, nil
	}

	var results []*BuildResult
	var firstError error

	for _, extension := range extensions {
		builder, err := f.BuilderFor(extension)
		if err != nil {
			if firstError == nil {
				firstError = err
			}
			results = append(results, &BuildResult{
				Success: false,
				Error:   err,
			})
			continue
		}

		result, err := builder.Build(ctx, config, extension)
		if err != nil {
			if firstError == nil {
				firstError = err
			}
			if result == nil {
				result = &BuildResult{
					Success: false,
					Error:   err,
				}
			}
		}

		results = append(results, result)

		// Stop on first failure unless configured otherwise
		if !result.Success {
			break
		}
	}

	return results, firstError
}

// Common helper functions for builders

// MatchesPattern checks if a filename matches any of the given regex patterns
func MatchesPattern(filename string, patterns ...string) bool {
	for _, pattern := range patterns {
		if matched, _ := regexp.MatchString(pattern, filename); matched {
			return true
		}
	}
	return false
}

// MatchesExtension checks if a filename has any of the given extensions
func MatchesExtension(filename string, extensions ...string) bool {
	for _, ext := range extensions {
		if strings.HasSuffix(strings.ToLower(filename), strings.ToLower(ext)) {
			return true
		}
	}
	return false
}

// BuildError creates a standardized build error
func BuildError(builder string, output []string, err error) error {
	outputStr := strings.Join(output, "\n")
	if outputStr != "" {
		return fmt.Errorf("%s build failed: %v\n\nBuild output:\n%s", builder, err, outputStr)
	}
	return fmt.Errorf("%s build failed: %v", builder, err)
}

// runCommonBuild executes the standard 3-step build process
func runCommonBuild(ctx context.Context, config *BuildConfig, extensionFile string, steps CommonBuildSteps) (*BuildResult, error) {
	result := &BuildResult{
		Success: false,
		Output:  []string{},
	}

	extensionPath := filepath.Join(config.GemDir, extensionFile)
	extensionDir := filepath.Dir(extensionPath)

	// Step 1: Configure/prepare
	if err := steps.ConfigureFunc(ctx, config, extensionDir, result); err != nil {
		result.Error = err
		return result, err
	}

	// Step 2: Build
	if err := steps.BuildFunc(ctx, config, extensionDir, result); err != nil {
		result.Error = err
		return result, err
	}

	// Step 3: Find built extensions
	extensions, err := steps.FindFunc(extensionDir)
	if err != nil {
		result.Error = err
		return result, err
	}

	result.Extensions = extensions
	result.Success = true
	return result, nil
}

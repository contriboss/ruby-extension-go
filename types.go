package rubyext

import "context"

// BuildResult contains the output and status of a build operation.
//
// After a build completes, this structure provides:
//   - Success status indicating if the build completed without errors
//   - Output lines captured from the build process (stdout/stderr)
//   - Extensions list of compiled extension files (.so/.bundle/.dll)
//   - Error information if the build failed
type BuildResult struct {
	Success             bool     // True if build completed successfully
	Output              []string // Lines of output from the build process
	Extensions          []string // Paths to built extension files
	Error               error    // Error if build failed, nil otherwise
	MissingDependencies []string // Names of build-time dependencies that were missing
}

// BuildConfig contains configuration for the build process.
//
// This structure controls all aspects of the extension build:
//
// Source paths define where files are located:
//   - GemDir: Root directory of the extracted gem
//   - ExtensionDir: Directory containing extension source files
//   - DestPath: Destination directory for compiled extensions
//   - LibDir: Optional lib directory for extension installation
//
// Build configuration:
//   - BuildArgs: Additional arguments passed to the build system
//   - Env: Environment variables set during build
//   - Parallel: Number of parallel jobs for make -j (0 = default)
//
// Ruby environment:
//   - RubyEngine: Ruby implementation (ruby, jruby, truffleruby)
//   - RubyVersion: Ruby version string (e.g., "3.4.0")
//   - RubyPath: Path to Ruby executable
//
// Build behavior:
//   - Verbose: Enable detailed build output
//   - CleanFirst: Run clean target before building
//   - StopOnFailure: Stop after first failed extension (default behavior)
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

	// Failure handling
	StopOnFailure bool // Stop after the first failed extension build
}

// CommonBuildSteps defines the standard 3-step build pattern used by multiple builders.
//
// Many Ruby extension build systems follow a similar pattern:
//  1. Configure: Generate build files (Makefile, etc.)
//  2. Build: Compile the extension
//  3. Find: Locate the compiled extension files
//
// This structure allows builders to implement this pattern consistently
// while customizing each step's behavior.
//
// Example usage in a builder:
//
//	return runCommonBuild(ctx, config, extensionFile, CommonBuildSteps{
//	    ConfigureFunc: b.generateMakefile,
//	    BuildFunc:     b.runCompilation,
//	    FindFunc:      b.locateExtensions,
//	})
type CommonBuildSteps struct {
	// ConfigureFunc prepares the build environment (e.g., run extconf.rb, cmake)
	ConfigureFunc func(ctx context.Context, config *BuildConfig, extensionDir string, result *BuildResult) error

	// BuildFunc compiles the extension (e.g., run make, cargo build)
	BuildFunc func(ctx context.Context, config *BuildConfig, extensionDir string, result *BuildResult) error

	// FindFunc locates the compiled extension files after build completes
	FindFunc func(extensionDir string) ([]string, error)
}

package rubyext

import "context"

// Builder defines the interface that all extension builders must implement.
//
// Each builder is responsible for a specific build system (extconf.rb, CMake, Cargo, etc.)
// and must implement these four methods to integrate with the BuilderFactory.
//
// # Builder Lifecycle
//
//  1. CanBuild() - Factory calls this to find the right builder for an extension file
//  2. Build() - Factory calls this to compile the extension
//  3. Clean() - Optional cleanup of build artifacts
//
// # Example Implementation
//
//	type MyBuilder struct{}
//
//	func (b *MyBuilder) Name() string {
//	    return "MyBuildSystem"
//	}
//
//	func (b *MyBuilder) CanBuild(extensionFile string) bool {
//	    return strings.HasSuffix(extensionFile, "mybuild.conf")
//	}
//
//	func (b *MyBuilder) Build(ctx context.Context, config *BuildConfig, extensionFile string) (*BuildResult, error) {
//	    // Build implementation
//	    result := &BuildResult{Success: true}
//	    // ... build logic ...
//	    return result, nil
//	}
//
//	func (b *MyBuilder) Clean(ctx context.Context, config *BuildConfig, extensionFile string) error {
//	    // Cleanup implementation
//	    return nil
//	}
//
// # Thread Safety
//
// Builder implementations should be stateless and thread-safe.
// The same builder instance may be used to build multiple extensions concurrently.
type Builder interface {
	// Name returns the human-readable name of this builder.
	//
	// This name is used in error messages and logs.
	// Examples: "ExtConf", "CMake", "Cargo"
	Name() string

	// CanBuild checks if this builder can handle the given extension file.
	//
	// The extensionFile parameter is typically just the filename (e.g., "extconf.rb")
	// or a relative path (e.g., "ext/myext/extconf.rb").
	//
	// Returns true if this builder should be used for the file.
	CanBuild(extensionFile string) bool

	// Build compiles the extension and returns the result.
	//
	// This method should:
	//  1. Configure the build (generate Makefile, etc.)
	//  2. Compile the extension
	//  3. Locate the compiled extension files
	//
	// The extensionFile path is relative to config.GemDir.
	//
	// Returns:
	//   - BuildResult with Success=true and Extensions list on success
	//   - BuildResult with Success=false and Error on failure
	Build(ctx context.Context, config *BuildConfig, extensionFile string) (*BuildResult, error)

	// Clean removes build artifacts.
	//
	// This is optional - some builders may not support cleaning.
	// Returns nil if cleaning is not supported or completes successfully.
	//
	// The extensionFile path is relative to config.GemDir.
	Clean(ctx context.Context, config *BuildConfig, extensionFile string) error
}

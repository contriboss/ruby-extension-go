package rubyext

import (
	"context"
	"fmt"
	"path/filepath"
)

// BuilderFactory manages the registration and selection of extension builders.
//
// The factory maintains a registry of Builder implementations and provides
// methods to:
//   - Register new builders
//   - Find the appropriate builder for an extension file
//   - Build multiple extensions in sequence
//
// # Usage
//
// Create a factory with all standard builders:
//
//	factory := rubyext.NewBuilderFactory()
//
// Or create an empty factory and register custom builders:
//
//	factory := &rubyext.BuilderFactory{}
//	factory.Register(&MyCustomBuilder{})
//
// Then use it to build extensions:
//
//	results, err := factory.BuildAllExtensions(ctx, config, extensions)
//
// # Builder Selection
//
// When building an extension, the factory:
//  1. Extracts the filename from the extension path
//  2. Calls CanBuild() on each registered builder in order
//  3. Uses the first builder that returns true
//  4. Returns an error if no builder can handle the file
//
// # Thread Safety
//
// BuilderFactory is NOT thread-safe for registration.
// Register all builders before concurrent use.
// After registration, Read operations (BuilderFor, BuildAllExtensions) are safe.
type BuilderFactory struct {
	builders []Builder
}

// NewBuilderFactory creates a factory with all standard builders registered.
//
// The standard builders are registered in this order:
//  1. ExtConfBuilder - extconf.rb files
//  2. ConfigureBuilder - configure scripts
//  3. RakeBuilder - Rakefile and mkrf_conf.rb
//  4. CmakeBuilder - CMakeLists.txt
//  5. CargoBuilder - Cargo.toml
//
// This is the recommended way to create a BuilderFactory for most use cases.
func NewBuilderFactory() *BuilderFactory {
	factory := &BuilderFactory{}

	// Register all standard builders in priority order
	factory.Register(&ExtConfBuilder{})
	factory.Register(&ConfigureBuilder{})
	factory.Register(&RakeBuilder{})
	factory.Register(&CmakeBuilder{})
	factory.Register(&CargoBuilder{})

	return factory
}

// Register adds a new builder to the factory.
//
// Builders are checked in the order they are registered.
// If multiple builders can handle the same file type, the first
// registered builder will be used.
//
// Not thread-safe. Register all builders before concurrent use.
func (f *BuilderFactory) Register(builder Builder) {
	f.builders = append(f.builders, builder)
}

// BuilderFor returns the appropriate builder for the given extension file.
//
// The extensionFile can be a full path (e.g., "ext/myext/extconf.rb")
// or just a filename (e.g., "extconf.rb"). Only the base filename
// is used for matching.
//
// Returns the first builder whose CanBuild() method returns true,
// or an error if no builder can handle the file.
func (f *BuilderFactory) BuilderFor(extensionFile string) (Builder, error) {
	filename := filepath.Base(extensionFile)

	for _, builder := range f.builders {
		if builder.CanBuild(filename) {
			return builder, nil
		}
	}

	return nil, fmt.Errorf("no builder found for extension file: %s", filename)
}

// ListBuilders returns a copy of all registered builders.
//
// The returned slice is a copy and can be modified without affecting
// the factory's internal state.
func (f *BuilderFactory) ListBuilders() []Builder {
	return append([]Builder{}, f.builders...)
}

// BuildAllExtensions builds all extensions in sequence.
//
// This method processes each extension in order:
//  1. Check for context cancellation
//  2. Find the appropriate builder
//  3. Build the extension
//  4. Collect the result
//  5. Stop on first failure if config.StopOnFailure is true
//
// # Return Values
//
// Returns:
//   - A slice of BuildResult, one for each extension processed
//   - The first error encountered (if any)
//
// Even if an error is returned, the results slice will contain
// partial results for extensions that were processed.
//
// # Error Handling
//
// If config.StopOnFailure is true (default):
//   - Processing stops after the first failed extension
//   - Results slice contains results up to and including the failure
//
// If config.StopOnFailure is false:
//   - All extensions are processed regardless of failures
//   - Results slice contains results for all extensions
//   - The first error encountered is returned
//
// # Context Cancellation
//
// If the context is canceled during processing:
//   - Processing stops immediately
//   - A BuildResult with context.Canceled error is added
//   - The context error is returned
func (f *BuilderFactory) BuildAllExtensions(ctx context.Context, config *BuildConfig, extensions []string) ([]*BuildResult, error) {
	if len(extensions) == 0 {
		return nil, nil
	}

	var results []*BuildResult
	var firstError error

	for _, extension := range extensions {
		// Check for context cancellation
		if ctxErr := ctx.Err(); ctxErr != nil {
			if firstError == nil {
				firstError = ctxErr
			}
			results = append(results, &BuildResult{
				Success: false,
				Error:   ctxErr,
			})
			break
		}

		// Find appropriate builder
		builder, err := f.BuilderFor(extension)
		if err != nil {
			if firstError == nil {
				firstError = err
			}
			results = append(results, &BuildResult{
				Success: false,
				Error:   err,
			})
			if config.StopOnFailure {
				break
			}
			continue
		}

		// Build the extension
		result, err := builder.Build(ctx, config, extension)
		if err != nil {
			if firstError == nil {
				firstError = err
			}
			// Ensure we have a result even if builder didn't return one
			if result == nil {
				result = &BuildResult{
					Success: false,
					Error:   err,
				}
			}
		}

		results = append(results, result)

		// Stop on first failure if configured
		if !result.Success && config.StopOnFailure {
			break
		}
	}

	return results, firstError
}

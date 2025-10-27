package rubyext

import (
	"context"
	"path/filepath"
)

// runCommonBuild executes the standard 3-step build process.
//
// Many Ruby extension build systems follow a similar pattern:
//  1. Configure: Generate build files (Makefile, build.ninja, etc.)
//  2. Build: Compile the extension using the generated files
//  3. Find: Locate the compiled extension files (.so, .bundle, .dll)
//
// This function provides a consistent way to execute this pattern,
// allowing builders to focus on implementing their specific logic
// for each step.
//
// # Process Flow
//
//  1. Create empty BuildResult
//  2. Calculate extension directory from extensionFile path
//  3. Call ConfigureFunc to prepare the build
//  4. Call BuildFunc to compile the extension
//  5. Call FindFunc to locate compiled files
//  6. Return BuildResult with Success=true
//
// If any step fails, processing stops and the error is returned
// with Success=false.
//
// # Parameters
//
//   - ctx: Context for cancellation
//   - config: Build configuration
//   - extensionFile: Path to extension file (relative to config.GemDir)
//   - steps: The three functions to execute
//
// # Returns
//
// Returns:
//   - BuildResult with Success=true and Extensions list on success
//   - BuildResult with Success=false and Error on failure
//
// The BuildResult.Output field is populated by the step functions
// as they execute.
//
// # Example
//
// Typical usage in a builder:
//
//	func (b *MyBuilder) Build(ctx context.Context, config *BuildConfig, extensionFile string) (*BuildResult, error) {
//	    return runCommonBuild(ctx, config, extensionFile, CommonBuildSteps{
//	        ConfigureFunc: func(ctx context.Context, config *BuildConfig, extensionDir string, result *BuildResult) error {
//	            // Run ./configure or generate Makefile
//	            cmd := exec.CommandContext(ctx, "./configure")
//	            cmd.Dir = extensionDir
//	            output, err := cmd.CombinedOutput()
//	            result.Output = append(result.Output, string(output))
//	            return err
//	        },
//	        BuildFunc: func(ctx context.Context, config *BuildConfig, extensionDir string, result *BuildResult) error {
//	            // Run make
//	            cmd := exec.CommandContext(ctx, "make")
//	            cmd.Dir = extensionDir
//	            output, err := cmd.CombinedOutput()
//	            result.Output = append(result.Output, string(output))
//	            return err
//	        },
//	        FindFunc: func(extensionDir string) ([]string, error) {
//	            // Find *.so files
//	            return filepath.Glob(filepath.Join(extensionDir, "*.so"))
//	        },
//	    })
//	}
//
// # Error Handling
//
// If any step returns an error:
//   - result.Error is set to the error
//   - result.Success remains false
//   - The BuildResult and error are returned
//   - Subsequent steps are not executed
//
// # Thread Safety
//
// This function is thread-safe as long as the provided step functions
// are thread-safe and don't share mutable state.
func runCommonBuild(ctx context.Context, config *BuildConfig, extensionFile string, steps CommonBuildSteps) (*BuildResult, error) {
	result := &BuildResult{
		Success: false,
		Output:  []string{},
	}

	// Calculate extension directory
	extensionPath := filepath.Join(config.GemDir, extensionFile)
	extensionDir := filepath.Dir(extensionPath)

	// Step 1: Configure/prepare the build
	if err := steps.ConfigureFunc(ctx, config, extensionDir, result); err != nil {
		result.Error = err
		return result, err
	}

	// Step 2: Build/compile the extension
	if err := steps.BuildFunc(ctx, config, extensionDir, result); err != nil {
		result.Error = err
		return result, err
	}

	// Step 3: Find the built extension files
	extensions, err := steps.FindFunc(extensionDir)
	if err != nil {
		result.Error = err
		return result, err
	}

	// Success!
	result.Extensions = extensions
	result.Success = true
	return result, nil
}

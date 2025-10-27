package rubyext

import (
	"fmt"
	"regexp"
	"strings"
)

// MatchesPattern checks if a filename matches any of the given regex patterns.
//
// This is a helper function for builder implementations to determine if they
// can handle a given extension file based on filename patterns.
//
// # Parameters
//
//   - filename: The file to check (typically just the base name)
//   - patterns: One or more regex patterns to match against
//
// # Returns
//
// Returns true if the filename matches any pattern, false otherwise.
// If a pattern is invalid regex, it is silently skipped.
//
// # Example
//
//	// Check if file is extconf.rb
//	if MatchesPattern(filename, `extconf\.rb$`) {
//	    // Handle extconf.rb
//	}
//
//	// Check for configure or configure.sh
//	if MatchesPattern(filename, `^configure$`, `^configure\.sh$`) {
//	    // Handle configure scripts
//	}
//
// # Thread Safety
//
// This function is thread-safe and can be called concurrently.
func MatchesPattern(filename string, patterns ...string) bool {
	for _, pattern := range patterns {
		if matched, _ := regexp.MatchString(pattern, filename); matched {
			return true
		}
	}
	return false
}

// MatchesExtension checks if a filename has any of the given extensions.
//
// This is a case-insensitive check for file extensions.
// Useful for checking compiled extension files (.so, .bundle, .dll).
//
// # Parameters
//
//   - filename: The file to check
//   - extensions: One or more extensions to check (with or without leading dot)
//
// # Returns
//
// Returns true if the filename ends with any of the extensions (case-insensitive).
//
// # Example
//
//	// Check for compiled extensions
//	if MatchesExtension(filename, ".so", ".bundle", ".dll") {
//	    // This is a compiled extension
//	}
//
//	// Works with or without leading dot
//	if MatchesExtension(filename, "rb", ".rb") {
//	    // This is a Ruby file
//	}
//
// # Thread Safety
//
// This function is thread-safe and can be called concurrently.
func MatchesExtension(filename string, extensions ...string) bool {
	for _, ext := range extensions {
		if strings.HasSuffix(strings.ToLower(filename), strings.ToLower(ext)) {
			return true
		}
	}
	return false
}

// BuildError creates a standardized build error with output context.
//
// This helper formats build errors consistently across all builders,
// including the build output for debugging.
//
// # Parameters
//
//   - builder: Name of the builder (e.g., "ExtConf", "CMake")
//   - output: Lines of output from the build process
//   - err: The underlying error (can be nil)
//
// # Returns
//
// A formatted error message containing:
//   - The builder name
//   - The underlying error message (if provided)
//   - The full build output (if available)
//
// # Format
//
// With error and output:
//
//	ExtConf build failed: make: *** [target] Error 1
//
//	Build output:
//	gcc -o extension.o -c extension.c
//	gcc: error: invalid option
//
// With error but no output:
//
//	ExtConf build failed: make: *** [target] Error 1
//
// With output but no error:
//
//	ExtConf build failed
//
//	Build output:
//	... output lines ...
//
// # Example
//
//	output := []string{"Building...", "Error: compilation failed"}
//	err := BuildError("ExtConf", output, fmt.Errorf("make exited with code 2"))
//	// Returns formatted error with full context
//
// # Thread Safety
//
// This function is thread-safe and can be called concurrently.
func BuildError(builder string, output []string, err error) error {
	outputStr := strings.Join(output, "\n")

	var prefix string
	if err != nil {
		prefix = fmt.Sprintf("%s build failed: %v", builder, err)
	} else {
		prefix = fmt.Sprintf("%s build failed", builder)
	}

	if outputStr != "" {
		return fmt.Errorf("%s\n\nBuild output:\n%s", prefix, outputStr)
	}

	return fmt.Errorf("%s", prefix)
}

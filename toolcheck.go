package rubyext

import (
	"fmt"
	"os/exec"
	"strings"
)

// ToolChecker is an optional interface for builders that require external tools.
//
// Builders can implement this interface to declare their tool dependencies
// and verify that required tools are available before attempting to build.
//
// This is an opt-in interface - builders that don't implement it will
// work exactly as before, maintaining backward compatibility.
//
// # Platform Support
//
// Tool alternatives handle platform differences:
//   - FreeBSD: Uses gmake instead of make, clang instead of gcc
//   - Windows: Uses cl (MSVC) instead of gcc, nmake instead of make
//   - macOS: Uses clang by default
//   - Linux: Uses gcc/make by default
//
// # Example Implementation
//
//	func (b *CmakeBuilder) RequiredTools() []ToolRequirement {
//	    return []ToolRequirement{
//	        {Name: "cmake", Purpose: "CMake build system"},
//	        {Name: "ninja", Optional: true, Purpose: "Ninja build tool (faster than make)"},
//	    }
//	}
//
//	func (b *CmakeBuilder) CheckTools() error {
//	    return CheckRequiredTools(b.RequiredTools())
//	}
//
// # Consumer Usage
//
// Check tools before building:
//
//	if checker, ok := builder.(ToolChecker); ok {
//	    if err := checker.CheckTools(); err != nil {
//	        return fmt.Errorf("build tools missing: %w", err)
//	    }
//	}
//
// # Thread Safety
//
// Implementations should be thread-safe as they may be called concurrently.
type ToolChecker interface {
	// RequiredTools returns the list of tools this builder needs.
	//
	// Returns a slice of ToolRequirement describing each required tool,
	// including optional tools and alternatives.
	RequiredTools() []ToolRequirement

	// CheckTools verifies that all required tools are available.
	//
	// Returns nil if all required tools are found, or an error describing
	// which tools are missing. Optional tools don't cause errors if missing.
	//
	// This method can be called before Build() to fail fast if tools
	// are unavailable, providing better error messages to users.
	CheckTools() error
}

// ToolRequirement describes a build tool dependency.
//
// This structure allows builders to declare:
//   - Required tools (must be available)
//   - Optional tools (nice to have, but not required)
//   - Alternative tools (any one of several tools can satisfy the requirement)
//
// # Examples
//
// Required tool:
//
//	ToolRequirement{
//	    Name: "cmake",
//	    Purpose: "CMake build system",
//	}
//
// Optional tool:
//
//	ToolRequirement{
//	    Name: "ninja",
//	    Optional: true,
//	    Purpose: "Faster build than make",
//	}
//
// Tool with alternatives:
//
//	ToolRequirement{
//	    Name: "gcc",
//	    Alternatives: []string{"clang", "cc"},
//	    Purpose: "C compiler",
//	}
type ToolRequirement struct {
	// Name is the primary tool binary name (e.g., "cmake", "cargo").
	Name string

	// Alternatives are alternative tool names that can satisfy this requirement.
	// If any tool in Alternatives is found, the requirement is satisfied.
	// Example: []string{"gcc", "clang", "cc"}
	Alternatives []string

	// Optional indicates this tool is optional and won't cause an error if missing.
	// Optional tools are still checked and logged, but don't fail the build.
	Optional bool

	// Purpose is a human-readable description of why this tool is needed.
	// Example: "CMake build system" or "Rust compiler and package manager"
	Purpose string
}

// CheckToolAvailable checks if a tool is available in the system PATH.
//
// This is a simple wrapper around exec.LookPath that provides
// consistent error messages.
//
// # Parameters
//
//   - tool: The tool binary name to check (e.g., "cmake", "cargo")
//
// # Returns
//
// Returns nil if the tool is found in PATH, or an error if not found.
// The error message includes the tool name for easy identification.
//
// # Example
//
//	if err := CheckToolAvailable("cmake"); err != nil {
//	    return fmt.Errorf("cmake is required: %w", err)
//	}
//
// # Thread Safety
//
// This function is thread-safe and can be called concurrently.
func CheckToolAvailable(tool string) error {
	_, err := exec.LookPath(tool)
	if err != nil {
		return fmt.Errorf("%s not found in PATH", tool)
	}
	return nil
}

// CheckRequiredTools verifies all required tools are available.
//
// This helper function checks a list of ToolRequirements and returns
// a detailed error if any required tools are missing.
//
// # Behavior
//
//   - Checks the primary tool name first
//   - If not found, tries each alternative tool in order
//   - Optional tools are checked but don't cause errors
//   - Returns all missing required tools in a single error
//
// # Parameters
//
//   - requirements: List of tools to check
//
// # Returns
//
// Returns nil if all required tools are available.
// Returns an error listing all missing required tools if any are not found.
//
// # Example
//
//	requirements := []ToolRequirement{
//	    {Name: "cmake", Purpose: "CMake build system"},
//	    {Name: "ninja", Optional: true},
//	}
//	if err := CheckRequiredTools(requirements); err != nil {
//	    return fmt.Errorf("missing tools: %w", err)
//	}
//
// # Error Format
//
// Single missing tool:
//
//	cmake not found in PATH (required for: CMake build system)
//
// Multiple missing tools:
//
//	missing required tools: cmake (CMake build system), cargo (Rust compiler)
//
// # Thread Safety
//
// This function is thread-safe and can be called concurrently.
func CheckRequiredTools(requirements []ToolRequirement) error {
	var missingTools []string

	for _, req := range requirements {
		// Try the primary tool
		found := CheckToolAvailable(req.Name) == nil

		// If not found, try alternatives
		if !found && len(req.Alternatives) > 0 {
			for _, alt := range req.Alternatives {
				if CheckToolAvailable(alt) == nil {
					found = true
					break
				}
			}
		}

		// If still not found and not optional, record it
		if !found && !req.Optional {
			if req.Purpose != "" {
				missingTools = append(missingTools, fmt.Sprintf("%s (%s)", req.Name, req.Purpose))
			} else {
				missingTools = append(missingTools, req.Name)
			}
		}
	}

	if len(missingTools) == 0 {
		return nil
	}

	if len(missingTools) == 1 {
		return fmt.Errorf("%s not found in PATH", missingTools[0])
	}

	return fmt.Errorf("missing required tools: %s", strings.Join(missingTools, ", "))
}

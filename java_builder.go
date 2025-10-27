package rubyext

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	pomXMLFile = "pom.xml"
)

// JavaBuilder handles Java-based builds for JRuby extensions.
//
// This builder compiles Java source code into .jar files or .class files
// that can be loaded by JRuby. It supports both direct Java compilation
// and Maven-based builds.
//
// Common use cases:
//   - JRuby native extensions written in Java
//   - Java libraries wrapped for Ruby use
//   - Performance-critical code using the JVM
//
// Supported build files:
//   - *.java - Direct Java source compilation
//   - pom.xml - Maven-based build
type JavaBuilder struct{}

// Name returns the builder name
func (b *JavaBuilder) Name() string {
	return "Java"
}

// RequiredTools returns the tools needed for Java builds
func (b *JavaBuilder) RequiredTools() []ToolRequirement {
	return []ToolRequirement{
		{
			Name:    "javac",
			Purpose: "Java compiler",
		},
		{
			Name:     "mvn",
			Optional: true,
			Purpose:  "Maven build tool (for pom.xml projects)",
		},
	}
}

// CheckTools verifies that Java toolchain is available
func (b *JavaBuilder) CheckTools() error {
	return CheckRequiredTools(b.RequiredTools())
}

// CanBuild checks if this builder can handle the extension file
func (b *JavaBuilder) CanBuild(extensionFile string) bool {
	ext := strings.ToLower(filepath.Ext(extensionFile))
	base := strings.ToLower(filepath.Base(extensionFile))
	return ext == ".java" || base == pomXMLFile
}

// Build compiles the Java extension
func (b *JavaBuilder) Build(ctx context.Context, config *BuildConfig, extensionFile string) (*BuildResult, error) {
	// Check if this is a Maven project
	if strings.ToLower(filepath.Base(extensionFile)) == pomXMLFile {
		return runCommonBuild(ctx, config, extensionFile, CommonBuildSteps{
			ConfigureFunc: b.noConfigure,
			BuildFunc:     b.runMavenBuild,
			FindFunc:      b.findBuiltExtensions,
		})
	}

	// Otherwise, direct Java compilation
	return runCommonBuild(ctx, config, extensionFile, CommonBuildSteps{
		ConfigureFunc: b.noConfigure,
		BuildFunc:     b.runJavacBuild,
		FindFunc:      b.findBuiltExtensions,
	})
}

// Clean removes build artifacts
func (b *JavaBuilder) Clean(ctx context.Context, config *BuildConfig, extensionFile string) error {
	extensionPath := filepath.Join(config.GemDir, extensionFile)
	extensionDir := filepath.Dir(extensionPath)

	// If Maven project, use mvn clean
	if strings.ToLower(filepath.Base(extensionFile)) == "pom.xml" {
		cleanCmd := exec.CommandContext(ctx, "mvn", "clean")
		cleanCmd.Dir = extensionDir
		_ = cleanCmd.Run()
		return nil
	}

	// Otherwise, just remove .class and .jar files
	patterns := []string{"*.class", "*.jar"}
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(filepath.Join(extensionDir, pattern))
		for _, match := range matches {
			_ = os.Remove(match)
		}
	}

	return nil
}

// noConfigure is a no-op since Java doesn't need configuration
func (b *JavaBuilder) noConfigure(ctx context.Context, config *BuildConfig, extensionDir string, result *BuildResult) error {
	if config.Verbose {
		result.Output = append(result.Output, "Java project, no configuration needed")
	}
	return nil
}

// runMavenBuild executes mvn package for Maven projects
func (b *JavaBuilder) runMavenBuild(ctx context.Context, config *BuildConfig, extensionDir string, result *BuildResult) error {
	args := []string{"package"}

	// Add any additional build args
	args = append(args, config.BuildArgs...)

	// Run mvn package
	cmd := exec.CommandContext(ctx, "mvn", args...)
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
			fmt.Sprintf("Running: mvn %s", strings.Join(args, " ")),
			fmt.Sprintf("Working directory: %s", extensionDir))
	}

	if err != nil {
		return BuildError("Maven", result.Output, err)
	}

	return nil
}

// runJavacBuild executes javac for direct Java compilation
func (b *JavaBuilder) runJavacBuild(ctx context.Context, config *BuildConfig, extensionDir string, result *BuildResult) error {
	// Find all .java files in the directory
	javaFiles, err := filepath.Glob(filepath.Join(extensionDir, "*.java"))
	if err != nil || len(javaFiles) == 0 {
		return fmt.Errorf("no Java source files found in %s", extensionDir)
	}

	// Build javac arguments
	args := []string{"-d", extensionDir}
	args = append(args, config.BuildArgs...)

	// Add all Java files
	for _, javaFile := range javaFiles {
		args = append(args, filepath.Base(javaFile))
	}

	// Run javac
	cmd := exec.CommandContext(ctx, "javac", args...)
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
			fmt.Sprintf("Running: javac %s", strings.Join(args, " ")),
			fmt.Sprintf("Working directory: %s", extensionDir))
	}

	if err != nil {
		return BuildError("Javac", result.Output, err)
	}

	// Create a JAR file from the compiled classes
	jarName := "extension.jar"
	if config.DestPath != "" {
		jarName = filepath.Join(config.DestPath, jarName)
	}

	jarCmd := exec.CommandContext(ctx, "jar", "cf", jarName, "-C", extensionDir, ".")
	jarOutput, jarErr := jarCmd.CombinedOutput()
	result.Output = append(result.Output, strings.Split(string(jarOutput), "\n")...)

	if jarErr != nil {
		return BuildError("Jar", result.Output, jarErr)
	}

	return nil
}

// findBuiltExtensions locates the compiled .jar and .class files
func (b *JavaBuilder) findBuiltExtensions(extensionDir string) ([]string, error) {
	var extensions []string

	// Look for JAR files (Maven produces these in target/)
	patterns := []string{
		"*.jar",
		"target/*.jar",
	}

	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(extensionDir, pattern))
		if err != nil {
			return nil, fmt.Errorf("failed to glob pattern %s in %s: %v", pattern, extensionDir, err)
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

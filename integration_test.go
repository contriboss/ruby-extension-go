package rubyext

import (
	"os"
	"path/filepath"
	"testing"
)

// Builder name constants
const (
	extConfBuilderName = "ExtConf"
	rubyEngineCommand  = "ruby"
)

func TestPgExtensionDetection(t *testing.T) {
	// Skip if pg gem is not available
	pgPath := filepath.Join("..", "..", "vendor-test", "ruby", "3.4.0", "gems", "pg-1.5.9")
	if _, err := os.Stat(pgPath); os.IsNotExist(err) {
		t.Skip("pg gem not found, skipping integration test")
	}

	factory := NewBuilderFactory()

	// Test detection of pg's extconf.rb
	extconfPath := filepath.Join(pgPath, "ext", "extconf.rb")
	if _, err := os.Stat(extconfPath); os.IsNotExist(err) {
		t.Skip("pg extconf.rb not found")
	}

	builder, err := factory.BuilderFor("ext/extconf.rb")
	if err != nil {
		t.Fatalf("Expected builder for extconf.rb, got error: %v", err)
	}

	if builder.Name() != extConfBuilderName {
		t.Errorf("Expected ExtConf builder, got %s", builder.Name())
	}

	// Verify the builder can detect the file
	if !builder.CanBuild("extconf.rb") {
		t.Error("ExtConf builder should be able to build extconf.rb")
	}
}

func TestBuiltPgExtension(t *testing.T) {
	// Check if pg extension was already built
	pgExtPath := filepath.Join("..", "..", "vendor-test", "ruby", "3.4.0", "gems", "pg-1.5.9", "ext", "pg_ext.bundle")
	if _, err := os.Stat(pgExtPath); os.IsNotExist(err) {
		t.Skip("pg_ext.bundle not found, extension not compiled")
	}

	t.Logf("Found compiled pg extension at: %s", pgExtPath)

	// Test that our extension finder would detect this
	extConfBuilder := &ExtConfBuilder{}
	extensionDir := filepath.Dir(pgExtPath)

	extensions, err := extConfBuilder.findBuiltExtensions(extensionDir)
	if err != nil {
		t.Fatalf("Failed to find built extensions: %v", err)
	}

	if len(extensions) == 0 {
		t.Error("Expected to find built extensions in pg directory")
	}

	found := false
	for _, ext := range extensions {
		if filepath.Base(ext) == "pg_ext.bundle" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected to find pg_ext.bundle in extensions list: %v", extensions)
	}
}

func TestExtensionBuildConfig(t *testing.T) {
	config := &BuildConfig{
		GemDir:       "/Users/seuros/Projects/ore/vendor-test/ruby/3.4.0/gems/pg-1.5.9",
		ExtensionDir: "/Users/seuros/Projects/ore/vendor-test/ruby/3.4.0/gems/pg-1.5.9/ext",
		DestPath:     "/tmp/test-dest",
		RubyEngine:   rubyEngineCommand,
		RubyVersion:  "3.4.0",
		RubyPath:     "/usr/bin/ruby",
		Verbose:      true,
		Parallel:     2,
		BuildArgs:    []string{"--with-pg-config=/opt/homebrew/bin/pg_config"},
	}

	// Test configuration is properly set
	if config.RubyEngine != rubyEngineCommand {
		t.Errorf("Expected RubyEngine 'ruby', got '%s'", config.RubyEngine)
	}

	if config.Parallel != 2 {
		t.Errorf("Expected Parallel 2, got %d", config.Parallel)
	}

	if len(config.BuildArgs) != 1 {
		t.Errorf("Expected 1 build arg, got %d", len(config.BuildArgs))
	}
}

// This test demonstrates how the extension building would work in practice
func TestExtensionBuildWorkflow(t *testing.T) {
	factory := NewBuilderFactory()

	// Simulate finding extensions in a gem
	extensions := []string{
		"ext/extconf.rb",     // Most common (pg, nokogiri, etc.)
		"ext/configure",      // Autotools
		"ext/Rakefile",       // Ruby-based
		"ext/CMakeLists.txt", // CMake
		"ext/Cargo.toml",     // Rust
	}

	for _, extension := range extensions {
		t.Run(extension, func(t *testing.T) {
			builder, err := factory.BuilderFor(extension)
			if err != nil {
				t.Fatalf("Failed to find builder for %s: %v", extension, err)
			}

			t.Logf("Found %s builder for %s", builder.Name(), extension)

			// Verify the builder can handle this extension
			if !builder.CanBuild(filepath.Base(extension)) {
				t.Errorf("Builder %s claims it cannot build %s", builder.Name(), extension)
			}
		})
	}
}

// Test builder priority - first match wins
func TestBuilderPriority(t *testing.T) {
	factory := NewBuilderFactory()

	// Test that extconf.rb takes precedence
	builder, err := factory.BuilderFor("ext/extconf.rb")
	if err != nil {
		t.Fatalf("Failed to find builder: %v", err)
	}

	if builder.Name() != extConfBuilderName {
		t.Errorf("Expected ExtConf builder for extconf.rb, got %s", builder.Name())
	}
}

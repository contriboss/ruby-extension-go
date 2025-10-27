package rubyext

import (
	"context"
	"errors"
	"testing"
)

const (
	failingExtension   = "first.ext"
	secondaryExtension = "second.ext"
)

func TestBuilderFactory(t *testing.T) {
	factory := NewBuilderFactory()

	// Test that all expected builders are registered
	builders := factory.ListBuilders()
	expectedCount := 11 // 5 original + 3 new specific + 3 generic language builders
	if len(builders) != expectedCount {
		t.Errorf("Expected %d builders, got %d", expectedCount, len(builders))
	}

	// Test builder detection for each type
	testCases := []struct {
		extensionFile string
		expectedName  string
	}{
		{"ext/extconf.rb", "ExtConf"},
		{"ext/configure", "Configure"},
		{"ext/configure.sh", "Configure"},
		{"ext/Rakefile", "Rake"},
		{"ext/rakefile.rb", "Rake"},
		{"ext/mkrf_conf.rb", "Rake"},
		{"ext/CMakeLists.txt", "CMake"},
		{"ext/Cargo.toml", "Cargo"},
	}

	for _, tc := range testCases {
		t.Run(tc.extensionFile, func(t *testing.T) {
			builder, err := factory.BuilderFor(tc.extensionFile)
			if err != nil {
				t.Fatalf("Expected builder for %s, got error: %v", tc.extensionFile, err)
			}

			if builder.Name() != tc.expectedName {
				t.Errorf("Expected builder %s for %s, got %s", tc.expectedName, tc.extensionFile, builder.Name())
			}
		})
	}

	// Test unsupported extension
	_, err := factory.BuilderFor("unknown.file")
	if err == nil {
		t.Error("Expected error for unsupported extension file")
	}
}

func TestBuilderDetection(t *testing.T) {
	testCases := []struct {
		name         string
		builder      Builder
		validFiles   []string
		invalidFiles []string
	}{
		{
			name:    "ExtConfBuilder",
			builder: &ExtConfBuilder{},
			validFiles: []string{
				"extconf.rb",
				"ext/extconf.rb",
				"path/to/extconf.rb",
			},
			invalidFiles: []string{
				"configure",
				"Rakefile",
				"CMakeLists.txt",
				"Cargo.toml",
				"other.rb",
			},
		},
		{
			name:    "ConfigureBuilder",
			builder: &ConfigureBuilder{},
			validFiles: []string{
				"configure",
				"configure.sh",
				"ext/configure",
			},
			invalidFiles: []string{
				"extconf.rb",
				"Rakefile",
				"configure.in",
				"CMakeLists.txt",
			},
		},
		{
			name:    "RakeBuilder",
			builder: &RakeBuilder{},
			validFiles: []string{
				"Rakefile",
				"rakefile",
				"Rakefile.rb",
				"mkrf_conf",
				"mkrf_conf.rb",
			},
			invalidFiles: []string{
				"extconf.rb",
				"configure",
				"CMakeLists.txt",
				"Cargo.toml",
			},
		},
		{
			name:    "CmakeBuilder",
			builder: &CmakeBuilder{},
			validFiles: []string{
				"CMakeLists.txt",
				"ext/CMakeLists.txt",
			},
			invalidFiles: []string{
				"extconf.rb",
				"configure",
				"Rakefile",
				"Cargo.toml",
				"cmake.txt",
			},
		},
		{
			name:    "CargoBuilder",
			builder: &CargoBuilder{},
			validFiles: []string{
				"Cargo.toml",
				"ext/Cargo.toml",
			},
			invalidFiles: []string{
				"extconf.rb",
				"configure",
				"Rakefile",
				"CMakeLists.txt",
				"cargo.toml",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test valid files
			for _, file := range tc.validFiles {
				if !tc.builder.CanBuild(file) {
					t.Errorf("%s should be able to build %s", tc.name, file)
				}
			}

			// Test invalid files
			for _, file := range tc.invalidFiles {
				if tc.builder.CanBuild(file) {
					t.Errorf("%s should not be able to build %s", tc.name, file)
				}
			}
		})
	}
}

func TestMatchesPattern(t *testing.T) {
	testCases := []struct {
		filename string
		patterns []string
		expected bool
	}{
		{"extconf.rb", []string{"extconf\\.rb$"}, true},
		{"configure", []string{"configure$"}, true},
		{"configure.sh", []string{"configure$", "configure\\.sh$"}, true},
		{"Rakefile", []string{"rakefile", "Rakefile"}, true},
		{"CMakeLists.txt", []string{"CMakeLists\\.txt$"}, true},
		{"Cargo.toml", []string{"Cargo\\.toml$"}, true},
		{"unknown.file", []string{"extconf\\.rb$", "configure$"}, false},
	}

	for _, tc := range testCases {
		t.Run(tc.filename, func(t *testing.T) {
			result := MatchesPattern(tc.filename, tc.patterns...)
			if result != tc.expected {
				t.Errorf("MatchesPattern(%s, %v) = %v, expected %v",
					tc.filename, tc.patterns, result, tc.expected)
			}
		})
	}
}

func TestMatchesExtension(t *testing.T) {
	testCases := []struct {
		filename   string
		extensions []string
		expected   bool
	}{
		{"file.rb", []string{".rb"}, true},
		{"file.RB", []string{".rb"}, true},
		{"file.txt", []string{".rb", ".txt"}, true},
		{"file.py", []string{".rb", ".txt"}, false},
		{"noext", []string{".rb"}, false},
	}

	for _, tc := range testCases {
		t.Run(tc.filename, func(t *testing.T) {
			result := MatchesExtension(tc.filename, tc.extensions...)
			if result != tc.expected {
				t.Errorf("MatchesExtension(%s, %v) = %v, expected %v",
					tc.filename, tc.extensions, result, tc.expected)
			}
		})
	}
}

func TestBuildError(t *testing.T) {
	output := []string{"line 1", "line 2", "error occurred"}
	err := BuildError("TestBuilder", output, nil)

	expected := "TestBuilder build failed\n\nBuild output:\nline 1\nline 2\nerror occurred"
	if err.Error() != expected {
		t.Errorf("BuildError output mismatch.\nExpected: %s\nGot: %s", expected, err.Error())
	}
}

func TestBuildConfig(t *testing.T) {
	config := &BuildConfig{
		GemDir:       "/path/to/gem",
		ExtensionDir: "/path/to/gem/ext",
		DestPath:     "/path/to/dest",
		RubyEngine:   "ruby",
		RubyVersion:  "3.4.0",
		RubyPath:     "/usr/bin/ruby",
		Verbose:      true,
		Parallel:     4,
	}

	// Test that configuration values are properly set
	if config.GemDir != "/path/to/gem" {
		t.Errorf("Expected GemDir '/path/to/gem', got '%s'", config.GemDir)
	}

	if config.Parallel != 4 {
		t.Errorf("Expected Parallel 4, got %d", config.Parallel)
	}

	if !config.Verbose {
		t.Error("Expected Verbose to be true")
	}
}

func TestBuildAllExtensions(t *testing.T) {
	factory := NewBuilderFactory()

	config := &BuildConfig{
		GemDir:      "/tmp/test",
		RubyEngine:  "ruby",
		RubyVersion: "3.4.0",
	}

	ctx := context.Background()

	// Test with no extensions
	results, err := factory.BuildAllExtensions(ctx, config, nil)
	if err != nil {
		t.Errorf("Expected no error for empty extensions, got %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results for empty extensions, got %d", len(results))
	}

	// Test with unknown extension
	results, err = factory.BuildAllExtensions(ctx, config, []string{"unknown.file"})
	if err == nil {
		t.Error("Expected error for unknown extension")
	}
	if len(results) != 1 || results[0].Success {
		t.Error("Expected 1 failed result for unknown extension")
	}
}

type mockBuilder struct {
	name       string
	canBuildFn func(string) bool
	buildFn    func(context.Context, *BuildConfig, string) (*BuildResult, error)
	cleanFn    func(context.Context, *BuildConfig, string) error
	buildCalls int
}

func (m *mockBuilder) Name() string { return m.name }

func (m *mockBuilder) CanBuild(extensionFile string) bool {
	if m.canBuildFn == nil {
		return false
	}
	return m.canBuildFn(extensionFile)
}

func (m *mockBuilder) Build(ctx context.Context, config *BuildConfig, extensionFile string) (*BuildResult, error) {
	m.buildCalls++
	if m.buildFn == nil {
		return &BuildResult{Success: true}, nil
	}
	return m.buildFn(ctx, config, extensionFile)
}

func (m *mockBuilder) Clean(ctx context.Context, config *BuildConfig, extensionFile string) error {
	if m.cleanFn == nil {
		return nil
	}
	return m.cleanFn(ctx, config, extensionFile)
}

func TestBuildAllExtensionsStopsAfterMissingBuilder(t *testing.T) {
	factory := &BuilderFactory{}
	trackingBuilder := &mockBuilder{
		name: "tracking",
		canBuildFn: func(ext string) bool {
			return ext == "tracked.ext"
		},
	}
	factory.Register(trackingBuilder)

	ctx := context.Background()
	config := &BuildConfig{GemDir: "/tmp/test", StopOnFailure: true}

	results, err := factory.BuildAllExtensions(ctx, config, []string{"unknown.file", "tracked.ext"})

	if err == nil {
		t.Fatal("expected error for missing builder")
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if trackingBuilder.buildCalls != 0 {
		t.Fatalf("expected tracking builder to be skipped, but Build was called %d times", trackingBuilder.buildCalls)
	}
}

func TestBuildAllExtensionsHonorsContextCancellation(t *testing.T) {
	factory := &BuilderFactory{}
	ctx, cancel := context.WithCancel(context.Background())

	cancelBuilder := &mockBuilder{
		name: "cancel",
		canBuildFn: func(ext string) bool {
			return ext == failingExtension
		},
		buildFn: func(ctx context.Context, config *BuildConfig, extensionFile string) (*BuildResult, error) {
			cancel()
			return &BuildResult{Success: true}, nil
		},
	}

	trackingBuilder := &mockBuilder{
		name: "tracking",
		canBuildFn: func(ext string) bool {
			return ext == secondaryExtension
		},
	}

	factory.Register(cancelBuilder)
	factory.Register(trackingBuilder)

	config := &BuildConfig{GemDir: "/tmp/test"}

	results, err := factory.BuildAllExtensions(ctx, config, []string{failingExtension, secondaryExtension})

	if err == nil {
		t.Fatal("expected context cancellation error")
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled error, got %v", err)
	}

	if trackingBuilder.buildCalls != 0 {
		t.Fatalf("expected tracking builder to be skipped, but Build was called %d times", trackingBuilder.buildCalls)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results (one success, one cancellation), got %d", len(results))
	}

	if results[1].Success || !errors.Is(results[1].Error, context.Canceled) {
		t.Fatalf("expected second result to carry context cancellation, got %+v", results[1])
	}
}

func TestBuildAllExtensionsContinuesAfterFailure(t *testing.T) {
	factory := &BuilderFactory{}

	failureErr := errors.New("build failed")
	failingBuilder := &mockBuilder{
		name: "fail",
		canBuildFn: func(ext string) bool {
			return ext == failingExtension
		},
		buildFn: func(ctx context.Context, config *BuildConfig, extensionFile string) (*BuildResult, error) {
			return &BuildResult{Success: false, Error: failureErr}, failureErr
		},
	}

	trackingBuilder := &mockBuilder{
		name: "tracking",
		canBuildFn: func(ext string) bool {
			return ext == secondaryExtension
		},
	}

	factory.Register(failingBuilder)
	factory.Register(trackingBuilder)

	config := &BuildConfig{GemDir: "/tmp/test"}

	ctx := context.Background()
	results, err := factory.BuildAllExtensions(ctx, config, []string{failingExtension, secondaryExtension})

	if err == nil {
		t.Fatal("expected an error from failing builder")
	}

	if !errors.Is(err, failureErr) {
		t.Fatalf("expected failureErr, got %v", err)
	}

	if trackingBuilder.buildCalls != 1 {
		t.Fatalf("expected tracking builder to execute once, got %d", trackingBuilder.buildCalls)
	}

	if len(results) != 2 {
		t.Fatalf("expected results for both extensions, got %d", len(results))
	}

	if results[0].Success {
		t.Fatal("expected first result to be a failure")
	}

	if !results[1].Success {
		t.Fatal("expected second result to succeed")
	}
}

package rubyext

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFinalizeNativeExtensionsInstallsToVersionedLib(t *testing.T) {
	gemDir := t.TempDir()
	extDir := filepath.Join(gemDir, "ext", "json")

	if err := os.MkdirAll(extDir, 0o755); err != nil {
		t.Fatalf("failed to create extension directory: %v", err)
	}

	extconf := "require 'mkmf'\ncreate_makefile 'json/ext/parser'\n"
	extconfPath := filepath.Join(extDir, "extconf.rb")
	if err := os.WriteFile(extconfPath, []byte(extconf), 0o600); err != nil {
		t.Fatalf("failed to write extconf.rb: %v", err)
	}
	if err := os.Chmod(extconfPath, 0o755); err != nil {
		t.Fatalf("failed to chmod extconf.rb: %v", err)
	}

	bundlePath := filepath.Join(extDir, "parser.bundle")
	if err := os.WriteFile(bundlePath, []byte("binary"), 0o600); err != nil {
		t.Fatalf("failed to write bundle: %v", err)
	}
	if err := os.Chmod(bundlePath, 0o755); err != nil {
		t.Fatalf("failed to chmod bundle: %v", err)
	}

	config := &BuildConfig{
		GemDir:      gemDir,
		RubyVersion: "3.4.2",
	}

	installed, err := finalizeNativeExtensions(config, "ext/json/extconf.rb", extDir, []string{"parser.bundle"})
	if err != nil {
		t.Fatalf("finalizeNativeExtensions returned error: %v", err)
	}

	expected := filepath.ToSlash(filepath.Join("lib", "3.4", "json", "ext", "parser.bundle"))
	if len(installed) != 1 || installed[0] != expected {
		t.Fatalf("expected installed paths [%s], got %v", expected, installed)
	}

	versioned := filepath.Join(gemDir, "lib", "3.4", "json", "ext", "parser.bundle")
	if _, err := os.Stat(versioned); err != nil {
		t.Fatalf("expected bundle copied to %s: %v", versioned, err)
	}

	unversioned := filepath.Join(gemDir, "lib", "json", "ext", "parser.bundle")
	if _, err := os.Stat(unversioned); err != nil {
		t.Fatalf("expected bundle copied to %s: %v", unversioned, err)
	}
}

func TestFinalizeNativeExtensionsReturnsOriginalPathsForNonNative(t *testing.T) {
	gemDir := t.TempDir()
	extDir := filepath.Join(gemDir, "ext", "pkg")

	if err := os.MkdirAll(extDir, 0o755); err != nil {
		t.Fatalf("failed to create extension directory: %v", err)
	}

	if err := os.WriteFile(filepath.Join(extDir, "artifact.txt"), []byte("data"), 0o600); err != nil {
		t.Fatalf("failed to write artifact: %v", err)
	}

	config := &BuildConfig{
		GemDir:      gemDir,
		RubyVersion: "3.3.0",
	}

	installed, err := finalizeNativeExtensions(config, "ext/pkg/Makefile", extDir, []string{"artifact.txt"})
	if err != nil {
		t.Fatalf("finalizeNativeExtensions returned error: %v", err)
	}

	expected := "ext/pkg/artifact.txt"
	if len(installed) != 1 || installed[0] != expected {
		t.Fatalf("expected installed paths [%s], got %v", expected, installed)
	}

	if _, err := os.Stat(filepath.Join(extDir, "artifact.txt")); err != nil {
		t.Fatalf("expected artifact to remain in place: %v", err)
	}
}

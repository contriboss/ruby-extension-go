package rubyext

import (
	"errors"
	"reflect"
	"testing"
)

const testSystemRakePath = "/usr/bin/rake"

func TestDetermineRakeCommandUsesSystemRake(t *testing.T) {
	origLookPath := execLookPath
	defer func() { execLookPath = origLookPath }()

	execLookPath = func(name string) (string, error) {
		if name != "rake" {
			return "", errors.New("unexpected binary lookup")
		}
		return testSystemRakePath, nil
	}

	builder := &RakeBuilder{}
	args := []string{"compile"}

	cmd, resolvedArgs := builder.determineRakeCommand(&BuildConfig{}, args)

	if cmd != testSystemRakePath {
		t.Fatalf("expected rake command to be /usr/bin/rake, got %q", cmd)
	}

	if !reflect.DeepEqual(resolvedArgs, args) {
		t.Fatalf("expected args to remain unchanged, got %v", resolvedArgs)
	}

	if !reflect.DeepEqual(args, []string{"compile"}) {
		t.Fatalf("original args slice was mutated, got %v", args)
	}
}

func TestDetermineRakeCommandFallsBackToRuby(t *testing.T) {
	origLookPath := execLookPath
	defer func() { execLookPath = origLookPath }()

	execLookPath = func(string) (string, error) {
		return "", errors.New("not found")
	}

	builder := &RakeBuilder{}
	config := &BuildConfig{RubyPath: "/opt/ruby/bin/ruby"}
	args := []string{"compile", "--jobs=4"}

	cmd, resolvedArgs := builder.determineRakeCommand(config, args)

	if cmd != "/opt/ruby/bin/ruby" {
		t.Fatalf("expected ruby fallback, got %q", cmd)
	}

	expected := []string{
		"-rrubygems",
		"-e", `load Gem.bin_path("rake", "rake")`,
		"--",
		"compile",
		"--jobs=4",
	}

	if !reflect.DeepEqual(resolvedArgs, expected) {
		t.Fatalf("unexpected resolved args, expected %v, got %v", expected, resolvedArgs)
	}

	if !reflect.DeepEqual(args, []string{"compile", "--jobs=4"}) {
		t.Fatalf("original args slice was mutated, got %v", args)
	}
}

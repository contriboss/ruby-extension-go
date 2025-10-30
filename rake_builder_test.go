package rubyext

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"testing"
)

const (
	testSystemRakePath = "/usr/bin/rake"
	testSystemRubyPath = "/usr/bin/ruby"
	testRakeCommand    = "rake"
)

func TestDetermineRakeCommandUsesSystemRake(t *testing.T) {
	origLookPath := execLookPath
	defer func() { execLookPath = origLookPath }()

	execLookPath = func(name string) (string, error) {
		if name != testRakeCommand {
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

func TestEnsureRakeAvailableMissingRake(t *testing.T) {
	origLookPath := execLookPath
	origCmdCtx := execCommandContext
	defer func() {
		execLookPath = origLookPath
		execCommandContext = origCmdCtx
	}()

	execLookPath = func(name string) (string, error) {
		switch name {
		case testRakeCommand:
			return "", errors.New("not found")
		case rubyCommand:
			return testSystemRubyPath, nil
		default:
			return "", fmt.Errorf("unexpected lookup for %s", name)
		}
	}

	execCommandContext = helperCommand(1)

	builder := &RakeBuilder{}
	missing, err := builder.ensureRakeAvailable(context.Background(), &BuildConfig{})

	if err == nil || err.Error() != "rake not found" {
		t.Fatalf("expected rake not found error, got %v", err)
	}

	expectedMissing := []string{"rake"}
	if !reflect.DeepEqual(missing, expectedMissing) {
		t.Fatalf("expected missing dependencies %v, got %v", expectedMissing, missing)
	}
}

func TestEnsureRakeAvailableViaRubyGem(t *testing.T) {
	origLookPath := execLookPath
	origCmdCtx := execCommandContext
	defer func() {
		execLookPath = origLookPath
		execCommandContext = origCmdCtx
	}()

	execLookPath = func(name string) (string, error) {
		switch name {
		case testRakeCommand:
			return "", errors.New("not found")
		case rubyCommand:
			return testSystemRubyPath, nil
		default:
			return "", fmt.Errorf("unexpected lookup for %s", name)
		}
	}

	execCommandContext = helperCommand(0)

	builder := &RakeBuilder{}
	missing, err := builder.ensureRakeAvailable(context.Background(), &BuildConfig{})

	if err != nil {
		t.Fatalf("expected no error when rake available via RubyGems, got %v", err)
	}

	if len(missing) != 0 {
		t.Fatalf("expected no missing dependencies, got %v", missing)
	}
}

func helperCommand(exitCode int) func(context.Context, string, ...string) *exec.Cmd {
	return func(ctx context.Context, name string, args ...string) *exec.Cmd {
		_ = name
		_ = args
		cmdArgs := []string{"-test.run=TestHelperProcess", "--", strconv.Itoa(exitCode)}
		cmd := exec.CommandContext(ctx, os.Args[0], cmdArgs...) // #nosec G204 - helper process for testing
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	for i := 0; i < len(os.Args); i++ {
		if os.Args[i] == "--" && i+1 < len(os.Args) {
			code, err := strconv.Atoi(os.Args[i+1])
			if err != nil {
				os.Exit(1)
			}
			os.Exit(code)
		}
	}

	os.Exit(0)
}

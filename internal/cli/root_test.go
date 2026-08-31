package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootUsesGPAndWiresGlobalFlags(t *testing.T) {
	cmd := New()
	if cmd.Use != "gp" {
		t.Fatalf("Use = %q, want gp", cmd.Use)
	}
	if cmd.PersistentFlags().Lookup("repo") == nil {
		t.Fatal("missing --repo")
	}
	if cmd.PersistentFlags().Lookup("json") == nil {
		t.Fatal("missing --json")
	}
}

func TestHelpListsCompleteCommandSurfaceAndExamples(t *testing.T) {
	tests := []struct {
		args []string
		want []string
	}{
		{[]string{"--help"}, []string{"gp init", "gp status", "gp check", "gp approve", "gp ready", "gp graph", "gp remove", "gp task"}},
		{[]string{"task", "--help"}, []string{"gp task add", "gp task list", "gp task show", "gp task start", "gp task complete", "gp task reorder", "gp task remove"}},
	}
	for _, tt := range tests {
		var stdout, stderr bytes.Buffer
		code := Execute(tt.args, &stdout, &stderr)
		if code != ExitOK {
			t.Fatalf("Execute(%v) = %d, stderr=%q", tt.args, code, stderr.String())
		}
		for _, want := range tt.want {
			if !strings.Contains(stdout.String(), want) {
				t.Errorf("help for %v missing %q\n%s", tt.args, want, stdout.String())
			}
		}
	}
}

func TestRootJSONErrorEnvelope(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Execute([]string{"--json", "status"}, &stdout, &stderr)
	if code != ExitRuntime {
		t.Fatalf("code = %d, want %d", code, ExitRuntime)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if got["schema"] != "go-plan/v1" || got["command"] != "status" || got["ok"] != false {
		t.Fatalf("unexpected envelope: %#v", got)
	}
	errorValue, ok := got["error"].(map[string]any)
	if !ok || errorValue["code"] != "not_implemented" {
		t.Fatalf("unexpected error: %#v", got["error"])
	}
	details, ok := errorValue["details"].([]any)
	if !ok || len(details) != 0 {
		t.Fatalf("details = %#v, want []", errorValue["details"])
	}
}

func TestRootUsageErrorsUseExitTwoWithoutUsageDump(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Execute([]string{"not-a-command"}, &stdout, &stderr)
	if code != ExitUsage {
		t.Fatalf("code = %d, want %d", code, ExitUsage)
	}
	if strings.Contains(stderr.String(), "Usage:") {
		t.Fatalf("runtime output dumped usage: %q", stderr.String())
	}
}

func TestRootHumanRuntimeErrorsUseStderrWithoutUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Execute([]string{"status"}, &stdout, &stderr)
	if code != ExitRuntime {
		t.Fatalf("code = %d, want %d", code, ExitRuntime)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "not implemented") || strings.Contains(stderr.String(), "Usage:") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRootJSONGoldens(t *testing.T) {
	tests := []struct {
		name string
		got  []byte
	}{
		{"success", marshalSuccess("ready", map[string]any{"reason": "plan_completed", "task": nil})},
		{"error", marshalError("check", "plan_invalid", "plan validation failed", []Detail{{Path: ".go-plan/plan.yaml", Field: "schema", Message: "unsupported schema"}})},
	}
	for _, tt := range tests {
		want, err := os.ReadFile(filepath.Join("testdata", "json_"+tt.name+".golden"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(tt.got, want) {
			t.Fatalf("%s JSON mismatch\nwant: %s\n got: %s", tt.name, want, tt.got)
		}
	}
}

func TestRootSubprocessExitCodes(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "gp")
	build := exec.Command("go", "build", "-o", bin, "../../cmd/gp")
	build.Env = append(os.Environ(), "GOCACHE="+filepath.Join(t.TempDir(), "gocache"))
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build gp: %v\n%s", err, output)
	}
	for _, tt := range []struct {
		name string
		args []string
		want int
	}{
		{"success", []string{"--help"}, ExitOK},
		{"runtime", []string{"status"}, ExitRuntime},
		{"usage", []string{"not-a-command"}, ExitUsage},
	} {
		cmd := exec.Command(bin, tt.args...)
		err := cmd.Run()
		got := 0
		if err != nil {
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("%s: %v", tt.name, err)
			}
			got = exitErr.ExitCode()
		}
		if got != tt.want {
			t.Errorf("%s exit = %d, want %d", tt.name, got, tt.want)
		}
	}
}

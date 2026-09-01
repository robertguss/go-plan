package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestHelpSurfaceHasExamples(t *testing.T) {
	root := NewRoot(&bytes.Buffer{}, &bytes.Buffer{})
	want := []string{"init", "status", "check", "approve", "ready", "graph", "remove", "task"}
	for _, name := range want {
		c, _, err := root.Find([]string{name})
		if err != nil || c.Name() != name {
			t.Fatalf("missing %s", name)
		}
	}
	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		if c != root && c.Example == "" {
			t.Errorf("%s has no example", c.CommandPath())
		}
		for _, child := range c.Commands() {
			if child.Name() != "completion" && child.Name() != "help" {
				walk(child)
			}
		}
	}
	walk(root)
}

func TestJSONSuccessEnvelope(t *testing.T) {
	d := t.TempDir()
	if out, err := exec.Command("git", "-C", d, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git: %v %s", err, out)
	}
	var out bytes.Buffer
	root := NewRoot(&out, &bytes.Buffer{})
	root.SetArgs([]string{"--repo", d, "--json", "init", "--title", "Demo"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["schema"] != "go-plan/v1" || got["command"] != "init" || got["ok"] != true {
		t.Fatalf("envelope: %s", out.Bytes())
	}
	if _, err := os.Stat(filepath.Join(d, ".go-plan", "plan.yaml")); err != nil {
		t.Fatal(err)
	}
}

func TestCLIHelper(t *testing.T) {
	if os.Getenv("GO_PLAN_HELPER") != "1" {
		return
	}
	os.Args = append([]string{"goplan"}, strings.Split(os.Getenv("GO_PLAN_ARGS"), "\x1f")...)
	os.Exit(Execute())
}

func subprocess(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=TestCLIHelper")
	cmd.Env = append(os.Environ(), "GO_PLAN_HELPER=1", "GO_PLAN_ARGS="+strings.Join(args, "\x1f"))
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()
	if err == nil {
		return out.String(), errOut.String(), 0
	}
	if e, ok := err.(*exec.ExitError); ok {
		return out.String(), errOut.String(), e.ExitCode()
	}
	t.Fatal(err)
	return "", "", -1
}

func TestRunPreservesTypedErrors(t *testing.T) {
	d := t.TempDir()
	var out, errOut bytes.Buffer
	code := Run(&out, &errOut, []string{"--repo", d, "--json", "status"})
	if code != 1 || errOut.Len() != 0 {
		t.Fatalf("code=%d stderr=%q", code, errOut.String())
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["command"] != "status" || got["ok"] != false {
		t.Fatalf("envelope: %s", out.Bytes())
	}
	errObj, _ := got["error"].(map[string]any)
	if errObj["code"] != "repository_not_found" {
		t.Fatalf("wanted repository_not_found, got %s", out.Bytes())
	}

	out.Reset()
	errOut.Reset()
	code = Run(&out, &errOut, []string{"--json", "status", "extra"})
	if code != 2 || errOut.Len() != 0 || !strings.Contains(out.String(), `"code":"invalid_usage"`) || !strings.Contains(out.String(), `"command":"status"`) {
		t.Fatalf("usage: code=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
	}
}

func TestExitAndErrorContracts(t *testing.T) {
	_, stderr, code := subprocess(t, "status", "extra")
	if code != 2 || strings.Contains(stderr, "Usage:") {
		t.Fatalf("usage: code=%d stderr=%q", code, stderr)
	}
	stdout, stderr, code := subprocess(t, "--json", "status", "extra")
	if code != 2 || stderr != "" || !strings.Contains(stdout, `"code":"invalid_usage"`) {
		t.Fatalf("JSON usage: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	d := t.TempDir()
	exec.Command("git", "-C", d, "init", "-q").Run()
	stdout, stderr, code = subprocess(t, "--repo", d, "--json", "check")
	if code != 1 || stderr != "" {
		t.Fatalf("runtime: code=%d stderr=%q", code, stderr)
	}
	var got struct {
		Command string `json:"command"`
		OK      bool   `json:"ok"`
		Error   struct {
			Code    string `json:"code"`
			Details []any  `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got.Command != "check" || got.OK || got.Error.Code != "plan_invalid" || got.Error.Details == nil {
		t.Fatalf("error envelope: %s", stdout)
	}
}

func TestV1CompatibilityFixture(t *testing.T) {
	d := t.TempDir()
	if err := copyTree(filepath.Join("..", "..", "testdata", "compat", "v1", "plan"), d); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", d, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git: %v %s", err, out)
	}
	var out bytes.Buffer
	root := NewRoot(&out, &bytes.Buffer{})
	root.SetArgs([]string{"--repo", d, "--json", "status"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("..", "..", "testdata", "compat", "v1", "status.json"))
	if err != nil {
		t.Fatal(err)
	}
	if out.String() != string(want) {
		t.Fatalf("compatibility output changed\nwant %s\ngot  %s", want, out.String())
	}
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	})
}

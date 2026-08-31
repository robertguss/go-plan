package workspace

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (w *Workspace) gitPlanClean() error {
	out, err := exec.Command("git", "-C", w.Root, "status", "--porcelain", "--", ".go-plan").Output()
	if err != nil {
		return err
	}
	if len(out) > 0 {
		return fmt.Errorf(".go-plan has uncommitted or untracked changes")
	}
	out, err = exec.Command("git", "-C", w.Root, "ls-files", "-z", "--", ".go-plan").Output()
	if err != nil {
		return err
	}
	tracked := map[string]bool{}
	for _, p := range splitGitNUL(out) {
		tracked[p] = true
	}
	managed := map[string]bool{}
	err = filepath.WalkDir(filepath.Join(w.Root, ".go-plan"), func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(w.Root, path)
		if err != nil {
			return err
		}
		managed[filepath.ToSlash(rel)] = true
		return nil
	})
	if err != nil {
		return err
	}
	if !samePathSet(tracked, managed) {
		return fmt.Errorf("every .go-plan file must be tracked before removal")
	}
	return nil
}

func splitGitNUL(out []byte) []string {
	if len(out) == 0 {
		return nil
	}
	parts := strings.Split(strings.TrimRight(string(out), "\x00"), "\x00")
	r := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			r = append(r, filepath.ToSlash(p))
		}
	}
	return r
}

func samePathSet(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

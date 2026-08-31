package workspace

import (
	"fmt"
	"os/exec"
	"strings"
)

func (w *Workspace) gitPlanClean(preview []string) error {
	out, err := exec.Command("git", "-C", w.Root, "status", "--porcelain", "--", ".go-plan").Output()
	if err != nil {
		return err
	}
	if len(out) > 0 {
		return fmt.Errorf(".go-plan has uncommitted or untracked changes")
	}
	out, err = exec.Command("git", "-C", w.Root, "ls-files", "--", ".go-plan").Output()
	if err != nil {
		return err
	}
	tracked := strings.Fields(string(out))
	if len(tracked) != len(preview)-1 {
		return fmt.Errorf("every .go-plan file must be tracked before removal")
	}
	return nil
}

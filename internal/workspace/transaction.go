package workspace

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func (w *Workspace) lockPath() (string, error) {
	out, err := exec.Command("git", "-C", w.Root, "rev-parse", "--git-path", "go-plan.lock").Output()
	if err != nil {
		return "", err
	}
	p := strings.TrimSpace(string(out))
	if !filepath.IsAbs(p) {
		p = filepath.Join(w.Root, p)
	}
	return p, nil
}

func (w *Workspace) withLock(fn func() error) error {
	lock, err := w.lockPath()
	if err != nil {
		return err
	}
	lf, err := os.OpenFile(lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("another goplan mutation is in progress")
	}
	lf.Close()
	defer os.Remove(lock)
	return fn()
}

// Publish atomically replaces a candidate set as one reported operation and
// restores every original byte if any publication step fails. A nil value
// removes a managed file.
func (w *Workspace) Publish(files map[string][]byte) error {
	return w.withLock(func() error {
		tmp, err := os.MkdirTemp(w.Root, ".goplan-transaction-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmp)
		keys := make([]string, 0, len(files))
		for k := range files {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		type original struct {
			data   []byte
			mode   os.FileMode
			exists bool
		}
		orig := map[string]original{}
		for i, k := range keys {
			p, err := w.safe(k, true)
			if err != nil {
				return err
			}
			fi, e := os.Lstat(p)
			if e == nil {
				if fi.IsDir() {
					return fmt.Errorf("managed target is a directory: %s", k)
				}
				b, e := os.ReadFile(p)
				if e != nil {
					return e
				}
				orig[k] = original{b, fi.Mode(), true}
			} else if !errors.Is(e, os.ErrNotExist) {
				return e
			}
			if files[k] != nil {
				stage := filepath.Join(tmp, fmt.Sprintf("%06d", i))
				if err = os.WriteFile(stage, files[k], 0644); err != nil {
					return err
				}
				f, e := os.Open(stage)
				if e == nil {
					e = f.Sync()
					f.Close()
				}
				if e != nil {
					return e
				}
			}
		}
		rollback := func() {
			for _, k := range keys {
				p, _ := w.safe(k, true)
				o := orig[k]
				if o.exists {
					os.MkdirAll(filepath.Dir(p), 0755)
					_ = os.WriteFile(p, o.data, o.mode)
				} else {
					_ = os.Remove(p)
				}
			}
		}
		for i, k := range keys {
			p, _ := w.safe(k, true)
			if w.beforePublish != nil {
				if err = w.beforePublish(i, k); err != nil {
					rollback()
					return err
				}
			}
			if err = os.MkdirAll(filepath.Dir(p), 0755); err != nil {
				rollback()
				return err
			}
			if files[k] == nil {
				err = os.Remove(p)
				if errors.Is(err, os.ErrNotExist) {
					err = nil
				}
			} else {
				err = os.Rename(filepath.Join(tmp, fmt.Sprintf("%06d", i)), p)
			}
			if err != nil {
				rollback()
				return fmt.Errorf("publish %s: %w", k, err)
			}
		}
		return nil
	})
}

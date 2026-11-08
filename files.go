package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// absPath expands ~ and checks that the path is absolute.
func absPath(dir string) (string, error) {
	if dir == "~" || strings.HasPrefix(dir, "~/") {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, dir[1:])
	}
	if !filepath.IsAbs(dir) {
		return "", fmt.Errorf("dir %q is not absolute. Use a full path on the moki machine, for example /home/me/mockup", dir)
	}
	return filepath.Clean(dir), nil
}

// copyTree copies the regular files of src into dst. It skips symlinks and .git.
func copyTree(src, dst string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, _ := filepath.Rel(src, p)
		if err := copyFile(p, filepath.Join(dst, rel)); err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	return files, err
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func hasFile(files []string, name string) bool {
	for _, f := range files {
		if f == name {
			return true
		}
	}
	return false
}

func joinLines(lines []string) string {
	if len(lines) > 200 {
		lines = append(lines[:200:200], fmt.Sprintf("... and %d more", len(lines)-200))
	}
	return strings.Join(lines, "\n")
}

func warnList(warns []string) string {
	if len(warns) == 0 {
		return "No warnings."
	}
	return "Warnings:\n- " + strings.Join(warns, "\n- ")
}

// warnNote is the warning count for an activity line, or "" when there is none.
func warnNote(warns []string) string {
	if len(warns) == 0 {
		return ""
	}
	return " · " + yellow(count(len(warns), "warning"))
}

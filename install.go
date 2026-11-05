package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func exeName() string {
	if runtime.GOOS == "windows" {
		return "local-docs.exe"
	}
	return "local-docs"
}

func installDir() (string, error) {
	if runtime.GOOS == "windows" {
		if d := os.Getenv("LOCALAPPDATA"); d != "" {
			return filepath.Join(d, "Programs", "local-docs"), nil
		}
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".local", "bin"), nil
}

func skillTargets() ([]string, error) {
	h, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return []string{filepath.Join(h, ".claude", "skills"), filepath.Join(h, ".codex", "skills")}, nil
}

func embeddedSkills() (map[string]map[string][]byte, error) {
	out := map[string]map[string][]byte{}
	err := fs.WalkDir(embeddedFS, ".agents/skills", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel := strings.TrimPrefix(p, ".agents/skills/")
		name, file, _ := strings.Cut(rel, "/")
		b, err := embeddedFS.ReadFile(p)
		if err != nil {
			return err
		}
		if out[name] == nil {
			out[name] = map[string][]byte{}
		}
		out[name][file] = b
		return nil
	})
	return out, err
}

func skillMatches(dir string, files map[string][]byte) bool {
	if fi, err := os.Lstat(dir); err != nil || fi.Mode()&os.ModeSymlink != 0 {
		return false
	}
	for f, want := range files {
		got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(f)))
		if err != nil || !bytes.Equal(got, want) {
			return false
		}
	}
	return true
}

func writeSkill(dir string, files map[string][]byte) error {
	for f, b := range files {
		p := filepath.Join(dir, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(p, b, 0644); err != nil {
			return err
		}
	}
	return nil
}

// installSkills writes the embedded skills into base (one skills directory).
// refresh only touches skills that already exist; force replaces differing
// ones after moving them to a backup under root.
func installSkills(base, root string, force, refresh bool) error {
	skills, err := embeddedSkills()
	if err != nil {
		return err
	}
	backup := filepath.Join(root, "skill-backups", time.Now().Format("20060102-150405"))
	for name, files := range skills {
		dest := filepath.Join(base, name)
		_, statErr := os.Lstat(dest)
		exists := statErr == nil
		switch {
		case !exists && refresh:
			continue
		case exists && skillMatches(dest, files):
			fmt.Printf("skill %s: up to date\n", dest)
			continue
		case exists && !force && !refresh:
			fmt.Printf("skill %s: differs, kept (use --force to back up and replace)\n", dest)
			continue
		case exists:
			b := filepath.Join(backup, filepath.Base(filepath.Dir(base)), name)
			if err := os.MkdirAll(filepath.Dir(b), 0700); err != nil {
				return err
			}
			if err := os.Rename(dest, b); err != nil {
				return err
			}
			fmt.Printf("skill %s: previous version saved in %s\n", dest, b)
		}
		if err := writeSkill(dest, files); err != nil {
			return err
		}
		fmt.Printf("skill %s: installed\n", dest)
	}
	return nil
}

func installSelf(root string) error {
	set := flag.NewFlagSet("install", flag.ExitOnError)
	force := set.Bool("force", false, "replace differing skills (previous ones are backed up)")
	refresh := set.Bool("refresh", false, "only refresh already installed skills (used by update)")
	skillsOnly := set.Bool("skills-only", false, "install skills only, not the binary and PATH")
	noPath := set.Bool("no-path", false, "do not modify the user PATH")
	_ = set.Parse(flag.Args()[1:])

	if !*skillsOnly && !*refresh {
		self, err := os.Executable()
		if err != nil {
			return errNoExe
		}
		dir, err := installDir()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		dest := filepath.Join(dir, exeName())
		if !samePath(self, dest) {
			if err := copyFile(self, dest); err != nil {
				return err
			}
		}
		fmt.Println("binary:", dest)
		if !*noPath {
			if err := addToPath(dir); err != nil {
				return err
			}
		}
		if err := initIfMissing(root); err != nil {
			return err
		}
	}
	targets, err := skillTargets()
	if err != nil {
		return err
	}
	for _, base := range targets {
		if err := installSkills(base, root, *force, *refresh); err != nil {
			return err
		}
	}
	if !*refresh {
		checkTools(root)
	}
	return nil
}

func initIfMissing(root string) error {
	if _, err := os.Stat(filepath.Join(root, "config.json")); err == nil {
		fmt.Println("config:", filepath.Join(root, "config.json"), "(exists)")
		return nil
	}
	if err := initConfig(root); err != nil {
		return err
	}
	fmt.Println("config created:", filepath.Join(root, "config.json"), "- adjust sources and tool paths")
	return nil
}

func samePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".new"
	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	if _, err = io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err = out.Close(); err != nil {
		return err
	}
	return replaceFile(tmp, dst)
}

// replaceFile moves tmp over dst; an existing (possibly running) dst is
// renamed to .old first, which also works for a running .exe on Windows.
func replaceFile(tmp, dst string) error {
	old := dst + ".old"
	_ = os.Remove(old)
	if _, err := os.Stat(dst); err == nil {
		if err := os.Rename(dst, old); err != nil {
			return err
		}
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Rename(old, dst)
		return err
	}
	return nil
}

func addToPath(dir string) error {
	if runtime.GOOS != "windows" {
		for _, p := range filepath.SplitList(os.Getenv("PATH")) {
			if p == dir {
				return nil
			}
		}
		fmt.Printf("add %s to your PATH, e.g. in your shell profile: export PATH=\"%s:$PATH\"\n", dir, dir)
		return nil
	}
	script := `$d=$env:LOCAL_DOCS_BIN; $p=[Environment]::GetEnvironmentVariable('Path','User'); if(-not $p){$p=''};` +
		`$parts=@($p -split ';' | Where-Object { $_ }); if($parts -notcontains $d){ [Environment]::SetEnvironmentVariable('Path',(($parts+$d) -join ';'),'User'); 'added' } else { 'present' }`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.Env = append(os.Environ(), "LOCAL_DOCS_BIN="+dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("PATH update failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	if strings.TrimSpace(string(out)) == "added" {
		fmt.Println("user PATH extended; open a new terminal to use `local-docs`")
	} else {
		fmt.Println("user PATH already contains", dir)
	}
	return nil
}

// checkTools resolves the external tools exactly like sync/page do: Poppler
// via poppler_bin/pdftotext from config.json, otherwise via PATH.
func checkTools(root string) {
	var cfg Config
	if b, err := os.ReadFile(filepath.Join(root, "config.json")); err == nil {
		_ = json.Unmarshal(b, &cfg)
	}
	hint := map[string]string{
		"rg":        "ripgrep (required for search)",
		"pdfinfo":   "Poppler (PDF page counts)",
		"pdftotext": "Poppler (PDF text)",
		"pdftoppm":  "Poppler (page images)",
	}
	for _, t := range []string{"rg", "pdfinfo", "pdftotext", "pdftoppm"} {
		name := t
		if t != "rg" {
			name = poppler(cfg, t)
		}
		if p, err := exec.LookPath(name); err == nil {
			fmt.Printf("tool %-9s %s\n", t, p)
		} else {
			fmt.Printf("tool %-9s MISSING - install %s or set poppler_bin/pdftotext in config.json\n", t, hint[t])
		}
	}
}

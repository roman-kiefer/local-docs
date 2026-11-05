package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// releaseAPI is a variable so tests can point it at a local server.
var releaseAPI = "https://api.github.com/repos/roman-kiefer/local-docs/releases/latest"

type release struct {
	Tag    string `json:"tag_name"`
	Assets []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func fetch(url string, limit int64) ([]byte, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "local-docs/"+version)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("%s: HTTP %d", url, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, limit))
}

func latestRelease() (release, error) {
	var r release
	b, err := fetch(releaseAPI, 4<<20)
	if err != nil {
		return r, err
	}
	err = json.Unmarshal(b, &r)
	return r, err
}

// parseVersion reads "v1.2.3" or "1.2.3-rc1" into numbers plus a prerelease flag.
func parseVersion(s string) (nums [3]int, pre bool, ok bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	core, rest, hasPre := strings.Cut(s, "-")
	parts := strings.Split(core, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nums, false, false
		}
		nums[i] = n
	}
	_ = rest
	return nums, hasPre, true
}

// newer reports whether candidate is a higher version than current.
func newer(candidate, current string) bool {
	c, cPre, ok1 := parseVersion(candidate)
	v, vPre, ok2 := parseVersion(current)
	if !ok1 || !ok2 {
		return false
	}
	for i := range c {
		if c[i] != v[i] {
			return c[i] > v[i]
		}
	}
	return vPre && !cPre
}

func assetName() string {
	return fmt.Sprintf("local-docs_%s_%s.zip", runtime.GOOS, runtime.GOARCH)
}

func updateSelf(root string, checkOnly bool) error {
	if exe, err := os.Executable(); err == nil {
		_ = os.Remove(exe + ".old")
	}
	r, err := latestRelease()
	if err != nil {
		return fmt.Errorf("release lookup: %w", err)
	}
	if !newer(r.Tag, version) {
		fmt.Printf("local-docs %s is up to date (latest release %s)\n", version, r.Tag)
		return nil
	}
	if checkOnly {
		fmt.Printf("new version available: %s (installed %s); run: local-docs update\n", r.Tag, version)
		return nil
	}
	if version == "dev" {
		return fmt.Errorf("refusing to replace a development build; install a release first")
	}
	var archiveURL, sumsURL string
	for _, a := range r.Assets {
		switch a.Name {
		case assetName():
			archiveURL = a.URL
		case "SHA256SUMS":
			sumsURL = a.URL
		}
	}
	if archiveURL == "" || sumsURL == "" {
		return fmt.Errorf("release %s has no %s or SHA256SUMS", r.Tag, assetName())
	}
	sums, err := fetch(sumsURL, 1<<20)
	if err != nil {
		return err
	}
	want := ""
	for _, line := range strings.Split(string(sums), "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == assetName() {
			want = strings.ToLower(f[0])
		}
	}
	if want == "" {
		return fmt.Errorf("SHA256SUMS has no entry for %s", assetName())
	}
	archive, err := fetch(archiveURL, 200<<20)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(archive)
	if hex.EncodeToString(sum[:]) != want {
		return fmt.Errorf("checksum mismatch for %s; update aborted", assetName())
	}
	bin, err := extractBinary(archive)
	if err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return errNoExe
	}
	if self, err = filepath.EvalSymlinks(self); err != nil {
		return err
	}
	tmp := self + ".new"
	if err := os.WriteFile(tmp, bin, 0755); err != nil {
		return err
	}
	if err := replaceFile(tmp, self); err != nil {
		return err
	}
	fmt.Printf("updated %s -> %s\n", version, r.Tag)
	// Let the new binary refresh its own embedded skills (only already installed ones).
	out, err := exec.Command(self, "install", "--refresh").CombinedOutput()
	fmt.Print(string(out))
	if err != nil {
		fmt.Println("warning: skill refresh failed:", err)
	}
	fmt.Println("hint: run `local-docs sync` if search results look outdated after an index format change")
	return nil
}

func extractBinary(archive []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, err
	}
	for _, f := range zr.File {
		if f.Name != exeName() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(io.LimitReader(rc, 200<<20))
	}
	return nil, fmt.Errorf("archive does not contain %s", exeName())
}

// maybeNotifyUpdate prints a one-line hint to stderr at most once per day.
func maybeNotifyUpdate(root string) {
	if version == "dev" {
		return
	}
	if b, err := os.ReadFile(filepath.Join(root, "config.json")); err == nil {
		var c Config
		if json.Unmarshal(b, &c) == nil && c.CheckUpdates != nil && !*c.CheckUpdates {
			return
		}
	}
	stamp := filepath.Join(dataDir(root), "update-check.json")
	if fi, err := os.Stat(stamp); err == nil && time.Since(fi.ModTime()) < 24*time.Hour {
		return
	}
	_ = os.MkdirAll(filepath.Dir(stamp), 0755)
	_ = os.WriteFile(stamp, []byte(time.Now().Format(time.RFC3339)), 0644)
	b, err := fetchShort(releaseAPI)
	if err != nil {
		return
	}
	var r release
	if json.Unmarshal(b, &r) == nil && newer(r.Tag, version) {
		fmt.Fprintf(os.Stderr, "note: new local-docs version %s available (installed %s); run `local-docs update` when convenient\n", r.Tag, version)
	}
}

func fetchShort(url string) ([]byte, error) {
	client := &http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "local-docs/"+version)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 4<<20))
}

package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveRootOrder(t *testing.T) {
	home := t.TempDir()
	env := t.TempDir()
	flagDir := t.TempDir()
	t.Setenv("LOCAL_DOCS_HOME", env)
	if got, _ := resolveRoot(flagDir); got != flagDir {
		t.Errorf("flag: %s", got)
	}
	if got, _ := resolveRoot(""); got != env {
		t.Errorf("env: %s", got)
	}
	t.Setenv("LOCAL_DOCS_HOME", "")
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	if err := os.Chdir(home); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "lad"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	t.Setenv("HOME", home)
	if got, _ := resolveRoot(""); got == home {
		t.Errorf("without config.json cwd must not win: %s", got)
	}
	if err := os.WriteFile("config.json", []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if got, _ := resolveRoot(""); got != home && !samePath(got, home) {
		t.Errorf("cwd with config.json: %s", got)
	}
}

func TestDataDirOverride(t *testing.T) {
	root := t.TempDir()
	defer func() { dataDirOverride = "" }()
	if dataDir(root) != filepath.Join(root, "data") {
		t.Fatal("default data dir")
	}
	abs := t.TempDir()
	os.WriteFile(filepath.Join(root, "config.json"), []byte(`{"data_dir":"`+filepath.ToSlash(abs)+`"}`), 0600)
	applyDataDir(root)
	if dataDir(root) != filepath.Clean(abs) {
		t.Fatalf("override: %s", dataDir(root))
	}
}

func TestNewer(t *testing.T) {
	cases := []struct {
		cand, cur string
		want      bool
	}{
		{"v1.2.0", "v1.1.9", true},
		{"v1.2.0", "v1.2.0", false},
		{"v1.2.0", "v1.10.0", false},
		{"v1.2.0", "v1.2.0-rc1", true},
		{"v1.2.0-rc1", "v1.2.0", false},
		{"v1.0.0", "dev", false},
	}
	for _, c := range cases {
		if got := newer(c.cand, c.cur); got != c.want {
			t.Errorf("newer(%s,%s)=%v", c.cand, c.cur, got)
		}
	}
}

func TestEmbeddedSkillsAndConfig(t *testing.T) {
	skills, err := embeddedSkills()
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"local-doc-search", "local-doc-sync", "local-doc-update"} {
		if skills[n]["SKILL.md"] == nil {
			t.Errorf("missing embedded skill %s", n)
		}
	}
	base := filepath.Join(t.TempDir(), "skills")
	root := t.TempDir()
	if err := installSkills(base, root, false, false); err != nil {
		t.Fatal(err)
	}
	if !skillMatches(filepath.Join(base, "local-doc-search"), skills["local-doc-search"]) {
		t.Fatal("installed skill differs")
	}
	// differing skill is kept without force, replaced with backup under force
	p := filepath.Join(base, "local-doc-sync", "SKILL.md")
	os.WriteFile(p, []byte("x"), 0644)
	installSkills(base, root, false, false)
	if b, _ := os.ReadFile(p); string(b) != "x" {
		t.Fatal("kept skill was overwritten")
	}
	installSkills(base, root, true, false)
	if b, _ := os.ReadFile(p); string(b) == "x" {
		t.Fatal("force did not replace")
	}
}

func TestExtractBinaryAndChecksumServer(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create(exeName())
	w.Write([]byte("binary"))
	zw.Close()
	got, err := extractBinary(buf.Bytes())
	if err != nil || string(got) != "binary" {
		t.Fatalf("extract: %v %q", err, got)
	}
	sum := sha256.Sum256(buf.Bytes())
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rel":
			w.Write([]byte(`{"tag_name":"v9.0.0","assets":[{"name":"` + assetName() + `","browser_download_url":"` + srv.URL + `/a"},{"name":"SHA256SUMS","browser_download_url":"` + srv.URL + `/s"}]}`))
		case "/a":
			w.Write(buf.Bytes())
		case "/s":
			w.Write([]byte(hex.EncodeToString(sum[:]) + "  " + assetName() + "\n"))
		}
	}))
	defer srv.Close()
	oldAPI, oldVer := releaseAPI, version
	defer func() { releaseAPI, version = oldAPI, oldVer }()
	releaseAPI = srv.URL + "/rel"
	version = "v1.0.0"
	if err := updateSelf(t.TempDir(), true); err != nil {
		t.Fatal(err)
	}
}

func TestReplaceFileKeepsOld(t *testing.T) {
	d := t.TempDir()
	dst := filepath.Join(d, "x")
	os.WriteFile(dst, []byte("old"), 0755)
	os.WriteFile(dst+".new", []byte("new"), 0755)
	if err := replaceFile(dst+".new", dst); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(dst); string(b) != "new" {
		t.Fatal("not replaced")
	}
	if b, _ := os.ReadFile(dst + ".old"); string(b) != "old" {
		t.Fatal("old not kept")
	}
}

func releaseServer(t *testing.T, archive []byte, sumLine string) release {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/a":
			w.Write(archive)
		case "/s":
			w.Write([]byte(sumLine))
		}
	}))
	t.Cleanup(srv.Close)
	r := release{Tag: "v9.0.0"}
	r.Assets = append(r.Assets, struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	}{assetName(), srv.URL + "/a"}, struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	}{"SHA256SUMS", srv.URL + "/s"})
	return r
}

func TestDownloadVerified(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create(exeName())
	w.Write([]byte("binary"))
	zw.Close()
	sum := sha256.Sum256(buf.Bytes())
	good := hex.EncodeToString(sum[:]) + "  " + assetName() + "\n"
	got, err := downloadVerified(releaseServer(t, buf.Bytes(), good))
	if err != nil || string(got) != "binary" {
		t.Fatalf("good: %v %q", err, got)
	}
	bad := strings.Repeat("0", 64) + "  " + assetName() + "\n"
	if _, err := downloadVerified(releaseServer(t, buf.Bytes(), bad)); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("mismatch not detected: %v", err)
	}
	other := hex.EncodeToString(sum[:]) + "  other.zip\n"
	if _, err := downloadVerified(releaseServer(t, buf.Bytes(), other)); err == nil {
		t.Fatal("missing checksum entry accepted")
	}
}

func TestDevBuildNotUpdated(t *testing.T) {
	old := version
	defer func() { version = old }()
	version = "dev"
	if err := updateSelf(t.TempDir(), false); err == nil {
		t.Fatal("dev build must refuse update")
	}
	if err := updateSelf(t.TempDir(), true); err != nil {
		t.Fatal(err)
	}
}

func TestDataDirOverlapsSource(t *testing.T) {
	root := t.TempDir()
	src := t.TempDir()
	defer func() { dataDirOverride = "" }()
	inside := filepath.Join(src, "index")
	cfg := `{"sources":["` + filepath.ToSlash(src) + `"],"data_dir":"` + filepath.ToSlash(inside) + `"}`
	os.WriteFile(filepath.Join(root, "config.json"), []byte(cfg), 0600)
	applyDataDir(root)
	if _, err := readConfig(root); err == nil {
		t.Fatal("data_dir inside source accepted")
	}
}

func TestRefreshKeepsSymlinkedSkill(t *testing.T) {
	base := filepath.Join(t.TempDir(), "skills")
	target := t.TempDir()
	os.MkdirAll(base, 0755)
	link := filepath.Join(base, "local-doc-search")
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	if err := installSkills(base, t.TempDir(), false, true); err != nil {
		t.Fatal(err)
	}
	if !isSymlink(link) {
		t.Fatal("refresh replaced a symlinked skill")
	}
}

func TestUpdateWithoutReleaseIsNotAnError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	oldAPI, oldVer := releaseAPI, version
	defer func() { releaseAPI, version = oldAPI, oldVer }()
	releaseAPI = srv.URL + "/latest"
	version = "v1.0.0"
	if _, err := latestRelease(); !errors.Is(err, errNoRelease) {
		t.Fatalf("want errNoRelease, got %v", err)
	}
	if err := updateSelf(t.TempDir(), false); err != nil {
		t.Fatalf("404 must not be an error: %v", err)
	}
}

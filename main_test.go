package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFamilyVersionAndPreference(t *testing.T) {
	cases := []struct{ path, family, version string }{
		{`C:\docs\Manuals\Manual_5.1.01-nokey.pdf`, "manual", "5.1.01"},
		{`C:\docs\Manuals\Manual_5.1.01.odt`, "manual", "5.1.01"},
		{`C:\docs\v_4_2_00\Firewall.pdf`, "firewall", "4.2.00"},
		{`C:\docs\Manual5002.pdf`, "manual", "5.0.02"},
	}
	for _, c := range cases {
		family, version := familyVersion(c.path)
		if family != c.family || version != c.version {
			t.Errorf("%q: got %q/%q", c.path, family, version)
		}
	}
	if preference(Document{Path: `C:\docs\Manual_5.1.01-nokey.pdf`, Format: "pdf"}) >= preference(Document{Path: `C:\docs\Manual_5.1.01.pdf`, Format: "pdf"}) {
		t.Fatal("nokey PDF should rank first")
	}
}

func TestSupportedSkipsOfficeLockFile(t *testing.T) {
	if supported(`C:\docs\~$manual.docx`, Config{}) {
		t.Fatal("Office lock file was accepted")
	}
}

func TestXMLExtraction(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct{ format, name, body string }{
		{"docx", "word/document.xml", `<w:document xmlns:w="urn:w"><w:body><w:p><w:r><w:t>Alpha</w:t></w:r><w:r><w:t>Beta</w:t></w:r></w:p><w:p><w:r><w:t>Gamma</w:t></w:r></w:p></w:body></w:document>`},
		{"odt", "content.xml", `<office:document-content xmlns:office="urn:o" xmlns:text="urn:t"><office:body><text:p>Alpha<text:span>Beta</text:span></text:p><text:h>Gamma</text:h></office:body></office:document-content>`},
	} {
		path := filepath.Join(dir, "sample."+tc.format)
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		z := zip.NewWriter(f)
		w, err := z.Create(tc.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(tc.body)); err != nil {
			t.Fatal(err)
		}
		if err := z.Close(); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		got, err := extractZIPXML(path, tc.format)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(got, "AlphaBeta\nGamma\n") {
			t.Errorf("%s: %q", tc.format, got)
		}
	}
}

func TestConfigRejectsSourceOverlap(t *testing.T) {
	root := t.TempDir()
	b := []byte(`{"sources":["` + filepath.ToSlash(root) + `"]}`)
	if err := os.WriteFile(filepath.Join(root, "config.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readConfig(root); err == nil {
		t.Fatal("expected overlap error")
	}
}

package main

import (
	"archive/zip"
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Sources         []string `json:"sources"`
	PopplerBin      string   `json:"poppler_bin"`
	PDFToText       string   `json:"pdftotext"`
	LibreOffice     string   `json:"libreoffice"`
	EnableLegacyDOC bool     `json:"enable_legacy_doc"`
}

type Document struct {
	ID       string   `json:"id"`
	Path     string   `json:"path"`
	Source   string   `json:"source"`
	Format   string   `json:"format"`
	Family   string   `json:"family"`
	Version  string   `json:"version"`
	SHA256   string   `json:"sha256"`
	Size     int64    `json:"size"`
	Modified string   `json:"modified"`
	Pages    int      `json:"pages,omitempty"`
	Texts    []string `json:"texts,omitempty"`
	Status   string   `json:"status"`
	Error    string   `json:"error,omitempty"`
}

type Manifest struct {
	Schema    int        `json:"schema"`
	Updated   string     `json:"updated"`
	Documents []Document `json:"documents"`
}

var versionRE = regexp.MustCompile(`(?i)(?:^|[^0-9])v?(\d+)[._-](\d+)(?:[._-](\d+))?(?:[^0-9]|$)`)
var compactVersionRE = regexp.MustCompile(`(\d)(\d)(\d{2})$`)
var suffixRE = regexp.MustCompile(`(?i)(?:[-_](?:nokey|old|key|neu|new|final|draft|kopie|copy))+$`)
var pagesRE = regexp.MustCompile(`(?m)^Pages:\s*(\d+)\s*$`)

func main() {
	rootFlag := flag.String("root", ".", "project directory")
	flag.Parse()
	if flag.NArg() == 0 {
		fatal(errors.New("use: local-docs [--root DIR] init|sync|search|page"))
	}
	root, err := filepath.Abs(*rootFlag)
	if err != nil {
		fatal(err)
	}
	cmd := flag.Arg(0)
	switch cmd {
	case "init":
		err = initConfig(root)
	case "sync":
		var cfg Config
		cfg, err = readConfig(root)
		if err == nil {
			err = syncDocs(root, cfg)
		}
	case "search":
		fs := flag.NewFlagSet("search", flag.ExitOnError)
		query := fs.String("query", "", "literal search phrase")
		all := fs.Bool("all", false, "include fallback variants")
		limit := fs.Int("limit", 100, "maximum number of hits")
		_ = fs.Parse(flag.Args()[1:])
		if *query == "" {
			err = errors.New("search requires --query")
		} else {
			err = searchDocs(root, *query, *all, *limit)
		}
	case "page":
		fs := flag.NewFlagSet("page", flag.ExitOnError)
		id := fs.String("id", "", "document ID from search")
		page := fs.Int("page", 0, "PDF page number")
		_ = fs.Parse(flag.Args()[1:])
		if *id == "" || *page < 1 {
			err = errors.New("page requires --id and positive --page")
		} else {
			var cfg Config
			cfg, err = readConfig(root)
			if err == nil {
				err = viewPage(root, cfg, *id, *page)
			}
		}
	default:
		err = fmt.Errorf("unknown command %q", cmd)
	}
	if err != nil {
		fatal(err)
	}
}

func fatal(err error) { fmt.Fprintln(os.Stderr, "error:", err); os.Exit(1) }

func initConfig(root string) error {
	b, err := os.ReadFile(filepath.Join(root, "config.example.json"))
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(root, "config.json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(b)
	return err
}

func readConfig(root string) (Config, error) {
	var cfg Config
	b, err := os.ReadFile(filepath.Join(root, "config.json"))
	if err != nil {
		return cfg, fmt.Errorf("config.json: %w (run init first)", err)
	}
	if err = json.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	if len(cfg.Sources) == 0 {
		return cfg, errors.New("config requires at least one source")
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return cfg, err
	}
	for i, source := range cfg.Sources {
		if !filepath.IsAbs(source) {
			return cfg, fmt.Errorf("source must be absolute: %q", source)
		}
		abs, err := filepath.Abs(source)
		if err != nil {
			return cfg, err
		}
		if within(abs, root) || within(root, abs) {
			return cfg, fmt.Errorf("project and source overlap: %q", source)
		}
		cfg.Sources[i] = filepath.Clean(abs)
	}
	return cfg, nil
}

func within(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func readManifest(root string) (Manifest, error) {
	var m Manifest
	b, err := os.ReadFile(filepath.Join(root, "data", "manifest.json"))
	if os.IsNotExist(err) {
		return Manifest{Schema: 1}, nil
	}
	if err != nil {
		return m, err
	}
	if err = json.Unmarshal(b, &m); err != nil {
		return m, err
	}
	if m.Schema != 1 {
		return m, fmt.Errorf("unsupported manifest schema %d", m.Schema)
	}
	return m, nil
}

func atomicJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	f, err := os.CreateTemp(filepath.Dir(path), ".manifest-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func fileID(path string) string {
	h := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(path))))
	return hex.EncodeToString(h[:12])
}

func fileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func familyVersion(path string) (string, string) {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	base = suffixRE.ReplaceAllString(base, "")
	version := ""
	if m := versionRE.FindStringSubmatch(base); m != nil {
		version = joinVersion(m)
		base = strings.Replace(base, m[0], "", 1)
	}
	if version == "" {
		if m := compactVersionRE.FindStringSubmatch(base); m != nil {
			version = strings.Join(m[1:], ".")
			base = strings.TrimSuffix(base, m[0])
		}
	}
	if version == "" {
		for dir := filepath.Dir(path); dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
			if m := versionRE.FindStringSubmatch(filepath.Base(dir)); m != nil {
				version = joinVersion(m)
				break
			}
		}
	}
	base = strings.Trim(base, " -_.")
	if base == "" {
		base = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	return strings.ToLower(base), version
}

func joinVersion(m []string) string {
	parts := []string{m[1], m[2]}
	if m[3] != "" {
		parts = append(parts, m[3])
	}
	return strings.Join(parts, ".")
}

func supported(path string, cfg Config) bool {
	if strings.HasPrefix(filepath.Base(path), "~$") {
		return false
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".pdf", ".docx", ".odt":
		return true
	case ".doc":
		return cfg.EnableLegacyDOC
	default:
		return false
	}
}

func syncDocs(root string, cfg Config) error {
	m, err := readManifest(root)
	if err != nil {
		return err
	}
	old := make(map[string]Document)
	for _, d := range m.Documents {
		old[strings.ToLower(filepath.Clean(d.Path))] = d
	}
	seen := make(map[string]bool)
	var docs []Document
	var failures int
	for _, source := range cfg.Sources {
		info, err := os.Stat(source)
		if err != nil || !info.IsDir() {
			fmt.Fprintln(os.Stderr, "warning: source unavailable:", source)
			for _, d := range m.Documents {
				if strings.EqualFold(d.Source, source) && !seen[strings.ToLower(d.Path)] {
					docs = append(docs, d)
					seen[strings.ToLower(d.Path)] = true
				}
			}
			continue
		}
		err = filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				fmt.Fprintln(os.Stderr, "warning:", walkErr)
				failures++
				return nil
			}
			if entry.IsDir() || !supported(path, cfg) {
				return nil
			}
			key := strings.ToLower(filepath.Clean(path))
			if seen[key] {
				return nil
			}
			seen[key] = true
			info, err := entry.Info()
			if err != nil {
				fmt.Fprintln(os.Stderr, "warning:", err)
				failures++
				return nil
			}
			oldDoc := old[key]
			hash, err := fileHash(path)
			if err != nil {
				fmt.Fprintln(os.Stderr, "warning:", err)
				failures++
				if oldDoc.ID != "" {
					docs = append(docs, oldDoc)
				}
				return nil
			}
			if oldDoc.ID != "" && oldDoc.SHA256 == hash && oldDoc.Status == "ready" {
				oldDoc.Size, oldDoc.Modified = info.Size(), info.ModTime().UTC().Format(time.RFC3339Nano)
				oldDoc.Family, oldDoc.Version = familyVersion(path)
				docs = append(docs, oldDoc)
				return nil
			}
			family, version := familyVersion(path)
			d := Document{ID: fileID(path), Path: path, Source: source, Format: strings.ToLower(strings.TrimPrefix(filepath.Ext(path), ".")), Family: family, Version: version, SHA256: hash, Size: info.Size(), Modified: info.ModTime().UTC().Format(time.RFC3339Nano), Status: "ready"}
			texts, err := extract(root, cfg, d)
			if err != nil {
				d.Status = "error"
				d.Error = err.Error()
				failures++
				fmt.Fprintln(os.Stderr, "warning:", path, err)
			} else {
				d.Texts = texts
				if d.Format == "pdf" {
					d.Pages = len(texts)
				}
				removeStale(root, oldDoc.Texts, texts)
			}
			docs = append(docs, d)
			return nil
		})
		if err != nil {
			return err
		}
	}
	for _, d := range m.Documents {
		if !seen[strings.ToLower(filepath.Clean(d.Path))] {
			removeStale(root, d.Texts, nil)
		}
	}
	sort.Slice(docs, func(i, j int) bool { return strings.ToLower(docs[i].Path) < strings.ToLower(docs[j].Path) })
	m.Documents, m.Updated = docs, time.Now().UTC().Format(time.RFC3339)
	if err := atomicJSON(filepath.Join(root, "data", "manifest.json"), m); err != nil {
		return err
	}
	fmt.Printf("sync: %d documents, %d errors; manifest: %s\n", len(docs), failures, filepath.Join(root, "data", "manifest.json"))
	return nil
}

func removeStale(root string, previous, current []string) {
	keep := make(map[string]bool)
	for _, p := range current {
		keep[p] = true
	}
	for _, p := range previous {
		if !keep[p] && strings.HasPrefix(filepath.Clean(p), "text"+string(filepath.Separator)) {
			_ = os.Remove(filepath.Join(root, "data", p))
		}
	}
}

func poppler(cfg Config, name string) string {
	if name == "pdftotext" && cfg.PDFToText != "" {
		return cfg.PDFToText
	}
	if cfg.PopplerBin == "" {
		return name
	}
	return filepath.Join(cfg.PopplerBin, name+".exe")
}

func extract(root string, cfg Config, d Document) ([]string, error) {
	var parts []string
	switch d.Format {
	case "pdf":
		out, err := exec.Command(poppler(cfg, "pdfinfo"), d.Path).CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("pdfinfo: %w: %s", err, strings.TrimSpace(string(out)))
		}
		m := pagesRE.FindSubmatch(out)
		if m == nil {
			return nil, errors.New("pdfinfo returned no page count")
		}
		count, _ := strconv.Atoi(string(m[1]))
		if count < 1 {
			return nil, errors.New("PDF has no pages")
		}
		out, err = exec.Command(poppler(cfg, "pdftotext"), "-layout", "-enc", "UTF-8", d.Path, "-").CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("pdftotext: %w: %s", err, strings.TrimSpace(string(out)))
		}
		parts = strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\f")
		if len(parts) > count {
			parts = parts[:count]
		}
		for len(parts) < count {
			parts = append(parts, "")
		}
	case "docx", "odt":
		text, err := extractZIPXML(d.Path, d.Format)
		if err != nil {
			return nil, err
		}
		parts = []string{text}
	case "doc":
		text, err := extractDOC(cfg, d.Path)
		if err != nil {
			return nil, err
		}
		parts = []string{text}
	default:
		return nil, fmt.Errorf("unsupported format %s", d.Format)
	}
	paths := make([]string, len(parts))
	for i, content := range parts {
		name := fmt.Sprintf("%s-p%04d.txt", d.ID, i+1)
		rel := filepath.Join("text", name)
		path := filepath.Join(root, "data", rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0644); err != nil {
			return nil, err
		}
		paths[i] = rel
	}
	return paths, nil
}

func extractZIPXML(path, format string) (string, error) {
	z, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer z.Close()
	want := "word/document.xml"
	if format == "odt" {
		want = "content.xml"
	}
	for _, f := range z.File {
		if f.Name != want {
			continue
		}
		if f.UncompressedSize64 > 128*1024*1024 {
			return "", errors.New("XML component exceeds 128 MiB")
		}
		r, err := f.Open()
		if err != nil {
			return "", err
		}
		defer r.Close()
		return xmlText(io.LimitReader(r, 128*1024*1024+1), format)
	}
	return "", fmt.Errorf("%s missing in ZIP", want)
}

func xmlText(r io.Reader, format string) (string, error) {
	dec := xml.NewDecoder(r)
	var b strings.Builder
	depth := 0
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if format == "docx" {
				if t.Name.Local == "p" {
					depth++
				}
				if depth > 0 && t.Name.Local == "tab" {
					b.WriteByte('\t')
				}
				if depth > 0 && t.Name.Local == "br" {
					b.WriteByte('\n')
				}
			} else {
				if t.Name.Local == "p" || t.Name.Local == "h" {
					depth++
				}
				if depth > 0 && t.Name.Local == "tab" {
					b.WriteByte('\t')
				}
				if depth > 0 && t.Name.Local == "line-break" {
					b.WriteByte('\n')
				}
			}
		case xml.CharData:
			if depth > 0 {
				b.Write([]byte(t))
			}
		case xml.EndElement:
			if (format == "docx" && t.Name.Local == "p") || (format == "odt" && (t.Name.Local == "p" || t.Name.Local == "h")) {
				if depth > 0 {
					depth--
					b.WriteByte('\n')
				}
			}
		}
	}
	return b.String(), nil
}

func extractDOC(cfg Config, path string) (string, error) {
	if cfg.LibreOffice == "" {
		return "", errors.New("libreoffice path required for DOC")
	}
	dir, err := os.MkdirTemp("", "codex-doc-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	profile := filepath.Join(dir, "profile")
	output := filepath.Join(dir, "output")
	if err := os.Mkdir(output, 0700); err != nil {
		return "", err
	}
	cmd := exec.Command(cfg.LibreOffice, "-env:UserInstallation=file:///"+filepath.ToSlash(profile), "--headless", "--convert-to", "txt:Text (encoded):UTF8", "--outdir", output, path)
	b, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("libreoffice: %w: %s", err, strings.TrimSpace(string(b)))
	}
	converted := filepath.Join(output, strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))+".txt")
	b, err = os.ReadFile(converted)
	if err != nil {
		return "", fmt.Errorf("libreoffice output: %w", err)
	}
	return string(b), nil
}

type rgEvent struct {
	Type string `json:"type"`
	Data struct {
		Path struct {
			Text string `json:"text"`
		} `json:"path"`
		Lines struct {
			Text string `json:"text"`
		} `json:"lines"`
		LineNumber int `json:"line_number"`
	} `json:"data"`
}

type Hit struct {
	Doc        Document
	Page, Line int
	Text       string
}

func preference(d Document) int {
	if d.Format == "pdf" && strings.HasSuffix(strings.ToLower(strings.TrimSuffix(filepath.Base(d.Path), filepath.Ext(d.Path))), "-nokey") {
		return 0
	}
	if d.Format == "pdf" {
		return 1
	}
	if d.Format == "odt" || d.Format == "docx" {
		return 2
	}
	return 3
}

func searchDocs(root, query string, all bool, limit int) error {
	if limit < 1 {
		return errors.New("limit must be positive")
	}
	m, err := readManifest(root)
	if err != nil {
		return err
	}
	byText := make(map[string]struct {
		doc  Document
		page int
	})
	for _, d := range m.Documents {
		if d.Status != "ready" {
			continue
		}
		for i, p := range d.Texts {
			byText[strings.ToLower(filepath.Clean(filepath.Join(root, "data", p)))] = struct {
				doc  Document
				page int
			}{d, i + 1}
		}
	}
	if len(byText) == 0 {
		return errors.New("empty index; run sync")
	}
	cmd := exec.Command("rg", "--json", "--ignore-case", "--fixed-strings", "--", query, filepath.Join(root, "data", "text"))
	b, err := cmd.Output()
	if err != nil {
		var ex *exec.ExitError
		if errors.As(err, &ex) && ex.ExitCode() == 1 {
			return nil
		}
		return fmt.Errorf("rg: %w", err)
	}
	var hits []Hit
	scan := bufio.NewScanner(strings.NewReader(string(b)))
	scan.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for scan.Scan() {
		var event rgEvent
		if json.Unmarshal(scan.Bytes(), &event) != nil || event.Type != "match" {
			continue
		}
		ref, ok := byText[strings.ToLower(filepath.Clean(event.Data.Path.Text))]
		if !ok {
			continue
		}
		hits = append(hits, Hit{Doc: ref.doc, Page: ref.page, Line: event.Data.LineNumber, Text: strings.TrimSpace(event.Data.Lines.Text)})
	}
	if err := scan.Err(); err != nil {
		return err
	}
	if !all {
		best := make(map[string]int)
		for _, h := range hits {
			key := h.Doc.Family + "\x00" + h.Doc.Version
			p := preference(h.Doc)
			if current, ok := best[key]; !ok || p < current {
				best[key] = p
			}
		}
		filtered := hits[:0]
		for _, h := range hits {
			if preference(h.Doc) == best[h.Doc.Family+"\x00"+h.Doc.Version] {
				filtered = append(filtered, h)
			}
		}
		hits = filtered
	}
	sort.Slice(hits, func(i, j int) bool {
		a, b := hits[i], hits[j]
		if a.Doc.Family != b.Doc.Family {
			return a.Doc.Family < b.Doc.Family
		}
		if a.Doc.Version != b.Doc.Version {
			return a.Doc.Version > b.Doc.Version
		}
		if preference(a.Doc) != preference(b.Doc) {
			return preference(a.Doc) < preference(b.Doc)
		}
		if a.Doc.Path != b.Doc.Path {
			return a.Doc.Path < b.Doc.Path
		}
		if a.Page != b.Page {
			return a.Page < b.Page
		}
		return a.Line < b.Line
	})
	for i, h := range hits {
		if i >= limit {
			fmt.Printf("... %d further hits (increase --limit)\n", len(hits)-limit)
			break
		}
		loc := fmt.Sprintf("line %d", h.Line)
		if h.Doc.Format == "pdf" {
			loc = fmt.Sprintf("page %d, line %d", h.Page, h.Line)
		}
		fmt.Printf("%s\t%s\t%s\t%s\t%s\n  %s\n", h.Doc.ID, h.Doc.Version, loc, h.Doc.Format, h.Doc.Path, h.Text)
	}
	return nil
}

func viewPage(root string, cfg Config, id string, page int) error {
	m, err := readManifest(root)
	if err != nil {
		return err
	}
	for _, d := range m.Documents {
		if d.ID != id {
			continue
		}
		if d.Format != "pdf" || d.Status != "ready" {
			return errors.New("document is not a ready PDF")
		}
		if page > d.Pages {
			return fmt.Errorf("page %d exceeds %d pages", page, d.Pages)
		}
		viewDir := filepath.Join(root, "data", "views")
		if err := os.MkdirAll(viewDir, 0755); err != nil {
			return err
		}
		prefix := filepath.Join(viewDir, fmt.Sprintf("%s-%s-p%d", d.ID, d.SHA256[:12], page))
		output := prefix + ".png"
		if _, err := os.Stat(output); err == nil {
			fmt.Println(output)
			return nil
		}
		b, err := exec.Command(poppler(cfg, "pdftoppm"), "-f", strconv.Itoa(page), "-l", strconv.Itoa(page), "-singlefile", "-r", "120", "-png", d.Path, prefix).CombinedOutput()
		if err != nil {
			return fmt.Errorf("pdftoppm: %w: %s", err, strings.TrimSpace(string(b)))
		}
		fmt.Println(output)
		return nil
	}
	return fmt.Errorf("document ID %s not found", id)
}

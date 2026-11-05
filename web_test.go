package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testSite() http.Handler {
	mux := http.NewServeMux()
	page := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(body))
		}
	}
	mux.HandleFunc("/kb/", page(`<html><head><title>Start</title><style>.x{}</style></head><body><nav><a href="/kb/nav-only">n</a></nav><p>Willkommen <b>Wissen</b></p><script>secret()</script><a href="a.html">A</a> <a href="/outside/x.html">out</a> <a href="https://example.org/">ext</a></body></html>`))
	mux.HandleFunc("/kb/a.html", page(`<html><title>Seite A</title><body><p>Inhalt Alpha</p><a href="/kb/">zurueck</a><a href="../outside/y.html">up</a></body></html>`))
	mux.HandleFunc("/kb/nav-only", page(`<html><body>Nur ueber Navigation</body></html>`))
	mux.HandleFunc("/outside/x.html", page(`<html><body>DRAUSSEN</body></html>`))
	mux.HandleFunc("/outside/y.html", page(`<html><body>DRAUSSEN</body></html>`))
	return mux
}

func TestCrawlStaysInPrefix(t *testing.T) {
	webDelay = 0
	srv := httptest.NewServer(testSite())
	defer srv.Close()
	pages, err := crawl(WebSource{URL: srv.URL + "/kb/"})
	if err != nil {
		t.Fatal(err)
	}
	all := ""
	urls := map[string]bool{}
	for _, p := range pages {
		all += p.Text + "\n"
		urls[strings.TrimPrefix(p.URL, srv.URL)] = true
	}
	for _, want := range []string{"/kb/", "/kb/a.html", "/kb/nav-only"} {
		if !urls[want] {
			t.Errorf("missing page %s in %v", want, urls)
		}
	}
	if len(pages) != 3 {
		t.Errorf("got %d pages, want 3", len(pages))
	}
	if strings.Contains(all, "DRAUSSEN") || strings.Contains(all, "secret") || strings.Contains(all, ".x{}") {
		t.Errorf("unexpected content: %q", all)
	}
	if !strings.Contains(all, "Start\nWillkommen Wissen") || !strings.Contains(all, "Seite A") {
		t.Errorf("title/text missing: %q", all)
	}
}

func TestCrawlMaxPages(t *testing.T) {
	webDelay = 0
	srv := httptest.NewServer(testSite())
	defer srv.Close()
	pages, err := crawl(WebSource{URL: srv.URL + "/kb/", MaxPages: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 2 {
		t.Errorf("got %d pages, want 2", len(pages))
	}
}

func TestCrawlBasicAuth(t *testing.T) {
	webDelay = 0
	site := testSite()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, p, ok := r.BasicAuth(); !ok || u != "user" || p != "pw" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		site.ServeHTTP(w, r)
	}))
	defer srv.Close()
	if _, err := crawl(WebSource{URL: srv.URL + "/kb/"}); err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected 401 error, got %v", err)
	}
	pages, err := crawl(WebSource{URL: srv.URL + "/kb/", Username: "user", Password: "pw"})
	if err != nil || len(pages) != 3 {
		t.Fatalf("pages=%d err=%v", len(pages), err)
	}
}

func TestCrawlDoesNotFollowRedirectOutOfScope(t *testing.T) {
	webDelay = 0
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("foreign host was contacted, auth=%q", r.Header.Get("Authorization"))
	}))
	defer other.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/", http.StatusFound)
	}))
	defer srv.Close()
	if _, err := crawl(WebSource{URL: srv.URL + "/kb/", Username: "u", Password: "p"}); err == nil {
		t.Fatal("expected error for out-of-scope redirect")
	}
}

func TestScopePrefix(t *testing.T) {
	cases := map[string]string{"": "/", "/": "/", "/kb": "/kb/", "/kb/": "/kb/", "/kb/index.html": "/kb/", "/index.html": "/"}
	for in, want := range cases {
		if got := scopePrefix(in); got != want {
			t.Errorf("scopePrefix(%q)=%q want %q", in, got, want)
		}
	}
}

func TestSyncWebSourceKeepsOnOutageAndDropsRemoved(t *testing.T) {
	webDelay = 0
	root := t.TempDir()
	srv := httptest.NewServer(testSite())
	ws, err := normalizeWebSource(WebSource{URL: srv.URL + "/kb/"})
	if err != nil {
		t.Fatal(err)
	}
	docs, failures := syncWebSource(root, ws, nil)
	if len(docs) != 3 || failures != 0 {
		t.Fatalf("docs=%d failures=%d", len(docs), failures)
	}
	for _, d := range docs {
		if !isWebDoc(d) || d.Format != "html" || len(d.Texts) != 1 {
			t.Fatalf("bad doc %+v", d)
		}
		if _, err := os.Stat(filepath.Join(root, "data", d.Texts[0])); err != nil {
			t.Fatal(err)
		}
	}
	// unchanged re-sync reuses entries
	again, _ := syncWebSource(root, ws, docs)
	if len(again) != 3 || again[0].Modified != docs[0].Modified {
		t.Fatalf("expected reuse, got %+v", again)
	}
	// outage keeps old documents
	srv.Close()
	kept, _ := syncWebSource(root, ws, docs)
	if len(kept) != 3 {
		t.Fatalf("outage dropped documents: %d", len(kept))
	}
}

func TestConfigWebSourceOnly(t *testing.T) {
	root := t.TempDir()
	write := func(s string) {
		if err := os.WriteFile(filepath.Join(root, "config.json"), []byte(s), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(`{"web_sources":[{"url":"https://KB.example.com/docs#x"}]}`)
	cfg, err := readConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WebSources[0].URL != "https://kb.example.com/docs" || cfg.WebSources[0].MaxPages != defaultMaxPages {
		t.Errorf("%+v", cfg.WebSources[0])
	}
	for _, bad := range []string{`{"web_sources":[{"url":"ftp://x/"}]}`, `{"web_sources":[{"url":"https://u:p@x/"}]}`, `{}`} {
		write(bad)
		if _, err := readConfig(root); err == nil {
			t.Errorf("expected error for %s", bad)
		}
	}
}

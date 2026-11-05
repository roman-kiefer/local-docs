package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const (
	defaultMaxPages = 500
	maxPageBytes    = 5 * 1024 * 1024
)

// webDelay is the pause between two requests to the same source.
var webDelay = 100 * time.Millisecond

type WebSource struct {
	URL      string `json:"url"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	MaxPages int    `json:"max_pages,omitempty"`
}

type webPage struct {
	URL  string
	Text string
}

// normalizeWebSource validates a configured web source and returns it with a
// cleaned URL (no fragment, lowercase host).
func normalizeWebSource(ws WebSource) (WebSource, error) {
	u, err := url.Parse(strings.TrimSpace(ws.URL))
	if err != nil {
		return ws, fmt.Errorf("invalid web source URL %q: %w", ws.URL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ws, fmt.Errorf("web source must be http or https: %q", ws.URL)
	}
	if u.Host == "" {
		return ws, fmt.Errorf("web source has no host: %q", ws.URL)
	}
	if u.User != nil {
		return ws, errors.New("web source URL must not contain credentials; use username/password")
	}
	if ws.MaxPages < 0 {
		return ws, errors.New("max_pages must not be negative")
	}
	if ws.MaxPages == 0 {
		ws.MaxPages = defaultMaxPages
	}
	u.Fragment = ""
	u.Host = strings.ToLower(u.Host)
	ws.URL = u.String()
	return ws, nil
}

// scopePrefix returns the path prefix that bounds the crawl. A start path
// whose last segment looks like a file (index.html) is bounded by its folder.
func scopePrefix(p string) string {
	if p == "" {
		return "/"
	}
	if strings.HasSuffix(p, "/") {
		return p
	}
	i := strings.LastIndex(p, "/")
	if strings.Contains(p[i+1:], ".") {
		return p[:i+1]
	}
	return p + "/"
}

func urlID(u string) string {
	h := sha256.Sum256([]byte(u))
	return hex.EncodeToString(h[:12])
}

func isWebDoc(d Document) bool {
	return strings.HasPrefix(d.Source, "http://") || strings.HasPrefix(d.Source, "https://")
}

type crawler struct {
	ws     WebSource
	host   string
	prefix string
	client *http.Client
}

func (c *crawler) inScope(u *url.URL) bool {
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	if !strings.EqualFold(u.Host, c.host) {
		return false
	}
	return u.Path == strings.TrimSuffix(c.prefix, "/") || strings.HasPrefix(u.Path, c.prefix)
}

// crawl fetches all pages below the source URL. An error means the source as
// a whole is unusable (unreachable, wrong credentials); pages that cannot be
// fetched are skipped with a warning.
func crawl(ws WebSource) ([]webPage, error) {
	start, err := url.Parse(ws.URL)
	if err != nil {
		return nil, err
	}
	max := ws.MaxPages
	if max <= 0 {
		max = defaultMaxPages
	}
	c := &crawler{ws: ws, host: start.Host, prefix: scopePrefix(start.Path)}
	c.client = &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("too many redirects")
			}
			if !c.inScope(req.URL) {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	queue := []*url.URL{start}
	visited := map[string]bool{start.String(): true}
	var pages []webPage
	for fetched := 0; len(queue) > 0 && fetched < max; fetched++ {
		u := queue[0]
		queue = queue[1:]
		if fetched > 0 && webDelay > 0 {
			time.Sleep(webDelay)
		}
		text, links, err := c.fetch(u, fetched == 0)
		if err != nil {
			if fetched == 0 {
				return nil, err
			}
			fmt.Fprintln(os.Stderr, "warning:", u.String(), err)
			continue
		}
		if text == "" && links == nil {
			continue // not an HTML/text resource
		}
		if text != "" {
			pages = append(pages, webPage{URL: u.String(), Text: text})
		}
		for _, l := range links {
			l.Fragment = ""
			l.Host = strings.ToLower(l.Host)
			if !c.inScope(l) || visited[l.String()] {
				continue
			}
			visited[l.String()] = true
			queue = append(queue, l)
		}
	}
	if len(queue) > 0 {
		fmt.Fprintf(os.Stderr, "warning: %s: page limit %d reached, further pages skipped\n", ws.URL, max)
	}
	return pages, nil
}

func (c *crawler) fetch(u *url.URL, first bool) (string, []*url.URL, error) {
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("User-Agent", "local-docs")
	req.Header.Set("Accept", "text/html, text/plain;q=0.8")
	if c.ws.Username != "" || c.ws.Password != "" {
		req.SetBasicAuth(c.ws.Username, c.ws.Password)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if first && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) {
			return "", nil, fmt.Errorf("HTTP %d: credentials missing or wrong (username/password in config.json)", resp.StatusCode)
		}
		return "", nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	ctype := strings.ToLower(resp.Header.Get("Content-Type"))
	isHTML := strings.HasPrefix(ctype, "text/html") || strings.HasPrefix(ctype, "application/xhtml")
	isText := strings.HasPrefix(ctype, "text/plain")
	if !isHTML && !isText {
		return "", nil, nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPageBytes+1))
	if err != nil {
		return "", nil, err
	}
	if len(body) > maxPageBytes {
		return "", nil, errors.New("page exceeds 5 MiB")
	}
	if isText {
		return normalizeText(string(body)), []*url.URL{}, nil
	}
	text, hrefs, err := htmlText(string(body))
	if err != nil {
		return "", nil, err
	}
	links := []*url.URL{}
	for _, h := range hrefs {
		ref, err := url.Parse(strings.TrimSpace(h))
		if err != nil {
			continue
		}
		links = append(links, resp.Request.URL.ResolveReference(ref))
	}
	return text, links, nil
}

var skipTags = map[string]bool{"script": true, "style": true, "noscript": true, "template": true, "nav": true, "head": true, "svg": true}
var blockTags = map[string]bool{"p": true, "div": true, "br": true, "li": true, "ul": true, "ol": true, "tr": true, "table": true, "section": true, "article": true, "header": true, "footer": true, "main": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true, "pre": true, "blockquote": true, "dt": true, "dd": true, "form": true}

// htmlText returns the readable text of an HTML document (title first) and
// all link targets, including those inside skipped elements such as nav.
func htmlText(src string) (string, []string, error) {
	doc, err := html.Parse(strings.NewReader(src))
	if err != nil {
		return "", nil, err
	}
	var b strings.Builder
	var links []string
	title := ""
	var walk func(n *html.Node, skip bool)
	walk = func(n *html.Node, skip bool) {
		switch n.Type {
		case html.TextNode:
			if !skip {
				b.WriteString(n.Data)
			}
			return
		case html.ElementNode:
			if n.Data == "a" {
				for _, a := range n.Attr {
					if a.Key == "href" {
						links = append(links, a.Val)
					}
				}
			}
			if n.Data == "title" && title == "" {
				title = nodeText(n)
			}
			if skipTags[n.Data] {
				skip = true
			}
			if !skip && blockTags[n.Data] {
				b.WriteString("\n")
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, skip)
		}
		if n.Type == html.ElementNode && !skip && blockTags[n.Data] {
			b.WriteString("\n")
		}
	}
	walk(doc, false)
	text := normalizeText(b.String())
	if t := strings.Join(strings.Fields(title), " "); t != "" {
		text = t + "\n" + text
	}
	return strings.TrimSpace(text), links, nil
}

func nodeText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

// normalizeText collapses whitespace within lines and drops empty lines.
func normalizeText(s string) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), " ", " ")
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line = strings.Join(strings.Fields(line), " "); line != "" {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

// syncWebSource crawls one source and returns its documents. If the source is
// unavailable, previously indexed pages are kept.
func syncWebSource(root string, ws WebSource, previous []Document) ([]Document, int) {
	var mine []Document
	old := make(map[string]Document)
	for _, d := range previous {
		if d.Source == ws.URL {
			mine = append(mine, d)
			old[d.Path] = d
		}
	}
	pages, err := crawl(ws)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: web source unavailable: %s: %v\n", ws.URL, err)
		return mine, 0
	}
	var docs []Document
	failures := 0
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, p := range pages {
		sum := sha256.Sum256([]byte(p.Text))
		hash := hex.EncodeToString(sum[:])
		if o, ok := old[p.URL]; ok && o.SHA256 == hash && o.Status == "ready" {
			docs = append(docs, o)
			continue
		}
		d := Document{ID: urlID(p.URL), Path: p.URL, Source: ws.URL, Format: "html", Family: p.URL, SHA256: hash, Size: int64(len(p.Text)), Modified: now, Status: "ready"}
		texts, err := writeParts(root, d.ID, []string{p.Text})
		if err != nil {
			d.Status, d.Error = "error", err.Error()
			fmt.Fprintln(os.Stderr, "warning:", p.URL, err)
			failures++
		} else {
			d.Texts = texts
			removeStale(root, old[p.URL].Texts, texts)
		}
		docs = append(docs, d)
	}
	return docs, failures
}

// Mdv serves rendered Markdown from the current directory on localhost:8780.
//
// Usage:
//
//	mdv [-a addr] [-r root]
//	mdv [-a addr] path
//
// The -a flag sets a different service address (default localhost:8780).
//
// The -r flag sets a different root directory to serve (default current directory).
//
// If path is given, mdv opens it in the browser. The serve root stays
// the current directory (or -r), so links to other Markdown files in the
// tree still resolve. A path outside the root becomes the new root.
//
// Files named *-talk.md are served as slide presentations, using an
// embedded style sheet and script: each # heading starts a new slide,
// and anything after a --- rule on a slide is speaker notes.
// Press ? in the browser for the list of presentation keys.
//
// Other Markdown files are served as a reading page that follows the
// system light or dark appearance.
//
// If the first line of the first slide's notes is a line like
//
//	time: 20m
//
// then the presenter view schedules the talk, dividing the time
// among the slides in proportion to the length of their notes.
package main

import (
	_ "embed"
	"flag"
	"fmt"
	"html"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"time"

	"rsc.io/markdown"
)

// Slides mode, for files named *-talk.md: each # heading starts a new slide.
var (
	//go:embed slides.css
	slidesCSS string

	//go:embed slides.js
	slidesJS string

	//go:embed read.css
	readCSS string

	//go:embed read.js
	readJS string
)

var (
	addr = flag.String("a", "localhost:8780", "serve HTTP requests on `addr`")
	root = flag.String("r", ".", "set `root` directory for serving content")

	dir        http.FileSystem
	fileServer http.Handler

	// start is the process start time. Markdown pages embed CSS (and talks,
	// JavaScript) from the binary, so they are only as old as the binary.
	start = time.Now()
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: mdv [-a addr] [-r root]\n")
	fmt.Fprintf(os.Stderr, "       mdv [-a addr] path\n")
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	log.SetPrefix("mdv: ")
	log.SetFlags(0)

	flag.Usage = usage
	flag.Parse()
	rootSet := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "r" {
			rootSet = true
		}
	})
	open := "/"
	switch flag.NArg() {
	case 0:
	case 1:
		open = resolvePath(flag.Arg(0), rootSet)
	default:
		usage()
	}

	dir = http.Dir(*root)
	fileServer = http.FileServer(dir)
	http.HandleFunc("/", md)
	fmt.Fprintf(os.Stderr, "mdv: serving %s on http://%s\n", *root, *addr)
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	go openWhenReady(*addr, open)
	log.Fatal(http.Serve(ln, nil))
}

// resolvePath sets *root from p if needed and returns the URL path to open.
func resolvePath(p string, rootSet bool) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		log.Fatal(err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		log.Fatal(err)
	}
	rootAbs, err := filepath.Abs(*root)
	if err != nil {
		log.Fatal(err)
	}
	rel, ok := under(rootAbs, abs)
	if !ok {
		if rootSet {
			log.Fatalf("%s is not under %s", abs, *root)
		}
		if info.IsDir() {
			*root = abs
			return "/"
		}
		*root = filepath.Dir(abs)
		return "/" + url.PathEscape(filepath.Base(abs))
	}
	u := urlPath(rel)
	if info.IsDir() && !strings.HasSuffix(u, "/") {
		u += "/"
	}
	return u
}

// under reports whether file is inside root, returning the relative path.
func under(root, file string) (string, bool) {
	rel, err := filepath.Rel(root, file)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
}

// open opens name in dir. If name is missing, a leading /<root-basename>/
// is stripped so URLs from a parent tree still resolve.
func open(name string) (http.File, error) {
	f, err := dir.Open(name)
	if err == nil {
		return f, nil
	}
	base := filepath.Base(*root)
	if base == "" || base == "." || base == string(filepath.Separator) {
		return nil, err
	}
	prefix := "/" + base
	p := path.Clean("/" + strings.TrimPrefix(name, "/"))
	if p != prefix && !strings.HasPrefix(p, prefix+"/") {
		return nil, err
	}
	rel := strings.TrimPrefix(p, prefix)
	if rel == "" {
		rel = "/"
	}
	return dir.Open(rel)
}

// urlPath turns a filesystem-relative path into a rooted URL path.
func urlPath(rel string) string {
	if rel == "." || rel == "" {
		return "/"
	}
	rel = filepath.ToSlash(rel)
	parts := strings.Split(rel, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return "/" + strings.Join(parts, "/")
}

// openWhenReady waits until addr accepts TCP connections, then opens u.
func openWhenReady(addr, path string) {
	d := net.Dialer{Timeout: 100 * time.Millisecond}
	for i := 0; i < 50; i++ {
		c, err := d.Dial("tcp", addr)
		if err == nil {
			c.Close()
			openBrowser("http://" + addr + path)
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	log.Printf("opening browser: %s never accepted a connection", addr)
}

func md(w http.ResponseWriter, req *http.Request) {
	if req.Method != "GET" {
		http.Error(w, "bad method", http.StatusMethodNotAllowed)
		return
	}

	if !isMarkdown(req.URL.Path) {
		serveDir(w, req)
		return
	}

	f, err := open(req.URL.Path)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	modTime := info.ModTime()
	if modTime.Before(start) {
		modTime = start
	}
	if checkLastModified(w, req, modTime) {
		f.Close()
		return
	}

	data, err := io.ReadAll(f)
	f.Close()
	if err != nil {
		http.Error(w, "error reading data", http.StatusInternalServerError)
		return
	}

	p := &markdown.Parser{
		HeadingID:     true,
		Strikethrough: true,
		TaskList:      true,
		AutoLinkText:  true,
		Table:         true,
		Emoji:         true,
		SmartDot:      true,
		SmartDash:     true,
		SmartQuote:    true,
	}
	doc := p.Parse(string(data))
	setHeadingIDs(doc)
	body := markdown.ToHTML(doc)
	if isTalk(req.URL.Path) {
		slides(w, req, doc, body)
		return
	}
	page(w, req, doc, body)
}

// serveDir lists a directory as a reading page, or hands the request to
// the file server for anything that is not a directory.
func serveDir(w http.ResponseWriter, req *http.Request) {
	f, err := open(req.URL.Path)
	if err != nil {
		fileServer.ServeHTTP(w, req)
		return
	}
	info, err := f.Stat()
	if err != nil || !info.IsDir() {
		f.Close()
		fileServer.ServeHTTP(w, req)
		return
	}
	if !strings.HasSuffix(req.URL.Path, "/") {
		loc := path.Base(req.URL.Path) + "/"
		if q := req.URL.RawQuery; q != "" {
			loc += "?" + q
		}
		f.Close()
		w.Header().Set("Location", loc)
		w.WriteHeader(http.StatusMovedPermanently)
		return
	}
	infos, err := f.Readdir(-1)
	f.Close()
	if err != nil {
		http.Error(w, "error reading directory", http.StatusInternalServerError)
		return
	}
	listing(w, req, infos)
}

// markdownExts are the file extensions served as rendered Markdown.
var markdownExts = []string{".md", ".markdown", ".mdown", ".mkd", ".mkdn"}

// isMarkdown reports whether p names a Markdown file.
func isMarkdown(p string) bool {
	return slices.Contains(markdownExts, strings.ToLower(path.Ext(p)))
}

// trimExt removes the extension from the base name of p.
func trimExt(p string) string {
	base := path.Base(p)
	return strings.TrimSuffix(base, path.Ext(base))
}

// isTalk reports whether p names a slide presentation.
func isTalk(p string) bool {
	return strings.HasSuffix(trimExt(p), "-talk")
}

// setHeadingIDs gives each heading an HTML id, so that links to a #fragment
// within this file or from another file land on the heading. Headings that
// carry an explicit {#id} keep it.
func setHeadingIDs(doc *markdown.Document) {
	seen := make(map[string]int)
	for _, b := range doc.Blocks {
		h, ok := b.(*markdown.Heading)
		if !ok {
			continue
		}
		id := h.ID
		if id == "" {
			id = slug(textOnly(markdown.ToHTML(h)))
		}
		if id == "" {
			continue
		}
		if n := seen[id]; n > 0 {
			seen[id] = n + 1
			id = fmt.Sprintf("%s-%d", id, n)
		} else {
			seen[id] = 1
		}
		h.ID = id
	}
}

// slug turns heading text into a fragment id, the way GitHub does:
// lowercase, spaces to hyphens, punctuation dropped.
func slug(text string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_', r > 127:
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('-')
		}
	}
	return b.String()
}

// slides writes doc's rendered HTML wrapped in the slide CSS and JavaScript,
// which split the document into one slide per # heading.
func slides(w http.ResponseWriter, req *http.Request, doc *markdown.Document, body string) {
	title := docTitle(doc)
	if title == "" {
		title = trimExt(req.URL.Path)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>%s</title>
<style>
%s</style>
</head>
<body>
<div id="deck">
%s</div>
<div id="status"></div>
<div id="notes"></div>
<div id="pageno"></div>
<div id="msg"></div>
<div id="help">%s</div>
<script>
%s</script>
</body>
</html>
`, html.EscapeString(title), slidesCSS, body, helpHTML, slidesJS)
}

// page writes doc's rendered HTML as a reading page.
func page(w http.ResponseWriter, req *http.Request, doc *markdown.Document, body string) {
	title := docTitle(doc)
	if title == "" {
		title = trimExt(req.URL.Path)
	}
	readPage(w, title, filesNav(req.URL.Path), body, false)
}

// listing writes a directory index of Markdown files and subdirectories.
func listing(w http.ResponseWriter, req *http.Request, infos []fs.FileInfo) {
	sort.Slice(infos, func(i, j int) bool {
		if infos[i].IsDir() != infos[j].IsDir() {
			return infos[i].IsDir()
		}
		return infos[i].Name() < infos[j].Name()
	})
	base := req.URL.Path
	var b strings.Builder
	b.WriteString("<h1>Files</h1>\n<ul class=\"index\">\n")
	if base != "/" {
		parent := path.Dir(strings.TrimSuffix(base, "/"))
		if parent != "/" {
			parent += "/"
		}
		fmt.Fprintf(&b, `<li><a href="%s"><span class="kind">dir</span>..</a></li>`, hrefPath(parent))
		b.WriteByte('\n')
	}
	for _, info := range infos {
		name := info.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		p := path.Join(base, name)
		kind := "file"
		mtime := ""
		if info.IsDir() {
			p += "/"
			kind = "dir"
		} else if !isMarkdown(name) {
			continue
		} else {
			mtime = fmt.Sprintf(` data-mtime="%d"`, info.ModTime().Unix())
		}
		fmt.Fprintf(&b, `<li><a href="%s"%s><span class="kind">%s</span>%s</a></li>`,
			hrefPath(p), mtime, kind, html.EscapeString(name))
		b.WriteByte('\n')
	}
	b.WriteString("</ul>\n")
	title := path.Base(strings.TrimSuffix(base, "/"))
	if base == "/" {
		title = filepath.Base(*root)
	}
	readPage(w, title, filesNav(base), b.String(), true)
}

func hrefPath(p string) string {
	if p == "" || p[0] != '/' {
		p = "/" + p
	}
	return html.EscapeString((&url.URL{Path: p}).String())
}

// mdFile is one Markdown file found while walking the served root.
type mdFile struct {
	href    string // URL path
	rel     string // slash-separated path relative to root, for display and name sort
	modTime time.Time
}

// markdownFiles walks the whole served root and returns every Markdown file,
// skipping dot files and directories.
func markdownFiles() []mdFile {
	rootAbs, err := filepath.Abs(*root)
	if err != nil {
		return nil
	}
	var files []mdFile
	filepath.WalkDir(rootAbs, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if p != rootAbs && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() || !isMarkdown(p) {
			return nil
		}
		rel, err := filepath.Rel(rootAbs, p)
		if err != nil {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		files = append(files, mdFile{
			href:    urlPath(rel),
			rel:     filepath.ToSlash(rel),
			modTime: info.ModTime(),
		})
		return nil
	})
	return files
}

// filesNav lists every Markdown file in the served tree, for the reading
// page's file switcher. Client-side script sorts and filters it further.
func filesNav(urlPath string) string {
	files := markdownFiles()
	sort.Slice(files, func(i, j int) bool { return files[i].rel < files[j].rel })
	cur := path.Clean(urlPath)
	var b strings.Builder
	b.WriteString(`<div class="label">Files<button type="button" id="files-sort" title="Sort files">Name</button></div>`)
	for _, f := range files {
		class := ""
		if path.Clean(f.href) == cur {
			class = ` class="current"`
		}
		fmt.Fprintf(&b, `<a href="%s" data-mtime="%d"%s>%s</a>`,
			hrefPath(f.href), f.modTime.Unix(), class, html.EscapeString(f.rel))
	}
	return b.String()
}

func readPage(w http.ResponseWriter, title, files, body string, listing bool) {
	bodyAttr := ""
	if listing {
		bodyAttr = ` data-listing="1"`
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="color-scheme" content="light dark">
<title>%s</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=JetBrains+Mono:ital,wght@0,400;0,500;1,400&family=Newsreader:ital,opsz,wght@0,6..72,400;0,6..72,500;0,6..72,600;1,6..72,400;1,6..72,500&display=swap" rel="stylesheet">
<style>
%s</style>
</head>
<body%s>
<div id="progress"><i></i></div>
<header id="top">
<a class="brand" href="/">mdv</a>
<div class="crumb">%s</div>
<div id="tools">
<button type="button" id="files-toggle" title="Show/hide files">Files</button>
<button type="button" id="nav-prev" title="Previous file">&larr;</button>
<button type="button" id="nav-next" title="Next file">&rarr;</button>
<div class="rule"></div>
<button type="button" id="font-dec" title="Smaller type">A−</button>
<button type="button" id="font-inc" title="Larger type">A+</button>
<div class="rule"></div>
<button type="button" id="measure" title="Column width"><span id="measure-label">48</span></button>
<div class="rule"></div>
<button type="button" class="swatch paper" id="theme-light" title="Paper"></button>
<button type="button" class="swatch sepia" id="theme-sepia" title="Sepia"></button>
<button type="button" class="swatch ink" id="theme-dark" title="Dark"></button>
</div>
</header>
<div class="cols">
<nav id="files">%s</nav>
<main>
<article>
%s</article>
</main>
<nav id="toc"></nav>
</div>
<script>
%s</script>
</body>
</html>
`, html.EscapeString(title), readCSS, bodyAttr, html.EscapeString(title), files, body, readJS)
}

// openBrowser opens u in the user's web browser.
func openBrowser(u string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", u)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("opening browser: %v", err)
		return
	}
	go cmd.Wait()
}

const helpHTML = `<table>
<tr><td><kbd>&rarr;</kbd> <kbd>space</kbd> <kbd>n</kbd></td><td>next slide</td></tr>
<tr><td><kbd>&larr;</kbd> <kbd>p</kbd></td><td>previous slide</td></tr>
<tr><td><kbd>home</kbd> <kbd>end</kbd></td><td>first, last slide</td></tr>
<tr><td><kbd>o</kbd> <kbd>esc</kbd></td><td>slide overview</td></tr>
<tr><td><kbd>f</kbd></td><td>full screen</td></tr>
<tr><td><kbd>P</kbd></td><td>present: slides in a new window, notes here</td></tr>
<tr><td><kbd>R</kbd></td><td>reset the talk timer</td></tr>
<tr><td><kbd>?</kbd></td><td>this help</td></tr>
</table>`

// docTitle returns the plain text of the document's first heading, if any.
func docTitle(doc *markdown.Document) string {
	for _, b := range doc.Blocks {
		if h, ok := b.(*markdown.Heading); ok {
			return textOnly(markdown.ToHTML(h))
		}
	}
	return ""
}

// textOnly returns the HTML fragment h with its tags removed.
func textOnly(h string) string {
	var buf strings.Builder
	for {
		i := strings.IndexByte(h, '<')
		if i < 0 {
			break
		}
		buf.WriteString(h[:i])
		j := strings.IndexByte(h[i:], '>')
		if j < 0 {
			return html.UnescapeString(strings.TrimSpace(buf.String()))
		}
		h = h[i+j+1:]
	}
	buf.WriteString(h)
	return html.UnescapeString(strings.TrimSpace(buf.String()))
}

// copied from net/http

var unixEpochTime = time.Unix(0, 0)

// modtime is the modification time of the resource to be served, or IsZero().
// return value is whether this request is now complete.
func checkLastModified(w http.ResponseWriter, r *http.Request, modtime time.Time) bool {
	if modtime.IsZero() || modtime.Equal(unixEpochTime) {
		// If the file doesn't have a modtime (IsZero), or the modtime
		// is obviously garbage (Unix time == 0), then ignore modtimes
		// and don't process the If-Modified-Since header.
		return false
	}

	// The Date-Modified header truncates sub-second precision, so
	// use mtime < t+1s instead of mtime <= t to check for unmodified.
	if t, err := time.Parse(http.TimeFormat, r.Header.Get("If-Modified-Since")); err == nil && modtime.Before(t.Add(1*time.Second)) {
		h := w.Header()
		delete(h, "Content-Type")
		delete(h, "Content-Length")
		w.WriteHeader(http.StatusNotModified)
		return true
	}
	w.Header().Set("Last-Modified", modtime.UTC().Format(http.TimeFormat))
	return false
}

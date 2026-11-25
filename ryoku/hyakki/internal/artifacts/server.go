package artifacts

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ryoku-i18n"
	"ryoku-rashin/internal/protocol"
)

const MaxBytes = 1 << 20
const idleTimeout = 15 * time.Minute

// Manager holds only previews the user has opened.
type Manager struct {
	mu       sync.Mutex
	previews map[string]*preview
}

type preview struct {
	mu                                                          sync.Mutex
	id, token, contentToken, session, origin, entry, code, kind string
	root                                                        *os.Root
	server                                                      *http.Server
	timer                                                       *time.Timer
	allowed                                                     func() bool
	errors                                                      []protocol.ArtifactError
	closed                                                      bool
}

func randomToken() (string, error) {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func (m *Manager) Open(ctx context.Context, p protocol.ArtifactOpenParams, allowed func() bool) (protocol.ArtifactOpenResult, error) {
	if err := ctx.Err(); err != nil {
		return protocol.ArtifactOpenResult{}, err
	}
	if p.Session == "" || (p.Path == "") == (p.Code == "") {
		return protocol.ArtifactOpenResult{}, errors.New(i18n.T("Choose one page or code block from a conversation."))
	}
	v := &preview{session: p.Session, code: p.Code, kind: strings.ToLower(p.Kind), allowed: allowed}
	if p.Path != "" {
		absolute, err := filepath.Abs(p.Path)
		if err != nil {
			return protocol.ArtifactOpenResult{}, err
		}
		v.kind = strings.TrimPrefix(strings.ToLower(filepath.Ext(absolute)), ".")
		v.entry = filepath.Base(absolute)
		v.root, err = os.OpenRoot(filepath.Dir(absolute))
		if err != nil {
			return protocol.ArtifactOpenResult{}, err
		}
		defer func() {
			if v.server == nil {
				_ = v.root.Close()
			}
		}()
		f, err := v.root.Open(v.entry)
		if err != nil {
			return protocol.ArtifactOpenResult{}, err
		}
		info, statErr := f.Stat()
		_ = f.Close()
		if statErr != nil || !info.Mode().IsRegular() || info.Size() > MaxBytes {
			return protocol.ArtifactOpenResult{}, errors.New(i18n.T("Preview pages must be regular files no larger than 1 MiB."))
		}
	} else {
		v.entry = "index.html"
	}
	if v.kind != "html" && v.kind != "svg" {
		return protocol.ArtifactOpenResult{}, errors.New(i18n.T("Preview an HTML or SVG page."))
	}
	if len(v.code) > MaxBytes {
		return protocol.ArtifactOpenResult{}, errors.New(i18n.T("Preview pages must be no larger than 1 MiB."))
	}
	var err error
	if v.id, err = randomToken(); err != nil {
		return protocol.ArtifactOpenResult{}, err
	}
	if v.token, err = randomToken(); err != nil {
		return protocol.ArtifactOpenResult{}, err
	}
	if v.contentToken, err = randomToken(); err != nil {
		return protocol.ArtifactOpenResult{}, err
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return protocol.ArtifactOpenResult{}, err
	}
	v.origin = "http://" + listener.Addr().String()
	v.server = &http.Server{Handler: v, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	m.mu.Lock()
	if m.previews == nil {
		m.previews = make(map[string]*preview)
	}
	enabled := allowed == nil || allowed()
	if len(m.previews) >= 16 || !enabled {
		m.mu.Unlock()
		_ = listener.Close()
		if v.root != nil {
			_ = v.root.Close()
		}
		if !enabled {
			return protocol.ArtifactOpenResult{}, errors.New(i18n.T("Turn on Artifacts in Settings first."))
		}
		return protocol.ArtifactOpenResult{}, errors.New(i18n.T("Close a preview before opening another."))
	}
	m.previews[v.id] = v
	v.timer = time.AfterFunc(idleTimeout, func() { m.Close(v.id) })
	m.mu.Unlock()
	go func() { _ = v.server.Serve(listener) }()
	return protocol.ArtifactOpenResult{ID: v.id, URL: v.origin + "/" + v.token + "/__view"}, nil
}

func (m *Manager) Session(id string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v := m.previews[id]
	if v == nil {
		return "", errors.New(i18n.T("This preview has closed. Open it again."))
	}
	return v.session, nil
}

func (m *Manager) Errors(id string) ([]protocol.ArtifactError, error) {
	m.mu.Lock()
	v := m.previews[id]
	m.mu.Unlock()
	if v == nil {
		return nil, errors.New(i18n.T("This preview has closed. Open it again."))
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	return append([]protocol.ArtifactError{}, v.errors...), nil
}

func (m *Manager) Close(id string) {
	m.mu.Lock()
	v := m.previews[id]
	delete(m.previews, id)
	m.mu.Unlock()
	if v != nil {
		v.close()
	}
}

func (m *Manager) CloseAll() {
	m.mu.Lock()
	previews := m.previews
	m.previews = nil
	m.mu.Unlock()
	for _, v := range previews {
		v.close()
	}
}

func (v *preview) close() {
	v.mu.Lock()
	v.closed = true
	if v.timer != nil {
		v.timer.Stop()
	}
	v.mu.Unlock()
	if v.server != nil {
		_ = v.server.Close()
	}
	if v.root != nil {
		_ = v.root.Close()
	}
}

func (m *Manager) Busy() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.previews) != 0
}

func sameToken(got, want string) bool {
	return got != "" && subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func (v *preview) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), display-capture=()")
	if r.Host != strings.TrimPrefix(v.origin, "http://") || (v.allowed != nil && !v.allowed()) {
		http.Error(w, "Preview unavailable", http.StatusForbidden)
		return
	}
	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/"), "/", 2)
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	trusted := sameToken(parts[0], v.token)
	child := sameToken(parts[0], v.contentToken)
	if !trusted && !child {
		http.NotFound(w, r)
		return
	}
	origin := r.Header.Get("Origin")
	if origin != "" && origin != v.origin && !(child && origin == "null") {
		http.Error(w, "Origin refused", http.StatusForbidden)
		return
	}
	if site := r.Header.Get("Sec-Fetch-Site"); trusted && site != "" && site != "same-origin" && site != "none" {
		http.Error(w, "Origin refused", http.StatusForbidden)
		return
	}
	v.mu.Lock()
	if v.closed {
		v.mu.Unlock()
		http.NotFound(w, r)
		return
	}
	if v.timer != nil {
		v.timer.Reset(idleTimeout)
	}
	v.mu.Unlock()
	if trusted && parts[1] == "__errors" {
		v.report(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if trusted {
		if parts[1] != "__view" {
			http.NotFound(w, r)
			return
		}
		v.wrapper(w, r)
		return
	}
	content := strings.SplitN(parts[1], "/", 2)
	if len(content) != 2 || (content[0] != "static" && content[0] != "run") {
		http.NotFound(w, r)
		return
	}
	run := content[0] == "run"
	name := content[1]
	if !fsName(name) {
		http.NotFound(w, r)
		return
	}
	var data []byte
	if v.root == nil {
		if name != v.entry {
			http.NotFound(w, r)
			return
		}
		data = []byte(v.code)
	} else {
		f, err := v.root.Open(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil || !info.Mode().IsRegular() || info.Size() > MaxBytes {
			http.NotFound(w, r)
			return
		}
		data, err = io.ReadAll(io.LimitReader(f, MaxBytes+1))
		if err != nil || len(data) > MaxBytes {
			http.Error(w, "Page too large", http.StatusRequestEntityTooLarge)
			return
		}
	}
	base := v.origin + "/" + v.contentToken + "/" + content[0] + "/"
	sandbox, scripts := "sandbox", "script-src 'none'"
	if run {
		sandbox, scripts = "sandbox allow-scripts", "script-src 'unsafe-inline' "+base
	}
	w.Header().Set("Content-Security-Policy", "default-src 'none'; "+sandbox+"; "+scripts+"; style-src 'unsafe-inline' "+base+"; img-src data: "+base+"; font-src "+base+"; media-src "+base+"; connect-src 'none'; frame-src 'none'; worker-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors "+v.origin)
	kind := strings.ToLower(path.Ext(name))
	if name == v.entry && v.kind == "svg" {
		kind = ".svg"
	}
	if kind == ".html" || kind == ".svg" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if kind == ".svg" {
			text := strings.TrimSpace(string(data))
			if strings.HasPrefix(text, "<?xml") {
				if end := strings.Index(text, "?>"); end >= 0 {
					text = text[end+2:]
				}
			}
			data = []byte("<!doctype html><meta charset=\"utf-8\"><body>" + text + "</body>")
		}
		if r.Method != http.MethodHead {
			if run {
				_, _ = io.WriteString(w, injectReporter(string(data), v.reporter()))
			} else {
				_, _ = w.Write(data)
			}
		}
		return
	}
	contentType := mime.TypeByExtension(kind)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	if r.Method != http.MethodHead {
		_, _ = w.Write(data)
	}
}

func fsName(name string) bool {
	return name != "" && name != "." && !strings.Contains(name, "\\") && !strings.ContainsRune(name, 0) && path.Clean(name) == name && !strings.HasPrefix(name, "/") && name != ".." && !strings.HasPrefix(name, "../")
}

func (v *preview) report(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if r.Header.Get("Origin") != v.origin || !sameToken(r.Header.Get("X-Artifact-Token"), v.token) {
		http.Error(w, "Origin or token refused", http.StatusForbidden)
		return
	}
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || contentType != "application/json" {
		w.WriteHeader(http.StatusUnsupportedMediaType)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxBytes)
	decoder := json.NewDecoder(r.Body)
	var item protocol.ArtifactError
	if decoder.Decode(&item) != nil || decoder.Decode(new(any)) != io.EOF || strings.TrimSpace(item.Message) == "" || len(item.Message) > 4096 || len(item.Source) > 2048 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	item.Time = time.Now().UTC().Format(time.RFC3339)
	for _, token := range []string{v.token, v.contentToken} {
		item.Message = strings.ReplaceAll(item.Message, v.origin+"/"+token+"/", "[preview]/")
		item.Source = strings.ReplaceAll(item.Source, v.origin+"/"+token+"/", "[preview]/")
	}
	v.mu.Lock()
	if len(v.errors) < 100 {
		v.errors = append(v.errors, item)
	}
	v.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (v *preview) wrapper(w http.ResponseWriter, r *http.Request) {
	base := v.origin + "/" + v.contentToken + "/"
	static, _ := json.Marshal(base + "static/" + url.PathEscape(v.entry))
	runnable, _ := json.Marshal(base + "run/" + url.PathEscape(v.entry))
	endpoint, _ := json.Marshal(v.origin + "/" + v.token + "/__errors")
	token, _ := json.Marshal(v.token)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'nonce-"+v.token+"'; style-src 'unsafe-inline'; frame-src "+base+"; connect-src "+v.origin+"/"+v.token+"/__errors; base-uri 'none'; form-action 'none'; frame-ancestors 'none'; object-src 'none'")
	if r.Method == http.MethodHead {
		return
	}
	// scripts stay off until requested, running them can still let the page navigate its own frame away
	_, _ = fmt.Fprintf(w, `<!doctype html><html><head><meta charset="utf-8"><title>Preview</title><style>html,body{height:100%%;margin:0;color-scheme:light dark;font:14px system-ui}body{display:flex;flex-direction:column}header{display:flex;align-items:center;gap:12px;padding:10px 16px;border-bottom:1px solid GrayText}button{font:inherit;padding:6px 12px;cursor:pointer}iframe{width:100%%;flex:1;border:0;background:Canvas}</style></head><body><header><span id="state">%s</span><button id="scripts" type="button">%s</button><span>%s</span></header><iframe id="preview" title="Preview page" sandbox="" referrerpolicy="no-referrer" src="%s"></iframe><script nonce="%s">(()=>{const frame=document.getElementById("preview"),button=document.getElementById("scripts"),state=document.getElementById("state");let running=false,count=0;button.addEventListener("click",()=>{running=!running;frame.setAttribute("sandbox",running?"allow-scripts":"");frame.src=running?%s:%s;button.textContent=running?%s:%s;state.textContent=running?%s:%s;count=0});addEventListener("message",event=>{if(!running||event.source!==frame.contentWindow||event.origin!=="null")return;const d=event.data;if(!d||typeof d!=="object"||Array.isArray(d)||Object.keys(d).length!==5||d.type!=="artifact-error"||typeof d.message!=="string"||!d.message.trim()||d.message.length>4096||typeof d.source!=="string"||d.source.length>2048||!Number.isInteger(d.line)||d.line<0||d.line>2147483647||!Number.isInteger(d.column)||d.column<0||d.column>2147483647||count++>=100)return;fetch(%s,{method:"POST",headers:{"Content-Type":"application/json","X-Artifact-Token":%s},body:JSON.stringify({message:d.message,source:d.source,line:d.line,column:d.column})}).catch(()=>{})})})();</script></body></html>`,
		html.EscapeString(i18n.T("Scripts are off")), html.EscapeString(i18n.T("Run scripts")), html.EscapeString(i18n.T("Only run scripts from pages you trust.")), html.EscapeString(base+"static/"+url.PathEscape(v.entry)), html.EscapeString(v.token), runnable, static,
		jsonString(i18n.T("Stop scripts")), jsonString(i18n.T("Run scripts")), jsonString(i18n.T("Scripts are running")), jsonString(i18n.T("Scripts are off")), endpoint, token)
}

func jsonString(text string) string {
	value, _ := json.Marshal(text)
	return string(value)
}

func (v *preview) reporter() string {
	origin, _ := json.Marshal(v.origin)
	return fmt.Sprintf(`<script>(()=>{let n=0;const send=(message,source,line,column)=>{if(n++>=100)return;parent.postMessage({type:"artifact-error",message:String(message).slice(0,4096),source:String(source||"").slice(0,2048),line:line||0,column:column||0},%s)};addEventListener("error",e=>send(e.message||"Resource could not load",e.filename||(e.target&&(e.target.src||e.target.href)),e.lineno,e.colno),true);addEventListener("unhandledrejection",e=>send(e.reason&&e.reason.stack||e.reason,"",0,0));})();</script>`, origin)
}

func injectReporter(page, reporter string) string {
	lower := strings.ToLower(page)
	for _, opening := range []string{"<head>", "<head ", "<html ", "<html>"} {
		if start := strings.Index(lower, opening); start >= 0 {
			if end := strings.IndexByte(page[start:], '>'); end >= 0 {
				at := start + end + 1
				return page[:at] + reporter + page[at:]
			}
		}
	}
	if start := strings.Index(lower, "<!doctype"); start >= 0 {
		if end := strings.IndexByte(page[start:], '>'); end >= 0 {
			at := start + end + 1
			return page[:at] + reporter + page[at:]
		}
	}
	return "<!doctype html>" + reporter + page
}

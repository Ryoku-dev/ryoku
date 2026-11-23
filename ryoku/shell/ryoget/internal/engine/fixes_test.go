package engine

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"ryoget/internal/media"
	"ryoget/internal/plan"
	"ryoget/internal/proto"
)

func sameBytes(t *testing.T, path string, want []byte) {
	t.Helper()
	if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, want) {
		t.Fatalf("%s holds %d bytes (%v), want %d", path, len(got), err, len(want))
	}
}

// strayParts lists any .part file or sidecar left in dirs.
func strayParts(dirs ...string) []string {
	var out []string
	for _, d := range dirs {
		entries, _ := os.ReadDir(d)
		for _, en := range entries {
			if strings.Contains(en.Name(), ".part") {
				out = append(out, filepath.Join(d, en.Name()))
			}
		}
	}
	return out
}

// Resolving can refile a sorted download into a type folder nothing made yet.
func TestResolvedKindRefilesIntoANewFolder(t *testing.T) {
	data := randBytes(64 << 10)
	srv := serveBytes(data, nil)
	defer srv.Close()
	f := newFixture(t, map[string]any{"sortByType": true}, nil)
	tk, err := f.e.Add(proto.AddParams{URL: srv.URL + "/pack.zip", Mode: "audio"})
	if err != nil {
		t.Fatal(err)
	}
	if music := filepath.Join(f.dir, typeFolder(typeMusic)); tk.Dir != music {
		t.Fatalf("an audio download was added to %s, want %s", tk.Dir, music)
	}
	done := waitTask(t, f.e, tk.ID, inState(proto.StateDone, proto.StateError))
	archives := filepath.Join(f.dir, typeFolder(typeArchives))
	if done.State != proto.StateDone || done.Dir != archives || filepath.Dir(done.Path) != archives {
		t.Fatalf("refiled download = %+v (error %+v)", done, done.Error)
	}
	sameBytes(t, done.Path, data)
}

// A refreshed download that already holds bytes stays in its folder with its partial.
func TestRefreshKeepsTheFolderOfATaskHoldingBytes(t *testing.T) {
	data := randBytes(128 << 10)
	srv := serveBytes(data, nil)
	defer srv.Close()
	base := filepath.Join(t.TempDir(), "sorted")
	archives := filepath.Join(base, typeFolder(typeArchives))
	os.MkdirAll(archives, 0o755)
	os.WriteFile(filepath.Join(archives, "a.zip.part"), data[:32<<10], 0o644)
	f := seedFixture(t, map[string]any{"sortByType": true}, []record{
		{Task: proto.Task{ID: "held", URL: srv.URL + "/a.zip?sig=old", Kind: "file", State: proto.StatePaused, Dir: archives, Name: "a.zip",
			Path: filepath.Join(archives, "a.zip"), Done: 32 << 10, Size: int64(len(data))}, Known: true, Sort: base},
	})
	if _, err := f.e.Refresh("held", srv.URL+"/get?id=1"); err != nil {
		t.Fatal(err)
	}
	done := waitTask(t, f.e, "held", inState(proto.StateDone, proto.StateError))
	if done.State != proto.StateDone || done.Dir != archives || done.Path != filepath.Join(archives, "a.zip") {
		t.Fatalf("refreshed download = %+v (error %+v)", done, done.Error)
	}
	sameBytes(t, done.Path, data)
	if stray := strayParts(archives, base); len(stray) > 0 {
		t.Fatalf("partial data left behind: %v", stray)
	}
}

func TestSetLimitRefusesTorrents(t *testing.T) {
	f := seedFixture(t, map[string]any{}, []record{
		{Task: proto.Task{ID: "tor", URL: "magnet:?xt=urn:btih:abc", Kind: "torrent", State: proto.StatePaused}, Known: true},
	})
	var pe *proto.Error
	if _, err := f.e.SetLimit("tor", 64); !errors.As(err, &pe) || pe.Code != proto.CodeInvalid {
		t.Fatalf("a torrent took its own limit: %v", err)
	}
	if got, _ := f.e.Task("tor"); got.Limit != 0 {
		t.Fatalf("refused limit was kept: %d", got.Limit)
	}
}

// Headers a download was added with reach only the host they were given for.
func TestRefreshToAnotherHostDropsHeaders(t *testing.T) {
	for _, c := range []struct {
		host, site string
		same       bool
	}{
		{"a.example", "a.example", true},
		{"cdn.A.example", "a.example", true},
		{"evila.example", "a.example", false},
		{"a.example", "cdn.a.example", false},
		{"a.example", "", false},
	} {
		if got := sameSite(c.host, c.site); got != c.same {
			t.Fatalf("sameSite(%q, %q) = %v", c.host, c.site, got)
		}
	}

	data := randBytes(32 << 10)
	var mu sync.Mutex
	cookies := map[string][]string{}
	handler := func(name string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			cookies[name] = append(cookies[name], r.URL.Path+" "+r.Header.Get("Cookie"))
			mu.Unlock()
			w.Header().Set("ETag", `"1"`)
			http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
		})
	}
	a := httptest.NewServer(handler("a"))
	defer a.Close()
	ln, err := net.Listen("tcp", "127.0.0.2:0")
	if err != nil {
		t.Skipf("no second loopback address: %v", err)
	}
	b := httptest.NewUnstartedServer(handler("b"))
	b.Listener.Close()
	b.Listener = ln
	b.Start()
	defer b.Close()

	secret := map[string]string{"Cookie": "session=s3cret"}
	f := seedFixture(t, map[string]any{}, []record{
		{Task: proto.Task{ID: "away", URL: a.URL + "/x.bin", Kind: "file", State: proto.StatePaused}, Known: true, Direct: true, Headers: secret},
		{Task: proto.Task{ID: "home", URL: a.URL + "/y.bin", Kind: "file", State: proto.StatePaused}, Known: true, Direct: true, Headers: secret},
	})
	if _, err := f.e.Refresh("away", b.URL+"/x.bin"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.e.Refresh("home", a.URL+"/z.bin"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"away", "home"} {
		if got := waitTask(t, f.e, id, inState(proto.StateDone, proto.StateError)); got.State != proto.StateDone {
			t.Fatalf("%s = %+v", id, got.Error)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(cookies["b"]) == 0 {
		t.Fatal("no request reached the new host")
	}
	for _, req := range cookies["b"] {
		if strings.Contains(req, "s3cret") {
			t.Fatalf("the cookie went to another host: %v", cookies["b"])
		}
	}
	sent := false
	for _, req := range cookies["a"] {
		if strings.HasPrefix(req, "/z.bin ") {
			if !strings.HasSuffix(req, "session=s3cret") {
				t.Fatalf("the same host lost the cookie: %v", cookies["a"])
			}
			sent = true
		}
	}
	if !sent {
		t.Fatalf("no request for the same host's new link: %v", cookies["a"])
	}
}

// holdHashes holds every checksum hash until release is called.
func holdHashes(t *testing.T) (entered <-chan struct{}, release func()) {
	in, gate := make(chan struct{}, 4), make(chan struct{})
	orig := hashFile
	hashFile = func(path, algo string) (string, error) {
		in <- struct{}{}
		<-gate
		return orig(path, algo)
	}
	t.Cleanup(func() { hashFile = orig })
	var once sync.Once
	return in, func() { once.Do(func() { close(gate) }) }
}

func waitEntered(t *testing.T, entered <-chan struct{}) {
	t.Helper()
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("no hash started")
	}
}

func TestPauseWhileHashingKeepsTheChecksumFailure(t *testing.T) {
	entered, release := holdHashes(t)
	defer release()
	data := randBytes(64 << 10)
	srv := serveBytes(data, nil)
	defer srv.Close()
	f := newFixture(t, nil, nil)
	wrong := md5.Sum([]byte("something else"))
	tk, err := f.e.Add(proto.AddParams{URL: srv.URL + "/bad.bin", Checksum: "md5:" + hex.EncodeToString(wrong[:])})
	if err != nil {
		t.Fatal(err)
	}
	waitEntered(t, entered)
	f.e.Pause(tk.ID, false)
	release()
	got := waitTask(t, f.e, tk.ID, inState(proto.StateError, proto.StatePaused, proto.StateDone))
	if got.State != proto.StateError || got.Error == nil || got.Error.Code != proto.CodeChecksum {
		t.Fatalf("a pause while hashing a mismatching file: %+v (error %+v)", got, got.Error)
	}
}

func TestCancelWhileHashingKeepsTheVerifiedFile(t *testing.T) {
	entered, release := holdHashes(t)
	defer release()
	data := randBytes(64 << 10)
	srv := serveBytes(data, nil)
	defer srv.Close()
	f := newFixture(t, nil, nil)
	sum := sha256.Sum256(data)
	tk, err := f.e.Add(proto.AddParams{URL: srv.URL + "/good.bin", Checksum: "sha256:" + hex.EncodeToString(sum[:])})
	if err != nil {
		t.Fatal(err)
	}
	waitEntered(t, entered)
	if err := f.e.Cancel(tk.ID); err != nil {
		t.Fatal(err)
	}
	release()
	got := waitTask(t, f.e, tk.ID, inState(proto.StateDone, proto.StateStopped, proto.StateError))
	if got.State != proto.StateDone || got.Verified != "sha256" {
		t.Fatalf("a cancel while hashing a matching file: %+v (error %+v)", got, got.Error)
	}
	sameBytes(t, got.Path, data)
}

func TestRetentionForgetsTorrentsAndKeepsTheirData(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "dl")
	album := filepath.Join(dir, "Album")
	os.MkdirAll(album, 0o755)
	track := filepath.Join(album, "01.flac")
	os.WriteFile(track, []byte("finished track"), 0o644)
	old := time.Now().Add(-40 * 24 * time.Hour).UTC().Format(time.RFC3339)
	f := seedFixture(t, map[string]any{"keepDays": 30}, []record{
		{Task: proto.Task{ID: "old-tor", URL: "magnet:?xt=urn:btih:abc", Kind: "torrent", State: proto.StateError, Dir: dir, Name: "Album",
			Size: 100, Done: 40, Finished: old, Error: &proto.Error{Code: proto.CodeNetwork}}, Known: true},
	})
	if tasks := f.e.Tasks(); len(tasks) != 0 {
		t.Fatalf("retention kept %+v", tasks)
	}
	if _, err := os.Stat(track); err != nil {
		t.Fatalf("retention deleted a torrent's data: %v", err)
	}
}

// servedResolver answers every link with one file served from another link.
type servedResolver struct{ to string }

func (r servedResolver) Resolve(context.Context, string) (*media.Media, error) {
	return &media.Media{Kind: plan.KindFile, URL: r.to, Name: "paper.pdf", Title: "Paper"}, nil
}

func (servedResolver) Plan(*media.Media, media.Options) (*plan.Plan, error) {
	return nil, errors.New("unused")
}

func TestSortByTypeUsesTheServedLink(t *testing.T) {
	data := randBytes(16 << 10)
	srv := serveBytes(data, nil)
	defer srv.Close()
	f := newFixture(t, map[string]any{"sortByType": true}, &Parts{Resolver: servedResolver{to: srv.URL + "/files/paper.pdf"}})
	tk, err := f.e.Add(proto.AddParams{URL: "https://page.example/download?id=7"})
	if err != nil {
		t.Fatal(err)
	}
	if tk.Dir != f.dir {
		t.Fatalf("a page link was added to %s", tk.Dir)
	}
	done := waitTask(t, f.e, tk.ID, inState(proto.StateDone, proto.StateError))
	docs := filepath.Join(f.dir, typeFolder(typeDocuments))
	if done.State != proto.StateDone || done.Dir != docs || filepath.Dir(done.Path) != docs {
		t.Fatalf("served pdf = %+v (error %+v)", done, done.Error)
	}
	sameBytes(t, done.Path, data)
}

// pageResolver serves a page listing files; each file resolves through the media resolver.
type pageResolver struct {
	page, base string
	env        *plan.Env
}

func (r pageResolver) Resolve(ctx context.Context, link string) (*media.Media, error) {
	if link != r.page {
		return media.Resolve(ctx, link, r.env, false)
	}
	return &media.Media{Kind: plan.KindMedia, URL: link, Title: "Mirror", Source: media.Source{ID: "page", Name: "Mirror"}, Items: []media.Item{
		{Title: "One", Kind: "file", URL: r.base + "/one.bin", Referer: r.page},
		{Title: "Two", Kind: "file", URL: r.base + "/two.bin", Referer: r.page},
	}}, nil
}

func (pageResolver) Plan(*media.Media, media.Options) (*plan.Plan, error) {
	return nil, fmt.Errorf("%w: a page of files", media.ErrUnsupported)
}

func TestPageFilesKeepTheirReferer(t *testing.T) {
	files := map[string][]byte{"/one.bin": randBytes(16 << 10), "/two.bin": randBytes(16 << 10)}
	var page string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if r.Referer() != page {
			http.Error(w, "hotlinking", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("ETag", `"`+r.URL.Path+`"`)
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
	}))
	defer srv.Close()
	page = srv.URL + "/page"
	f := newFixture(t, nil, &Parts{Resolver: pageResolver{page: page, base: srv.URL, env: &plan.Env{HTTP: srv.Client()}}})
	if _, err := f.e.Add(proto.AddParams{URL: page}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool {
		tasks := f.e.Tasks()
		return len(tasks) == 2 && !slices.ContainsFunc(tasks, func(tk proto.Task) bool { return !settled(tk.State) })
	})
	for _, tk := range f.e.Tasks() {
		if tk.State != proto.StateDone {
			t.Fatalf("%s = %s (error %+v)", tk.URL, tk.State, tk.Error)
		}
		sameBytes(t, tk.Path, files[strings.TrimPrefix(tk.URL, srv.URL)])
	}
}

func TestPlanSigIgnoresTheCover(t *testing.T) {
	audio := plan.Part{Kind: plan.PartHLS, Ext: "m4a", Role: "audio"}
	bare := &plan.Plan{Name: "Song", Container: "m4a", Post: plan.PostAudio, Parts: []plan.Part{audio}}
	covered := &plan.Plan{Name: "Song", Container: "m4a", Post: plan.PostAudio, Parts: []plan.Part{audio, {Kind: plan.PartHTTP, Ext: "jpg", Role: "cover"}}}
	if planSig(bare) != planSig(covered) {
		t.Fatal("a cover changed the plan signature")
	}
	other := &plan.Plan{Name: "Song", Container: "m4a", Post: plan.PostAudio, Parts: []plan.Part{{Kind: plan.PartHLS, Ext: "opus", Role: "audio"}}}
	if planSig(bare) == planSig(other) {
		t.Fatal("a different audio part kept the plan signature")
	}
}

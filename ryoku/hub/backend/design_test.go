package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func designTestChunk(kind string, b []byte) []byte {
	out := append([]byte(kind), make([]byte, 4)...)
	binary.LittleEndian.PutUint32(out[4:], uint32(len(b)))
	out = append(out, b...)
	if len(b)%2 != 0 {
		out = append(out, 0)
	}
	return out
}
func designTestRIFF(chunks ...[]byte) []byte {
	b := []byte("WEBP")
	for _, c := range chunks {
		b = append(b, c...)
	}
	out := append([]byte("RIFF"), make([]byte, 4)...)
	binary.LittleEndian.PutUint32(out[4:], uint32(len(b)))
	return append(out, b...)
}
func designTestVP8(w, h int) []byte {
	b := []byte{0, 0, 0, 0x9d, 1, 0x2a, 0, 0, 0, 0}
	binary.LittleEndian.PutUint16(b[6:], uint16(w))
	binary.LittleEndian.PutUint16(b[8:], uint16(h))
	return designTestChunk("VP8 ", b)
}
func designTest24(b []byte, n int) { b[0] = byte(n); b[1] = byte(n >> 8); b[2] = byte(n >> 16) }
func designTestAnimated(w, h, count, x int) []byte {
	header := make([]byte, 10)
	header[0] = 2
	designTest24(header[4:], w-1)
	designTest24(header[7:], h-1)
	chunks := [][]byte{designTestChunk("VP8X", header), designTestChunk("ANIM", make([]byte, 6))}
	for range count {
		b := make([]byte, 16)
		designTest24(b, x/2)
		designTest24(b[6:], w-1)
		designTest24(b[9:], h-1)
		b = append(b, designTestVP8(w, h)...)
		chunks = append(chunks, designTestChunk("ANMF", b))
	}
	return designTestRIFF(chunks...)
}
func designTestSpec() designSpec {
	return designSpec{Version: 1, Name: "Sample", Badge: designBadge{Emblem: "emblem.webp", Title: "CORE", Accent: "#FF8800"}, Card: designCard{Palette: designPalette{Ink: "#FFFFFF", Accent: "#FF8800", Plate: "#000000", Rim: "#DD6600"}, Art: []designArt{{Image: "art.webp", Depth: 2, Blend: "normal"}}, Effects: []designEffect{{ID: "foil", Strength: 2, Speed: -1, Color: "#FF8800"}}, Words: designWords{Tagline: "Hello\u202eworld", Epithet: "Team"}}}
}
func designTestPack(t *testing.T) designPack {
	t.Helper()
	raw, err := json.Marshal(designTestSpec())
	if err != nil {
		t.Fatal(err)
	}
	return designPack{files: map[string][]byte{"design.json": raw, "emblem.webp": designTestRIFF(designTestVP8(256, 256)), "art.webp": designTestAnimated(560, 390, 2, 0)}, names: []string{"art.webp", "design.json", "emblem.webp"}, root: "/sample"}
}

func TestDesignWebP(t *testing.T) {
	still := designTestRIFF(designTestVP8(256, 256))
	anim := designTestAnimated(560, 390, 2, 0)
	for name, raw := range map[string][]byte{"still": still, "animated": anim, "lossless": designTestRIFF(designTestChunk("VP8L", []byte{0x2f, 0xff, 0xc0, 0x3f, 0}))} {
		t.Run(name, func(t *testing.T) {
			info, err := parseWebP(raw)
			if err != nil {
				t.Fatal(err)
			}
			if info.Width < 1 || len(info.Frames) < 1 {
				t.Fatal(info)
			}
		})
	}
	info, err := parseWebP(anim)
	if err != nil || info.Width != 560 || info.Height != 390 || len(info.Frames) != 2 || info.Frames[1] != image.Rect(0, 0, 560, 390) {
		t.Fatalf("%+v %v", info, err)
	}
	bad := map[string][]byte{"truncated": still[:len(still)-1], "outside": designTestAnimated(256, 256, 1, 2), "canvas": designTestRIFF(designTestVP8(2049, 256)), "frames": designTestAnimated(1, 1, 97, 0), "pixels": designTestAnimated(2048, 2048, 23, 0), "header": designTestRIFF(designTestChunk("EXIF", []byte{0})), "no-image": designTestRIFF(), "chunk-size": designTestRIFF([]byte{'V', 'P', '8', ' ', 255, 255, 255, 255})}
	for name, raw := range bad {
		t.Run(name, func(t *testing.T) {
			if _, err := parseWebP(raw); err == nil {
				t.Fatal("accepted hostile WebP")
			}
		})
	}
}

func TestDesignValidate(t *testing.T) {
	p := designTestPack(t)
	s, err := designValidate(p)
	if err != nil {
		t.Fatal(err)
	}
	if s.Card.Art[0].Depth != 1 || s.Card.Effects[0].Speed != 0 || s.Badge.Accent != "#ff8800" || s.Badge.Emblem != "/sample/emblem.webp" || s.Card.Words.Tagline != "Helloworld" {
		t.Fatalf("normalization: %+v", s)
	}
	changes := map[string]func(*designSpec){"color": func(s *designSpec) { s.Badge.Accent = "orange" }, "contrast": func(s *designSpec) { s.Card.Palette.Ink = "#111111" }, "missing": func(s *designSpec) { s.Badge.Emblem = "missing.webp" }, "aspect": func(s *designSpec) { s.Card.Frame = "art.webp" }, "emblem-aspect": func(s *designSpec) { s.Badge.Emblem = "art.webp" }, "empty-art": func(s *designSpec) { s.Card.Art = nil }, "too-many-art": func(s *designSpec) { s.Card.Art = make([]designArt, 5) }, "effect": func(s *designSpec) { s.Card.Effects[0].ID = "code" }, "duplicate-effect": func(s *designSpec) { s.Card.Effects = append(s.Card.Effects, s.Card.Effects[0]) }, "title": func(s *designSpec) { s.Badge.Title = strings.Repeat("x", 13) }, "blend": func(s *designSpec) { s.Card.Art[0].Blend = "multiply" }, "path": func(s *designSpec) { s.Card.Art[0].Image = "../art.webp" }}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			p := designTestPack(t)
			s := designTestSpec()
			change(&s)
			p.files["design.json"], _ = json.Marshal(s)
			if _, err := designValidate(p); err == nil {
				t.Fatal("accepted invalid spec")
			}
		})
	}
	t.Run("unknown-key", func(t *testing.T) {
		p := designTestPack(t)
		p.files["design.json"] = bytes.Replace(p.files["design.json"], []byte(`"version":1`), []byte(`"version":1,"script":"evil"`), 1)
		if _, err := designValidate(p); err == nil {
			t.Fatal("accepted unknown key")
		}
	})
	t.Run("wrong-kind", func(t *testing.T) {
		p := designTestPack(t)
		p.files["wrong.png"] = p.files["art.webp"]
		p.names = append(p.names, "wrong.png")
		if _, err := designValidate(p); err == nil {
			t.Fatal("accepted disguised WebP")
		}
	})
}

func TestDesignPackPixelBudget(t *testing.T) {
	for _, format := range []string{"webp", "png", "jpg", "jpeg"} {
		for _, extra := range []int{0, 1, 2} {
			t.Run(fmt.Sprintf("%s/extra-%d", format, extra), func(t *testing.T) {
				p := designTestPack(t)
				p.files["emblem.webp"] = designTestRIFF(designTestVP8(999, 1001))
				p.files["art.webp"] = designTestAnimated(1000, 1000, 30, 0)
				p.files["unused.webp"] = designTestAnimated(1000, 1000, 33, 0)
				p.names = append(p.names, "unused.webp")
				if extra > 0 {
					var raw bytes.Buffer
					img := image.NewRGBA(image.Rect(0, 0, extra, 1))
					var err error
					switch format {
					case "webp":
						raw.Write(designTestRIFF(designTestVP8(extra, 1)))
					case "png":
						err = png.Encode(&raw, img)
					default:
						err = jpeg.Encode(&raw, img, nil)
					}
					if err != nil {
						t.Fatal(err)
					}
					name := "extra." + format
					p.files[name] = raw.Bytes()
					p.names = append(p.names, name)
				}
				_, err := designValidate(p)
				if extra <= 1 {
					if err != nil {
						t.Fatalf("rejected %d decoded pixels: %v", 63_999_999+extra, err)
					}
				} else if err == nil || err.Error() != "pack decoded pixels total 64000001 exceeds 64000000" {
					t.Fatalf("over-budget pack error: %v", err)
				}
			})
		}
	}
	t.Run("partial-frames-use-full-canvas", func(t *testing.T) {
		p := designTestPack(t)
		anim := designTestAnimated(1, 1, 8, 0)
		chunks, err := webpChunks(anim[12:])
		if err != nil {
			t.Fatal(err)
		}
		designTest24(chunks[0].body[4:], 2047)
		designTest24(chunks[0].body[7:], 2047)
		p.files["art.webp"] = anim
		p.files["unused.webp"] = anim
		p.names = append(p.names, "unused.webp")
		if _, err := designValidate(p); err == nil || err.Error() != "pack decoded pixels total 67174400 exceeds 64000000" {
			t.Fatalf("partial-frame pack budget error: %v", err)
		}
	})
}

func TestDesignApproveCurrentTime(t *testing.T) {
	owner, _, _ := badgeTestKeys(t)
	key := filepath.Join(t.TempDir(), "owner.key")
	if err := os.WriteFile(key, []byte(base64.StdEncoding.EncodeToString(owner)), 0600); err != nil {
		t.Fatal(err)
	}
	posted := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/designs" {
			t.Errorf("unexpected approval request: %s %s", r.Method, r.URL.Path)
		}
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		posted <- string(raw)
		w.Write([]byte("{}"))
	}))
	defer server.Close()
	t.Setenv("RYOKU_TEAM_URL", server.URL)
	sha := sha256.Sum256([]byte("approved pack"))
	before := time.Now().Unix()
	if err := designCommand([]string{"approve", "--issuer", "owner", "--key-file", key, "--login", "neur0map", "--sha", hex.EncodeToString(sha[:])}); err != nil {
		t.Fatal(err)
	}
	after := time.Now().Unix()
	approval, err := rydParse(<-posted)
	if err != nil {
		t.Fatal(err)
	}
	approvedAt, err := time.Parse(time.RFC3339, approval.ApprovedAt)
	if err != nil {
		t.Fatal(err)
	}
	if approvedAt.Unix() < before || approvedAt.Unix() > after {
		t.Fatalf("approval time %s is outside command run [%d, %d]", approval.ApprovedAt, before, after)
	}
	if approval.SHA != hex.EncodeToString(sha[:]) || approval.Login != "neur0map" || approval.Issuer != "owner" {
		t.Fatalf("wrong signed approval: %+v", approval)
	}
}

func designTestZip(t *testing.T, names []string, method uint16, payload []byte) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for _, name := range names {
		h := &zip.FileHeader{Name: name, Method: method}
		h.SetMode(0600)
		f, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.Write(payload); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func TestDesignHostilePacks(t *testing.T) {
	tests := map[string][]byte{"nested": designTestZip(t, []string{"dir/art.webp"}, zip.Store, nil), "parent": designTestZip(t, []string{"../art.webp"}, zip.Store, nil), "absolute": designTestZip(t, []string{"/art.webp"}, zip.Store, nil), "duplicate-case": designTestZip(t, []string{"art.webp", "ART.webp"}, zip.Store, nil), "bomb": designTestZip(t, []string{"art.webp"}, zip.Deflate, make([]byte, 1<<20)), "oversize": make([]byte, designZipLimit+1), "json-size": designTestZip(t, []string{"design.json"}, zip.Store, make([]byte, designJSONLimit+1))}
	var many []string
	for i := range 25 {
		many = append(many, fmt.Sprintf("art%d.webp", i))
	}
	tests["too-many"] = designTestZip(t, many, zip.Store, nil)
	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := designReadZip(raw, ""); err == nil {
				t.Fatal("accepted hostile zip")
			}
		})
	}
	dir := t.TempDir()
	if err := os.Symlink("/etc/passwd", filepath.Join(dir, "art.webp")); err != nil {
		t.Fatal(err)
	}
	if _, err := designReadPack(dir); err == nil {
		t.Fatal("accepted symlink")
	}
	if err := os.Remove(filepath.Join(dir, "art.webp")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "art.webp"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := designReadPack(dir); err == nil {
		t.Fatal("accepted directory entry")
	}
}

func TestDesignDeterministicPack(t *testing.T) {
	p := designTestPack(t)
	a, err := designZip(p)
	if err != nil {
		t.Fatal(err)
	}
	b, err := designZip(p)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("nondeterministic pack")
	}
	got, err := designReadZip(a, "")
	if err != nil {
		t.Fatal(err)
	}
	s, err := designValidate(got)
	if err != nil {
		t.Fatal(err)
	}
	if s.Badge.Emblem != "emblem.webp" || !reflect.DeepEqual(got.names, p.names) {
		t.Fatal("zip paths/names changed")
	}
}

func TestDesignFetch(t *testing.T) {
	p := designTestPack(t)
	raw, err := designZip(p)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	sha := hex.EncodeToString(digest[:])
	body := raw
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/packs/"+sha+".zip" && r.URL.Path != "/v1/pending/"+sha+".zip" {
			t.Errorf("wrong path %s", r.URL.Path)
		}
		w.Write(body)
	}))
	defer server.Close()
	t.Setenv("RYOKU_TEAM_URL", server.URL)
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	result, err := designFetch(sha, false)
	if err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(cache, "ryoku", "team", "designs", sha)
	if result.Dir != expected || result.Design.Badge.Emblem != filepath.Join(expected, "emblem.webp") {
		t.Fatal(result)
	}
	normalized, err := os.ReadFile(filepath.Join(expected, "normalized.json"))
	if err != nil {
		t.Fatal(err)
	}
	var spec designSpec
	if err = json.Unmarshal(normalized, &spec); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(spec, result.Design) {
		t.Fatal("normalized spec differs")
	}
	if _, err = designFetch(sha, true); err != nil {
		t.Fatal(err)
	}
	body = []byte("wrong sha")
	if _, err = designFetch(sha, false); err == nil {
		t.Fatal("accepted sha mismatch")
	}
	if _, err = designFetch("../outside", false); err == nil {
		t.Fatal("accepted sha path")
	}
	hostile := designTestZip(t, []string{"../outside"}, zip.Store, []byte("escape"))
	sum := sha256.Sum256(hostile)
	hostileSHA := hex.EncodeToString(sum[:])
	body = hostile
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(body) })
	if _, err = designFetch(hostileSHA, false); err == nil {
		t.Fatal("accepted traversal")
	}
	if _, err = os.Stat(filepath.Join(cache, "ryoku", "team", "outside")); !os.IsNotExist(err) {
		t.Fatal("wrote outside cache")
	}
	body = raw
	outside := t.TempDir()
	if err = os.RemoveAll(expected); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(outside, expected); err != nil {
		t.Fatal(err)
	}
	if _, err = designFetch(sha, false); err == nil {
		t.Fatal("accepted symlink cache")
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatal("wrote through symlink")
	}
}

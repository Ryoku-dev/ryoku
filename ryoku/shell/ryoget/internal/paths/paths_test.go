package paths

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestXDGLayout(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(root, "run"))
	t.Setenv("XDG_DOWNLOAD_DIR", filepath.Join(root, "Téléchargements"))
	for got, want := range map[string]string{
		StateDir():       filepath.Join(root, "state", "ryoku", "ryoget"),
		CacheDir():       filepath.Join(root, "cache", "ryoku", "ryoget"),
		ConfigFile():     filepath.Join(root, "config", "ryoku", "downloads.json"),
		TasksFile():      filepath.Join(root, "state", "ryoku", "ryoget", "tasks.json"),
		Socket():         filepath.Join(root, "run", "ryoget.sock"),
		Lock():           filepath.Join(root, "run", "ryoget.lock"),
		SeekStateDir():   filepath.Join(root, "state", "ryoku", "ryoseek"),
		SeekHealthFile(): filepath.Join(root, "state", "ryoku", "ryoseek", "health.json"),
		SeekLog():        filepath.Join(root, "state", "ryoku", "ryoseek", "ryoseek.log"),
		SeekCacheDir():   filepath.Join(root, "cache", "ryoku", "ryoseek"),
		SeekSocket():     filepath.Join(root, "run", "ryoseek.sock"),
		SeekLock():       filepath.Join(root, "run", "ryoseek.lock"),
		DownloadDir():    filepath.Join(root, "Téléchargements"),
		Expand("~/x"):    filepath.Join(root, "x"),
	} {
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}

func TestFallbacks(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_RUNTIME_DIR", "")
	t.Setenv("XDG_DOWNLOAD_DIR", "relative/ignored")
	t.Setenv("XDG_STATE_HOME", "relative")
	uid := strconv.Itoa(os.Getuid())
	if got, want := Socket(), filepath.Join(os.TempDir(), "ryoget-"+uid+".sock"); got != want {
		t.Errorf("socket %q, want %q", got, want)
	}
	if got, want := Lock(), filepath.Join(os.TempDir(), "ryoget-"+uid+".lock"); got != want {
		t.Errorf("lock %q, want %q", got, want)
	}
	if got, want := SeekSocket(), filepath.Join(os.TempDir(), "ryoseek-"+uid+".sock"); got != want {
		t.Errorf("ryoseek socket %q, want %q", got, want)
	}
	if got, want := SeekLock(), filepath.Join(os.TempDir(), "ryoseek-"+uid+".lock"); got != want {
		t.Errorf("ryoseek lock %q, want %q", got, want)
	}
	if got, want := SeekStateDir(), filepath.Join(root, ".local", "state", "ryoku", "ryoseek"); got != want {
		t.Errorf("ryoseek state %q, want %q", got, want)
	}
	if got, want := DownloadDir(), filepath.Join(root, "Downloads"); got != want {
		t.Errorf("download dir %q, want %q", got, want)
	}
	if got, want := StateDir(), filepath.Join(root, ".local", "state", "ryoku", "ryoget"); got != want {
		t.Errorf("state %q, want %q", got, want)
	}
}

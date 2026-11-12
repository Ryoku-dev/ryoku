package memory_test

import (
	"path/filepath"
	"testing"

	"ryoku-rashin/internal/memory"
	"ryoku-rashin/internal/memory/backend"
	"ryoku-rashin/internal/memory/backendtest"
)

func TestBuiltinContract(t *testing.T) {
	backendtest.Run(t, func(t *testing.T) backend.Backend {
		dir := t.TempDir()
		st, err := memory.OpenStore(memory.StoreConfig{Dir: filepath.Join(dir, "vault"), TeamDir: filepath.Join(dir, "teams"), DB: filepath.Join(dir, "memory.db")})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = st.Close() })
		return memory.Builtin(st)
	})
}

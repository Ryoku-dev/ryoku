package embed

import (
	_ "embed"
	"sync"
)

//go:embed model/model.safetensors
var bundledModel []byte

//go:embed model/tokenizer.json
var bundledTokenizer []byte

var global struct {
	sync.Mutex
	model     *Model
	err       error
	attempted bool
}

func Load() (*Model, error) {
	global.Lock()
	defer global.Unlock()
	if !global.attempted {
		global.attempted = true
		global.model, global.err = loadModel(bundledModel, bundledTokenizer)
	}
	return global.model, global.err
}

// Release drops decoded model state. Embedded bytes remain mapped by the binary.
func Release() {
	global.Lock()
	global.model = nil
	global.err = nil
	global.attempted = false
	global.Unlock()
}

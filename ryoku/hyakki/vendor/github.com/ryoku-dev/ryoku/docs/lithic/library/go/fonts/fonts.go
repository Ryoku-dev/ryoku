package fonts

import (
	"embed"
	"sync"

	"gioui.org/font"
	"gioui.org/font/opentype"
)

//go:embed *.ttf
var files embed.FS

var (
	once  sync.Once
	faces []font.FontFace
)

// Collection returns the embedded faces. It parses them once; the result is shared, so append
// to it only through a copy.
func Collection() []font.FontFace {
	once.Do(func() {
		entries, err := files.ReadDir(".")
		if err != nil {
			panic(err)
		}
		for _, entry := range entries {
			data, err := files.ReadFile(entry.Name())
			if err != nil {
				panic(err)
			}
			parsed, err := opentype.ParseCollection(data)
			if err != nil {
				panic(err)
			}
			faces = append(faces, parsed...)
		}
	})
	return faces[:len(faces):len(faces)]
}

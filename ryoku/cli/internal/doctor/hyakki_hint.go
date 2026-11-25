package doctor

import "ryoku-cli/internal/sys"

func hyakkiOn() bool {
	return sys.Has("hyakki") && readHyakkiEnabled()
}

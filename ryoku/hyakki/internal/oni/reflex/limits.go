package reflex

import (
	"sync"

	"ryoku-rashin/internal/oni/spec"
)

// limits is the spec's reflex section, read once.
var limits = sync.OnceValue(func() spec.Reflex {
	s, err := spec.Load()
	if err != nil {
		panic(err)
	}
	return s.Reflex
})

package generics

import (
	"golang.org/x/exp/constraints"
)

// Deprecated: Use MakeMapIfNil and MapInsert separately.
func MakeMapIfNilAndSet[K comparable, V any](pm *map[K]V, k K, v V) (added bool) {
	MakeMapIfNil(pm)
	m := *pm
	_, exists := m[k]
	added = !exists
	m[k] = v
	return
}

// Does this exist in the maps package?
func MakeMap[K comparable, V any, M ~map[K]V](pm *M) {
	*pm = make(M)
}

func MakeMapWithCap[K comparable, V any, M ~map[K]V, C constraints.Integer](pm *M, cap C) {
	*pm = make(M, cap)
}

func MakeMapIfNil[K comparable, V any, M ~map[K]V](pm *M) {
	if *pm == nil {
		MakeMap(pm)
	}
}

func MakeMapIfNilWithCap[K comparable, V any, M ~map[K]V, C constraints.Integer](pm *M, cap C) {
	if *pm == nil {
		MakeMapWithCap(pm, cap)
	}
}

func MapContains[K comparable, V any, M ~map[K]V](m M, k K) bool {
	_, ok := m[k]
	return ok
}

func MapMustGet[K comparable, V any, M ~map[K]V](m M, k K) V {
	v, ok := m[k]
	if !ok {
		panic(k)
	}
	return v
}

// Returns Some of the previous value if there was one.
func MapInsert[K comparable, V any, M ~map[K]V](m M, k K, v V) Option[V] {
	old, ok := m[k]
	m[k] = v
	return Option[V]{
		Value: old,
		Ok:    ok,
	}
}

// Deletes element with the key k. If there is no element with the specified key, panics. delete
// only applies to maps, so for now Map is not mentioned in the function name.
func MustDelete[K comparable, V any, M ~map[K]V](m M, k K) {
	_, ok := m[k]
	if !ok {
		panic(k)
	}
	delete(m, k)
}

// Panics if the key is already assigned in the map.
func MapMustAssignNew[K comparable, V any, M ~map[K]V](m M, k K, v V) {
	if MapInsert(m, k, v).Ok {
		panic(k)
	}
}

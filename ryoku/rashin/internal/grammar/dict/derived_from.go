package dict

import (
	"encoding/json"
	"fmt"
	"sort"
)

// DerivedFrom stores the sorted, unique canonical IDs a word is derived from.
type DerivedFrom []CanonicalWordId

func (d DerivedFrom) MarshalJSON() ([]byte, error) {
	ids := []CanonicalWordId(d)
	if ids == nil {
		ids = []CanonicalWordId{}
	}
	return json.Marshal(struct {
		Inner []CanonicalWordId `json:"inner"`
	}{ids})
}
func (d *DerivedFrom) UnmarshalJSON(data []byte) error {
	var value struct {
		Inner *[]CanonicalWordId `json:"inner"`
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if value.Inner == nil {
		return fmt.Errorf("missing derivation inner set")
	}
	var result DerivedFrom
	result.Extend(*value.Inner)
	*d = result
	return nil
}

func NewDerivedFromFromCanonicalWordId(id CanonicalWordId) DerivedFrom { return DerivedFrom{id} }

// Insert adds an ID unless it is already present.
func (d *DerivedFrom) Insert(id CanonicalWordId) {
	index := sort.Search(len(*d), func(i int) bool { return (*d)[i] >= id })
	if index < len(*d) && (*d)[index] == id {
		return
	}
	*d = append(*d, 0)
	copy((*d)[index+1:], (*d)[index:])
	(*d)[index] = id
}
func (d DerivedFrom) Contains(id CanonicalWordId) bool {
	i := sort.Search(len(d), func(i int) bool { return d[i] >= id })
	return i < len(d) && d[i] == id
}
func (d DerivedFrom) IsEmpty() bool           { return len(d) == 0 }
func (d DerivedFrom) IsNone() bool            { return d.IsEmpty() }
func (d DerivedFrom) Iter() []CanonicalWordId { return d }
func (d *DerivedFrom) Extend(ids []CanonicalWordId) {
	for _, id := range ids {
		d.Insert(id)
	}
}

// union preserves shared input storage when no IDs are added, and otherwise
// allocates the merged set once without modifying either input's backing array.
func (d DerivedFrom) union(other DerivedFrom) DerivedFrom {
	added := 0
	i := 0
	for _, id := range other {
		for i < len(d) && d[i] < id {
			i++
		}
		if i == len(d) || d[i] != id {
			added++
		}
	}
	if added == 0 {
		return d
	}
	out := make(DerivedFrom, 0, len(d)+added)
	i = 0
	for _, id := range other {
		for i < len(d) && d[i] < id {
			out = append(out, d[i])
			i++
		}
		if i == len(d) || d[i] != id {
			out = append(out, id)
		}
	}
	return append(out, d[i:]...)
}

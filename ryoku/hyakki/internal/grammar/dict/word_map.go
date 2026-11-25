package dict

// WordMapEntry stores one canonical spelling, its metadata and both IDs.
type WordMapEntry struct {
	CanonicalSpelling []rune
	Metadata          DictWordMetadata
	WordIds           WordIdPair
}

func NewWordMapEntry(spelling []rune) WordMapEntry {
	return WordMapEntry{CanonicalSpelling: append([]rune(nil), spelling...), WordIds: NewWordIdPairFromWordChars(spelling)}
}
func NewWordMapEntryStr(spelling string) WordMapEntry {
	chars := []rune(spelling)
	return WordMapEntry{CanonicalSpelling: chars, WordIds: NewWordIdPairFromWordChars(chars)}
}
func (entry WordMapEntry) WithMD(metadata DictWordMetadata) WordMapEntry {
	entry.Metadata = metadata
	return entry
}

// WordMap stores canonical entries in insertion order, indexed by both IDs.
type WordMap struct {
	entries    []WordMapEntry
	canonical  wordMapIndex
	caseFolded wordMapIndex
	alternates map[CaseFoldedWordId][]int
}

// wordMapIndex stores entry positions plus one; zero marks an empty bucket.
// IDs stay in entries rather than being duplicated in two hash maps.
type wordMapIndex struct {
	slots []uint32
	count int
}

func newWordMapIndex(capacity int) wordMapIndex {
	if capacity == 0 {
		return wordMapIndex{}
	}
	size := 8
	for capacity > size-size/4 {
		size *= 2
	}
	return wordMapIndex{slots: make([]uint32, size)}
}

func (h *wordMapIndex) lookup(entries []WordMapEntry, id uint64, folded bool) (int, bool) {
	if len(h.slots) == 0 {
		return 0, false
	}
	mask := len(h.slots) - 1
	for slot := int(id & uint64(mask)); ; slot = (slot + 1) & mask {
		value := h.slots[slot]
		if value == 0 {
			return 0, false
		}
		index := int(value - 1)
		key := uint64(entries[index].WordIds.Canonical)
		if folded {
			key = uint64(entries[index].WordIds.CaseFolded)
		}
		if key == id {
			return index, true
		}
	}
}

func (h *wordMapIndex) place(entries []WordMapEntry, index int, folded bool) {
	id := uint64(entries[index].WordIds.Canonical)
	if folded {
		id = uint64(entries[index].WordIds.CaseFolded)
	}
	mask := len(h.slots) - 1
	slot := int(id & uint64(mask))
	for h.slots[slot] != 0 {
		slot = (slot + 1) & mask
	}
	h.slots[slot] = uint32(index) + 1
}

func (h *wordMapIndex) insert(entries []WordMapEntry, index int, folded bool) {
	if h.count+1 > len(h.slots)-len(h.slots)/4 {
		old := h.slots
		h.slots = make([]uint32, max(8, len(old)*2))
		for _, value := range old {
			if value != 0 {
				h.place(entries, int(value-1), folded)
			}
		}
	}
	h.place(entries, index, folded)
	h.count++
}

func (m *WordMap) caseFoldedIndex(id CaseFoldedWordId) (int, bool) {
	return m.caseFolded.lookup(m.entries, uint64(id), true)
}

func NewWordMap() *WordMap { return NewWordMapWithCapacity(0) }
func NewWordMapWithCapacity(capacity int) *WordMap {
	return &WordMap{entries: make([]WordMapEntry, 0, capacity), canonical: newWordMapIndex(capacity), caseFolded: newWordMapIndex(capacity)}
}

// CuratedWordMap shares the curated dictionary's underlying canonical storage.
func CuratedWordMap() *WordMap { return Curated().wordMap }
func NewWordMapFromRuneFiles(wordList, attrs string) (*WordMap, error) {
	body, count, err := wordListBody(wordList)
	if err != nil {
		return nil, err
	}
	attributes, err := ParseAttributeList(attrs)
	if err != nil {
		return nil, err
	}
	result := NewWordMapWithCapacity(count * 3)
	var annotations []rune
	walkWordList(body, func(word, flags string) {
		annotations = annotations[:0]
		for _, flag := range flags {
			annotations = append(annotations, flag)
		}
		attributes.ExpandAnnotatedWord(AnnotatedWord{Letters: []rune(word), Annotations: annotations}, result)
	})
	return result, nil
}
func (m *WordMap) ContainsCanonical(id CanonicalWordId) bool { return m.GetCanonical(id) != nil }
func (m *WordMap) ContainsCaseFolded(id CaseFoldedWordId) bool {
	_, ok := m.caseFoldedIndex(id)
	return ok
}
func (m *WordMap) GetCanonical(id CanonicalWordId) *WordMapEntry {
	index, ok := m.canonical.lookup(m.entries, uint64(id), false)
	if !ok {
		return nil
	}
	return &m.entries[index]
}
func (m *WordMap) GetCaseFolded(id CaseFoldedWordId) []*WordMapEntry {
	index, ok := m.caseFoldedIndex(id)
	if !ok {
		return nil
	}
	out := make([]*WordMapEntry, 1, 1+len(m.alternates[id]))
	out[0] = &m.entries[index]
	for _, index := range m.alternates[id] {
		out = append(out, &m.entries[index])
	}
	return out
}
func (m *WordMap) GetCaseFoldedChars(word []rune) []*WordMapEntry {
	id, _, _ := caseFoldedWordID(word)
	return m.GetCaseFolded(id)
}
func (m *WordMap) GetMetadataMutCanonical(id CanonicalWordId) *DictWordMetadata {
	entry := m.GetCanonical(id)
	if entry == nil {
		return nil
	}
	return &entry.Metadata
}

// Insert merges metadata when a canonical identifier already exists.
func (m *WordMap) Insert(entry WordMapEntry) {
	if existing := m.GetCanonical(entry.WordIds.Canonical); existing != nil {
		existing.Metadata.Merge(entry.Metadata)
		return
	}
	index := len(m.entries)
	m.entries = append(m.entries, entry)
	m.canonical.insert(m.entries, index, false)
	if _, exists := m.caseFoldedIndex(entry.WordIds.CaseFolded); exists {
		if m.alternates == nil {
			m.alternates = make(map[CaseFoldedWordId][]int)
		}
		m.alternates[entry.WordIds.CaseFolded] = append(m.alternates[entry.WordIds.CaseFolded], index)
	} else {
		m.caseFolded.insert(m.entries, index, true)
	}
}

// Reserve ensures capacity for at least additional new canonical entries.
func (m *WordMap) Reserve(additional int) {
	if additional <= cap(m.entries)-len(m.entries) {
		return
	}
	capacity := len(m.entries) + additional
	if capacity < 2*cap(m.entries) {
		capacity = 2 * cap(m.entries)
	}
	entries := make([]WordMapEntry, len(m.entries), capacity)
	copy(entries, m.entries)
	m.entries = entries
}
func (m *WordMap) Iter() []WordMapEntry       { return m.entries }
func (m *WordMap) At(index int) *WordMapEntry { return &m.entries[index] }
func (m *WordMap) Len() int                   { return len(m.entries) }
func (m *WordMap) IsEmpty() bool              { return m.Len() == 0 }
func (m *WordMap) AppendWord(word []rune, metadata DictWordMetadata) {
	m.Insert(NewWordMapEntry(word).WithMD(metadata))
}
func (m *WordMap) AppendWordStr(word string, metadata DictWordMetadata) {
	m.Insert(NewWordMapEntryStr(word).WithMD(metadata))
}
func (m *WordMap) Extend(entries []WordMapEntry) {
	reserve := len(entries)
	if !m.IsEmpty() {
		reserve = (reserve + 1) / 2
	}
	m.Reserve(reserve)
	for _, entry := range entries {
		m.Insert(entry)
	}
}
func (m *WordMap) ExtendWords(entries []WordMapEntry) { m.Extend(entries) }

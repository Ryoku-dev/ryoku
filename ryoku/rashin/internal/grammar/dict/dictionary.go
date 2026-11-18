package dict

import (
	_ "embed"
	"ryoku-rashin/internal/grammar/text"
	"sort"
	"sync"
)

//go:embed dictionary.dict
var CuratedDictStr string

//go:embed annotations.json
var AnnotationsStr string

// Dictionary is a case-aware English word database. Returned rune slices and metadata are read-only.
type Dictionary interface {
	ContainsWord([]rune) bool
	ContainsWordStr(string) bool
	ContainsExactWord([]rune) bool
	ContainsExactWordStr(string) bool
	FuzzyMatch([]rune, uint8, int) []FuzzyMatchResult
	FuzzyMatchStr(string, uint8, int) []FuzzyMatchResult
	GetCorrectCapitalizationOf([]rune) ([]rune, bool)
	GetCorrectCapitalizationsOf([]rune) [][]rune
	GetWordMetadata([]rune) *DictWordMetadata
	GetWordMetadataStr(string) *DictWordMetadata
	GetWordMetadataExact([]rune) *DictWordMetadata
	WordsIter() [][]rune
	WordCount() int
	GetWordFromId(CanonicalWordId) ([]rune, bool)
	FindWordsWithPrefix([]rune) [][]rune
	FindWordsWithCommonPrefix([]rune) [][]rune
}
type FuzzyMatchResult struct {
	Word         []rune
	EditDistance uint8
	Metadata     *DictWordMetadata
}
type WordWithMetadata struct {
	Word     []rune
	Metadata DictWordMetadata
}
type MutableDictionary struct{ wordMap *WordMap }

func NewMutableDictionary() *MutableDictionary { return &MutableDictionary{wordMap: NewWordMap()} }
func FromRuneFiles(words, attrs string) (*MutableDictionary, error) {
	m, err := NewWordMapFromRuneFiles(words, attrs)
	if err != nil {
		return nil, err
	}
	return &MutableDictionary{wordMap: m}, nil
}
func (d *MutableDictionary) AppendWord(w []rune, m DictWordMetadata) { d.wordMap.AppendWord(w, m) }
func (d *MutableDictionary) AppendWordStr(w string, m DictWordMetadata) {
	d.wordMap.AppendWordStr(w, m)
}
func (d *MutableDictionary) ExtendWords(words []WordWithMetadata) {
	for _, w := range words {
		d.AppendWord(w.Word, w.Metadata)
	}
}
func (d *MutableDictionary) ContainsWord(w []rune) bool {
	id, _, _ := caseFoldedWordID(w)
	return d.wordMap.ContainsCaseFolded(id)
}
func (d *MutableDictionary) ContainsWordStr(w string) bool   { return d.ContainsWord([]rune(w)) }
func (d *MutableDictionary) ContainsExactWord(w []rune) bool { return d.GetWordMetadataExact(w) != nil }
func (d *MutableDictionary) ContainsExactWordStr(w string) bool {
	return d.ContainsExactWord([]rune(w))
}
func (d *MutableDictionary) GetWordMetadata(w []rune) *DictWordMetadata {
	id, _, _ := caseFoldedWordID(w)
	index, ok := d.wordMap.caseFoldedIndex(id)
	if !ok {
		return nil
	}
	if len(d.wordMap.alternates[id]) == 0 {
		return &d.wordMap.entries[index].Metadata
	}
	m := d.wordMap.entries[index].Metadata
	for _, i := range d.wordMap.alternates[id] {
		m.Merge(d.wordMap.entries[i].Metadata)
	}
	return &m
}
func (d *MutableDictionary) GetWordMetadataStr(w string) *DictWordMetadata {
	return d.GetWordMetadata([]rune(w))
}
func (d *MutableDictionary) GetWordMetadataExact(w []rune) *DictWordMetadata {
	e := d.wordMap.GetCanonical(NewCanonicalWordIdFromWordChars(w))
	if e == nil {
		return nil
	}
	return &e.Metadata
}
func (d *MutableDictionary) GetCorrectCapitalizationOf(w []rune) ([]rune, bool) {
	id, _, _ := caseFoldedWordID(w)
	index, ok := d.wordMap.caseFoldedIndex(id)
	if !ok || len(d.wordMap.alternates[id]) != 0 {
		return nil, false
	}
	return d.wordMap.entries[index].CanonicalSpelling, true
}
func (d *MutableDictionary) GetCorrectCapitalizationsOf(w []rune) [][]rune {
	e := d.wordMap.GetCaseFoldedChars(w)
	out := make([][]rune, 0, len(e))
	for _, v := range e {
		out = append(out, v.CanonicalSpelling)
	}
	return out
}
func (d *MutableDictionary) WordsIter() [][]rune {
	e := d.wordMap.Iter()
	out := make([][]rune, 0, len(e))
	for _, v := range e {
		out = append(out, v.CanonicalSpelling)
	}
	return out
}
func (d *MutableDictionary) WordCount() int { return d.wordMap.Len() }
func (d *MutableDictionary) GetWordFromId(id CanonicalWordId) ([]rune, bool) {
	e := d.wordMap.GetCanonical(id)
	if e == nil {
		return nil, false
	}
	return e.CanonicalSpelling, true
}
func (d *MutableDictionary) FindWordsWithPrefix(p []rune) [][]rune {
	var out [][]rune
	for _, e := range d.wordMap.Iter() {
		if hasRunePrefix(e.CanonicalSpelling, p) {
			out = append(out, e.CanonicalSpelling)
		}
	}
	return out
}
func (d *MutableDictionary) FindWordsWithCommonPrefix(w []rune) [][]rune {
	var out [][]rune
	for _, e := range d.wordMap.Iter() {
		if hasRunePrefix(w, e.CanonicalSpelling) {
			out = append(out, e.CanonicalSpelling)
		}
	}
	return out
}
func hasRunePrefix(w, p []rune) bool {
	if len(p) > len(w) {
		return false
	}
	for i, c := range p {
		if w[i] != c {
			return false
		}
	}
	return true
}
func runeLess(a, b []rune) bool {
	n := min(len(a), len(b))
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return len(a) < len(b)
}
func (d *MutableDictionary) FuzzyMatch(w []rune, maxDist uint8, limit int) []FuzzyMatchResult {
	q := normalized(w)
	lower := text.ToLower(q)
	var out []FuzzyMatchResult
	var a, b []int
	for _, e := range d.wordMap.Iter() {
		v := e.CanonicalSpelling
		if len(v) < max(1, len(q)-int(maxDist)) || len(v) > len(q)+int(maxDist) {
			continue
		}
		distance := levenshtein(q, v, &a, &b)
		if string(q) != string(lower) {
			distance = min(distance, levenshtein(lower, v, &a, &b))
		}
		if distance <= int(maxDist) {
			out = append(out, FuzzyMatchResult{v, uint8(distance), d.GetWordMetadataExact(v)})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].EditDistance < out[j].EditDistance })
	if limit < len(out) {
		out = out[:max(0, limit)]
	}
	return out
}
func (d *MutableDictionary) FuzzyMatchStr(w string, maxDist uint8, limit int) []FuzzyMatchResult {
	return d.FuzzyMatch([]rune(w), maxDist, limit)
}
func levenshtein(a, b []rune, x, y *[]int) int {
	if cap(*x) < len(b)+1 {
		*x = make([]int, len(b)+1)
	}
	if cap(*y) < len(b)+1 {
		*y = make([]int, len(b)+1)
	}
	prev, cur := (*x)[:len(b)+1], (*y)[:len(b)+1]
	for i := range prev {
		prev[i] = i
	}
	for i, c := range a {
		cur[0] = i + 1
		for j, d := range b {
			cost := 0
			if c != d {
				cost = 1
			}
			cur[j+1] = min(cur[j]+1, prev[j+1]+1, prev[j]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

// FstDictionary uses a compact trie instead of Rust's FST; search semantics are identical.
type FstDictionary struct {
	*MutableDictionary
	nodes     []trieNode
	edges     []trieEdge
	maxDepth  int
	wordOrder []int32
}
type trieNode struct{ start, count, entry int32 }
type trieEdge struct {
	letter rune
	child  int32
}

var curated = sync.OnceValue(func() *FstDictionary {
	d, err := FromRuneFiles(CuratedDictStr, AnnotationsStr)
	if err != nil {
		panic(err)
	}
	return FstDictionaryFromMutable(d)
})

func Curated() *FstDictionary { return curated() }

// CuratedMutableDictionary exposes the shared curated storage for read-only use.
func CuratedMutableDictionary() *MutableDictionary { return Curated().MutableDictionary }
func NewFstDictionary(words []WordWithMetadata) *FstDictionary {
	d := NewMutableDictionary()
	sort.Slice(words, func(i, j int) bool { return runeLess(words[i].Word, words[j].Word) })
	for _, w := range words {
		if !d.ContainsExactWord(w.Word) {
			d.AppendWord(w.Word, w.Metadata)
		}
	}
	return FstDictionaryFromMutable(d)
}
func FstDictionaryFromMutable(d *MutableDictionary) *FstDictionary {
	entries := d.wordMap.entries
	order := make([]int32, len(entries))
	for i := range order {
		order[i] = int32(i)
	}
	sort.Slice(order, func(i, j int) bool {
		return runeLess(entries[order[i]].CanonicalSpelling, entries[order[j]].CanonicalSpelling)
	})
	// Every distinct nonempty prefix is one node. Counting adjacent shared
	// prefixes avoids growing and copying the large node and edge arrays.
	nodeCount := 1
	maxDepth := 0
	var previous []rune
	for _, index := range order {
		word := entries[index].CanonicalSpelling
		shared := 0
		for shared < min(len(previous), len(word)) && previous[shared] == word[shared] {
			shared++
		}
		nodeCount += len(word) - shared
		maxDepth = max(maxDepth, len(word))
		previous = word
	}
	out := &FstDictionary{
		MutableDictionary: d,
		nodes:             make([]trieNode, 1, nodeCount),
		edges:             make([]trieEdge, 0, nodeCount-1),
		maxDepth:          maxDepth,
		wordOrder:         order,
	}
	var build func(int, []int32, int)
	build = func(node int, ws []int32, depth int) {
		out.nodes[node].entry = -1
		if len(ws) > 0 && len(entries[ws[0]].CanonicalSpelling) == depth {
			out.nodes[node].entry = ws[0]
			ws = ws[1:]
		}
		start := len(out.edges)
		for i := 0; i < len(ws); {
			c := entries[ws[i]].CanonicalSpelling[depth]
			j := i + 1
			for j < len(ws) && entries[ws[j]].CanonicalSpelling[depth] == c {
				j++
			}
			out.edges = append(out.edges, trieEdge{letter: c})
			i = j
		}
		count := len(out.edges) - start
		out.nodes[node].start = int32(start)
		out.nodes[node].count = int32(count)
		i := 0
		for e := start; e < start+count; e++ {
			j := i + 1
			for j < len(ws) && entries[ws[j]].CanonicalSpelling[depth] == entries[ws[i]].CanonicalSpelling[depth] {
				j++
			}
			child := len(out.nodes)
			out.nodes = append(out.nodes, trieNode{})
			out.edges[e].child = int32(child)
			build(child, ws[i:j], depth+1)
			i = j
		}
	}
	build(0, order, 0)
	return out
}
func (d *FstDictionary) FuzzyMatch(w []rune, maxDist uint8, limit int) []FuzzyMatchResult {
	q := normalized(w)
	lower := text.ToLower(q)
	best := map[CanonicalWordId]uint8{}
	search := func(query []rune) {
		rows := make([]int, (d.maxDepth+1)*(len(query)+1))
		width := len(query) + 1
		for j := range width {
			rows[j] = j
		}
		var walk func(int, int, rune)
		walk = func(node, depth int, last rune) {
			n := d.nodes[node]
			prev := rows[depth*width : (depth+1)*width]
			if n.entry != -1 && prev[len(query)] <= int(maxDist) {
				id := d.wordMap.entries[n.entry].WordIds.Canonical
				dist := uint8(prev[len(query)])
				if old, ok := best[id]; !ok || dist < old {
					best[id] = dist
				}
			}
			for _, e := range d.edges[n.start : n.start+n.count] {
				cur := rows[(depth+1)*width : (depth+2)*width]
				cur[0] = depth + 1
				small := cur[0]
				for j, c := range query {
					cost := 0
					if c != e.letter {
						cost = 1
					}
					v := min(cur[j]+1, prev[j+1]+1, prev[j]+cost)
					if depth > 0 && j > 0 && e.letter == query[j-1] && last == c {
						v = min(v, rows[(depth-1)*width+j-1]+1)
					}
					cur[j+1] = v
					small = min(small, v)
				}
				if small <= int(maxDist) {
					walk(int(e.child), depth+1, e.letter)
				}
			}
		}
		walk(0, 0, 0)
	}
	search(q)
	if string(q) != string(lower) {
		search(lower)
	}
	out := make([]FuzzyMatchResult, 0, len(best))
	for id, dist := range best {
		if dist == 0 {
			continue
		}
		word, _ := d.GetWordFromId(id)
		out = append(out, FuzzyMatchResult{word, dist, d.GetWordMetadataExact(word)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].EditDistance != out[j].EditDistance {
			return out[i].EditDistance < out[j].EditDistance
		}
		return runeLess(out[i].Word, out[j].Word)
	})
	if limit < len(out) {
		out = out[:max(0, limit)]
	}
	return out
}
func (d *FstDictionary) FuzzyMatchStr(w string, dist uint8, limit int) []FuzzyMatchResult {
	return d.FuzzyMatch([]rune(w), dist, limit)
}
func (d *FstDictionary) WordsIter() [][]rune {
	out := make([][]rune, len(d.wordOrder))
	for i, index := range d.wordOrder {
		out[i] = d.wordMap.entries[index].CanonicalSpelling
	}
	return out
}
func (d *FstDictionary) GetCorrectCapitalizationsOf(w []rune) [][]rune {
	out := d.MutableDictionary.GetCorrectCapitalizationsOf(w)
	if len(out) > 1 {
		sort.Slice(out, func(i, j int) bool { return runeLess(out[i], out[j]) })
	}
	return out
}
func (d *FstDictionary) GetWordMetadata(w []rune) *DictWordMetadata {
	id, _, _ := caseFoldedWordID(w)
	index, ok := d.wordMap.caseFoldedIndex(id)
	if !ok {
		return nil
	}
	if len(d.wordMap.alternates[id]) == 0 {
		return &d.wordMap.entries[index].Metadata
	}
	entries := d.wordMap.GetCaseFolded(id)
	sort.Slice(entries, func(i, j int) bool { return runeLess(entries[i].CanonicalSpelling, entries[j].CanonicalSpelling) })
	m := entries[0].Metadata
	for _, e := range entries[1:] {
		m.Merge(e.Metadata)
	}
	return &m
}
func (d *FstDictionary) GetWordMetadataStr(w string) *DictWordMetadata {
	return d.GetWordMetadata([]rune(w))
}

type TrieDictionary struct {
	Dictionary
	words [][]rune
}

func NewTrieDictionary(inner Dictionary) *TrieDictionary {
	words := inner.WordsIter()
	sort.Slice(words, func(i, j int) bool { return runeLess(words[i], words[j]) })
	return &TrieDictionary{inner, words}
}
func TrieDictionaryFromMutable(d *MutableDictionary) *TrieDictionary { return NewTrieDictionary(d) }

var curatedTrie = sync.OnceValue(func() *TrieDictionary { return NewTrieDictionary(Curated()) })

func CuratedTrieDictionary() *TrieDictionary { return curatedTrie() }
func (d *TrieDictionary) FindWordsWithPrefix(p []rune) [][]rune {
	start := sort.Search(len(d.words), func(i int) bool { return !runeLess(d.words[i], p) })
	end := start
	for end < len(d.words) && hasRunePrefix(d.words[end], p) {
		end++
	}
	return append([][]rune(nil), d.words[start:end]...)
}
func (d *TrieDictionary) FindWordsWithCommonPrefix(w []rune) [][]rune {
	var out [][]rune
	for i := range len(w) + 1 {
		prefix := w[:i]
		j := sort.Search(len(d.words), func(j int) bool { return !runeLess(d.words[j], prefix) })
		if j < len(d.words) && len(d.words[j]) == i && hasRunePrefix(w, d.words[j]) {
			out = append(out, d.words[j])
		}
	}
	return out
}

type MergedDictionary struct{ children []Dictionary }

func NewMergedDictionary() *MergedDictionary               { return &MergedDictionary{} }
func (d *MergedDictionary) AddDictionary(child Dictionary) { d.children = append(d.children, child) }
func (d *MergedDictionary) ContainsWord(w []rune) bool {
	for _, c := range d.children {
		if c.ContainsWord(w) {
			return true
		}
	}
	return false
}
func (d *MergedDictionary) ContainsWordStr(w string) bool { return d.ContainsWord([]rune(w)) }
func (d *MergedDictionary) ContainsExactWord(w []rune) bool {
	for _, c := range d.children {
		if c.ContainsExactWord(w) {
			return true
		}
	}
	return false
}
func (d *MergedDictionary) ContainsExactWordStr(w string) bool { return d.ContainsExactWord([]rune(w)) }
func (d *MergedDictionary) GetWordMetadata(w []rune) *DictWordMetadata {
	var out *DictWordMetadata
	owned := false
	for _, c := range d.children {
		if m := c.GetWordMetadata(w); m != nil {
			if out == nil {
				out = m
			} else {
				if !owned {
					out = new(*out)
					owned = true
				}
				out.Merge(*m)
			}
		}
	}
	return out
}
func (d *MergedDictionary) GetWordMetadataStr(w string) *DictWordMetadata {
	return d.GetWordMetadata([]rune(w))
}
func (d *MergedDictionary) GetWordMetadataExact(w []rune) *DictWordMetadata {
	for _, c := range d.children {
		if m := c.GetWordMetadataExact(w); m != nil {
			return m
		}
	}
	return nil
}
func (d *MergedDictionary) GetCorrectCapitalizationOf(w []rune) ([]rune, bool) {
	for _, c := range d.children {
		if r, ok := c.GetCorrectCapitalizationOf(w); ok {
			return r, true
		}
	}
	return nil, false
}
func (d *MergedDictionary) GetCorrectCapitalizationsOf(w []rune) [][]rune {
	var out [][]rune
	for _, c := range d.children {
		out = append(out, c.GetCorrectCapitalizationsOf(w)...)
	}
	return out
}
func (d *MergedDictionary) WordsIter() [][]rune {
	var out [][]rune
	for _, c := range d.children {
		out = append(out, c.WordsIter()...)
	}
	return out
}
func (d *MergedDictionary) WordCount() int {
	n := 0
	for _, c := range d.children {
		n += c.WordCount()
	}
	return n
}
func (d *MergedDictionary) GetWordFromId(id CanonicalWordId) ([]rune, bool) {
	for _, c := range d.children {
		if r, ok := c.GetWordFromId(id); ok {
			return r, true
		}
	}
	return nil, false
}
func uniqueWords(w [][]rune) [][]rune {
	sort.Slice(w, func(i, j int) bool { return runeLess(w[i], w[j]) })
	out := w[:0]
	for _, v := range w {
		if len(out) == 0 || string(out[len(out)-1]) != string(v) {
			out = append(out, v)
		}
	}
	return out
}
func (d *MergedDictionary) FindWordsWithPrefix(w []rune) [][]rune {
	var out [][]rune
	for _, c := range d.children {
		out = append(out, c.FindWordsWithPrefix(w)...)
	}
	return uniqueWords(out)
}
func (d *MergedDictionary) FindWordsWithCommonPrefix(w []rune) [][]rune {
	var out [][]rune
	for _, c := range d.children {
		out = append(out, c.FindWordsWithCommonPrefix(w)...)
	}
	return uniqueWords(out)
}
func (d *MergedDictionary) FuzzyMatch(w []rune, dist uint8, limit int) []FuzzyMatchResult {
	var out []FuzzyMatchResult
	for _, c := range d.children {
		out = append(out, c.FuzzyMatch(w, dist, limit)...)
	}
	sort.SliceStable(out, func(i, j int) bool { return runeLess(out[i].Word, out[j].Word) })
	result := out[:0]
	for _, v := range out {
		if len(result) == 0 || string(result[len(result)-1].Word) != string(v.Word) {
			result = append(result, v)
		}
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].EditDistance < result[j].EditDistance })
	if limit < len(result) {
		result = result[:max(0, limit)]
	}
	return result
}
func (d *MergedDictionary) FuzzyMatchStr(w string, dist uint8, limit int) []FuzzyMatchResult {
	return d.FuzzyMatch([]rune(w), dist, limit)
}

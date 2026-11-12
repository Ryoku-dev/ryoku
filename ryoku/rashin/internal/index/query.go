package index

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/index/embed"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/rank"
	"ryoku-rashin/internal/redact"
)

func (e *Engine) queryStore(ctx context.Context, ref protocol.IndexRef) (registryEntry, *indexStore, error) {
	entry, err := e.resolve(ctx, ref, true)
	if err != nil {
		return registryEntry{}, nil, err
	}
	if err := e.ensureFresh(ctx, entry); err != nil {
		return registryEntry{}, nil, err
	}
	store, err := e.storeFor(entry.ID)
	return entry, store, err
}
func queryLimit(value int) int {
	if value <= 0 {
		return 20
	}
	if value > 200 {
		return 200
	}
	return value
}

func (e *Engine) Search(ctx context.Context, p protocol.SearchParams) (protocol.SearchResult, error) {
	var entry registryEntry
	var store *indexStore
	var err error
	if p.Existing {
		entry, err = e.resolve(ctx, p.IndexRef, false)
		if err == nil {
			store, err = e.storeFor(entry.ID)
		}
		if err == nil {
			e.queueRefreshCheck(entry)
		}
	} else {
		entry, store, err = e.queryStore(ctx, p.IndexRef)
	}
	if err != nil {
		return protocol.SearchResult{}, err
	}
	limit := queryLimit(p.Limit)
	wide := max(limit*4, 40)
	lexical, err := store.bm25(p.Query, wide)
	if err != nil {
		return protocol.SearchResult{}, err
	}
	semanticEnabled := e.currentSettings().Index.Semantic && p.Mode != protocol.SearchLexical
	var semantic []rankedChunk
	used := false
	if semanticEnabled {
		_, chunks, _, vectors, countErr := store.storedCounts()
		if countErr != nil {
			return protocol.SearchResult{}, countErr
		}
		if vectors < chunks && chunks > 0 {
			e.startEmbedding(entry, store)
		}
		if vectors > 0 {
			model, loadErr := embed.Load()
			if loadErr == nil {
				encoded, _ := model.Embed(ctx, []string{p.Query})
				if len(encoded) == 1 {
					semantic, err = store.vectorSearch(encoded[0], wide)
					if err != nil {
						return protocol.SearchResult{}, err
					}
					used = len(semantic) > 0
				}
			}
		}
	}
	var ranked []rankedChunk
	switch {
	case p.Mode == protocol.SearchSemantic && used:
		ranked = semantic
		if len(ranked) > limit {
			ranked = ranked[:limit]
		}
	case used:
		ranked = fuseRanks(limit, semantic, lexical)
	default:
		if len(lexical) > limit {
			lexical = lexical[:limit]
		}
		ranked = lexical
	}
	return protocol.SearchResult{Index: entry.ID, Hits: searchHits(ranked, p.Query), Semantic: used}, nil
}

func (e *Engine) Find(ctx context.Context, p protocol.NameParams) (protocol.SymbolsResult, error) {
	entry, store, err := e.queryStore(ctx, p.IndexRef)
	if err != nil {
		return protocol.SymbolsResult{}, err
	}
	records, err := store.symbols()
	if err != nil {
		return protocol.SymbolsResult{}, err
	}
	query := strings.ToLower(strings.TrimSpace(p.Name))
	type candidate struct {
		record   symbolRecord
		rank     int
		distance int
	}
	var found []candidate
	for _, record := range records {
		name := strings.ToLower(record.Name)
		bare := name
		if cut := strings.LastIndexAny(bare, ".:"); cut >= 0 {
			bare = bare[cut+1:]
		}
		rank := 99
		switch {
		case strings.EqualFold(record.ID, p.Name):
			rank = 0
		case name == query || bare == query:
			rank = 0
		case strings.HasPrefix(name, query) || strings.HasPrefix(bare, query):
			rank = 1
		case containsPart(record.Name, query):
			rank = 2
		default:
			distance := editDistance(name, query)
			if distance <= max(2, len([]rune(query))/3) {
				rank = 3
				found = append(found, candidate{record: record, rank: rank, distance: distance})
				continue
			}
		}
		if rank < 99 {
			found = append(found, candidate{record: record, rank: rank})
		}
	}
	sort.SliceStable(found, func(i, j int) bool {
		if found[i].rank != found[j].rank {
			return found[i].rank < found[j].rank
		}
		if found[i].distance != found[j].distance {
			return found[i].distance < found[j].distance
		}
		if declared, other := strings.HasSuffix(found[i].record.Signature, ";"), strings.HasSuffix(found[j].record.Signature, ";"); declared != other {
			return other
		}
		if found[i].record.Path != found[j].record.Path {
			return found[i].record.Path < found[j].record.Path
		}
		return found[i].record.Start < found[j].record.Start
	})
	limit := queryLimit(p.Limit)
	if len(found) > limit {
		found = found[:limit]
	}
	symbols := make([]protocol.Symbol, 0, len(found))
	for _, item := range found {
		symbols = append(symbols, protocolSymbol(item.record))
	}
	return protocol.SymbolsResult{Index: entry.ID, Symbols: symbols}, nil
}
func protocolSymbol(item symbolRecord) protocol.Symbol {
	return protocol.Symbol{ID: item.ID, Name: item.Name, Kind: item.Kind, Signature: item.Signature, Path: item.Path, Start: item.Start, End: item.End, Parent: item.Parent, Doc: item.Doc, Depth: item.Depth}
}

// indexedPath turns a path the CLI resolved against its working folder into
// the root-relative form the index stores; a relative path is taken as is
func indexedPath(entry registryEntry, path string) string {
	if filepath.IsAbs(path) {
		absolute := filepath.Clean(path)
		if resolved, err := filepath.EvalSymlinks(absolute); err == nil {
			absolute = resolved
		}
		for _, root := range entry.Roots {
			if relative, err := filepath.Rel(root, absolute); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				return filepath.ToSlash(relative)
			}
		}
	}
	return filepath.ToSlash(filepath.Clean(path))
}
func containsPart(name, query string) bool {
	for _, part := range rank.SplitIdentifier(name) {
		if part == query || strings.HasPrefix(part, query) {
			return true
		}
	}
	return false
}
func editDistance(a, b string) int {
	left, right := []rune(a), []rune(b)
	previous := make([]int, len(right)+1)
	for i := range previous {
		previous[i] = i
	}
	for i, x := range left {
		current := make([]int, len(right)+1)
		current[0] = i + 1
		for j, y := range right {
			cost := 1
			if x == y {
				cost = 0
			}
			current[j+1] = min(current[j]+1, previous[j+1]+1, previous[j]+cost)
		}
		previous = current
	}
	return previous[len(right)]
}

func (e *Engine) Def(ctx context.Context, p protocol.NameParams) (protocol.DefResult, error) {
	found, err := e.Find(ctx, protocol.NameParams{IndexRef: p.IndexRef, Name: p.Name, Limit: 200})
	if err != nil {
		return protocol.DefResult{}, err
	}
	if len(found.Symbols) == 0 {
		return protocol.DefResult{}, fmt.Errorf("%s", i18n.Tf("Symbol %s was not found.", p.Name))
	}
	entry, store, err := e.queryStore(ctx, p.IndexRef)
	if err != nil {
		return protocol.DefResult{}, err
	}
	symbol := found.Symbols[0]
	file, ok, err := store.file(symbol.Path)
	if err != nil || !ok {
		if err == nil {
			err = os.ErrNotExist
		}
		return protocol.DefResult{}, err
	}
	body, _, _, truncated, err := readLines(file.Source, symbol.Start, symbol.End, 200, 4<<20)
	if err != nil {
		return protocol.DefResult{}, err
	}
	var others []protocol.Symbol
	for _, candidate := range found.Symbols[1:] {
		if candidate.Name == symbol.Name {
			others = append(others, candidate)
		}
	}
	return protocol.DefResult{Index: entry.ID, Symbol: symbol, Body: body, Truncated: truncated, Others: others}, nil
}
func (e *Engine) Outline(ctx context.Context, p protocol.PathParams) (protocol.SymbolsResult, error) {
	entry, store, err := e.queryStore(ctx, p.IndexRef)
	if err != nil {
		return protocol.SymbolsResult{}, err
	}
	records, err := store.symbols()
	if err != nil {
		return protocol.SymbolsResult{}, err
	}
	path := indexedPath(entry, p.Path)
	var symbols []protocol.Symbol
	for _, record := range records {
		if record.Path == path {
			symbols = append(symbols, protocolSymbol(record))
		}
	}
	sort.SliceStable(symbols, func(i, j int) bool {
		if symbols[i].Start != symbols[j].Start {
			return symbols[i].Start < symbols[j].Start
		}
		return symbols[i].Depth < symbols[j].Depth
	})
	return protocol.SymbolsResult{Index: entry.ID, Symbols: symbols}, nil
}
func (e *Engine) References(ctx context.Context, p protocol.NameParams) (protocol.ReferencesResult, error) {
	entry, store, err := e.queryStore(ctx, p.IndexRef)
	if err != nil {
		return protocol.ReferencesResult{}, err
	}
	files, err := store.files()
	if err != nil {
		return protocol.ReferencesResult{}, err
	}
	symbols, _ := store.symbols()
	defs := map[string]map[int]bool{}
	byFile := map[string][]symbolRecord{}
	for _, symbol := range symbols {
		byFile[symbol.Path] = append(byFile[symbol.Path], symbol)
		if symbol.Name == p.Name || strings.HasSuffix(symbol.Name, "."+p.Name) {
			if defs[symbol.Path] == nil {
				defs[symbol.Path] = map[int]bool{}
			}
			defs[symbol.Path][symbol.Start] = true
		}
	}
	limit := queryLimit(p.Limit)
	var refs []protocol.Reference
	for _, file := range files {
		source, readErr := os.ReadFile(file.Source)
		if readErr != nil {
			continue
		}
		scanner := bufio.NewScanner(strings.NewReader(redact.Patterns(string(source), nil)))
		scanner.Buffer(make([]byte, 64<<10), 4<<20)
		line := 0
		for scanner.Scan() {
			line++
			text := scanner.Text()
			if !defs[file.Path][line] && wholeWord(text, p.Name) {
				refs = append(refs, protocol.Reference{Path: file.Path, Line: line, Text: strings.TrimSpace(text), Symbol: enclosingSymbol(byFile[file.Path], line)})
				if len(refs) >= limit {
					break
				}
			}
		}
		if len(refs) >= limit {
			break
		}
	}
	return protocol.ReferencesResult{Index: entry.ID, Name: p.Name, References: refs}, nil
}
func enclosingSymbol(symbols []symbolRecord, line int) string {
	name := ""
	width := int(^uint(0) >> 1)
	for _, symbol := range symbols {
		if symbol.Start <= line && line <= symbol.End && symbol.End-symbol.Start < width {
			name = symbol.Name
			width = symbol.End - symbol.Start
		}
	}
	return name
}
func wholeWord(text, name string) bool {
	for offset := 0; offset <= len(text)-len(name); {
		index := strings.Index(text[offset:], name)
		if index < 0 {
			return false
		}
		index += offset
		before := index == 0 || !identifierByte(text[index-1])
		after := index+len(name) == len(text) || !identifierByte(text[index+len(name)])
		if before && after {
			return true
		}
		offset = index + 1
	}
	return false
}
func identifierByte(value byte) bool {
	return value == '_' || value >= '0' && value <= '9' || value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

func (e *Engine) Peek(ctx context.Context, p protocol.PeekParams) (protocol.PeekResult, error) {
	entry, store, err := e.queryStore(ctx, p.IndexRef)
	if err != nil {
		return protocol.PeekResult{}, err
	}
	path := indexedPath(entry, p.Path)
	file, ok, err := store.file(path)
	if err != nil {
		return protocol.PeekResult{}, err
	}
	if !ok {
		return protocol.PeekResult{}, os.ErrNotExist
	}
	end := p.End
	if end <= 0 {
		end = p.Start + 199
	}
	text, start, end, truncated, err := readLines(file.Source, p.Start, end, 200, 4<<20)
	if err != nil {
		return protocol.PeekResult{}, err
	}
	return protocol.PeekResult{Index: entry.ID, Path: path, Start: start, End: end, Text: text, Truncated: truncated}, nil
}
func readLines(path string, start, end, maxLines int, maxBytes int64) (string, int, int, bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", 0, 0, false, err
	}
	if info.Size() > maxBytes {
		return "", 0, 0, false, fmt.Errorf("%s", i18n.Tf("%s is larger than the 4 MiB read limit.", filepath.Base(path)))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", 0, 0, false, err
	}
	lines := strings.Split(redact.Patterns(string(data), nil), "\n")
	if start < 1 {
		start = 1
	}
	if start > len(lines) {
		return "", start, start, false, nil
	}
	if end < start {
		end = start
	}
	if end > len(lines) {
		end = len(lines)
	}
	truncated := end-start+1 > maxLines
	if truncated {
		end = start + maxLines - 1
	}
	return strings.Join(lines[start-1:end], "\n"), start, end, truncated, nil
}

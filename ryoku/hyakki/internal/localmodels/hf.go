package localmodels

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/protocol"
)

const huggingFace = "https://huggingface.co"

// ParseHFInput accepts an owner/repository, repository URL, or a pinned-file
// URL. The returned filename is a suggestion; searchHF still pins the current
// repository revision and verifies the file in its tree.
func ParseHFInput(input string) (repo, file string, err error) {
	input = strings.TrimSpace(input)
	if validRepo(input) {
		return input, "", nil
	}
	u, parseErr := url.Parse(input)
	if parseErr != nil || u.Scheme != "https" || u.Host != "huggingface.co" && u.Host != "www.huggingface.co" ||
		u.User != nil || u.Fragment != "" || u.RawQuery != "" && u.RawQuery != "download=true" || u.Opaque != "" {
		return "", "", protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Enter a Hugging Face repository or GGUF file URL."))
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 || !validRepo(parts[0]+"/"+parts[1]) {
		return "", "", protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Enter a Hugging Face repository as owner/name."))
	}
	repo = parts[0] + "/" + parts[1]
	if len(parts) == 2 {
		return repo, "", nil
	}
	if len(parts) < 5 || parts[2] != "blob" && parts[2] != "resolve" || parts[3] == "" {
		return "", "", protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Enter a Hugging Face GGUF file URL."))
	}
	file = strings.Join(parts[4:], "/")
	if !validName(file) || !strings.EqualFold(path.Ext(file), ".gguf") {
		return "", "", protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Choose a GGUF file URL."))
	}
	return repo, file, nil
}

// searchHF pins a repository before listing its files. The tree request uses
// that revision, not main, so a later download cannot silently change files.
func (m *Manager) searchHF(ctx context.Context, input string) (protocol.LocalSearchResult, error) {
	repo, selected, err := ParseHFInput(input)
	if err != nil {
		return protocol.LocalSearchResult{}, err
	}
	var meta struct {
		SHA      string          `json:"sha"`
		Gated    json.RawMessage `json:"gated"`
		CardData struct {
			License     string `json:"license"`
			LicenseName string `json:"license_name"`
		} `json:"cardData"`
	}
	endpoint := m.hfBase + "/api/models/" + repo
	if err := m.hfJSON(ctx, endpoint, &meta); err != nil {
		return protocol.LocalSearchResult{}, err
	}
	if len(meta.SHA) != 40 || !hexRevision(meta.SHA) {
		return protocol.LocalSearchResult{}, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("Hugging Face returned no pinned revision for this repository."))
	}
	result := protocol.LocalSearchResult{Repo: repo, Revision: meta.SHA, License: meta.CardData.LicenseName, Gated: string(meta.Gated) != "false"}
	if result.License == "" {
		result.License = meta.CardData.License
	}
	if result.Gated {
		return result, nil
	}
	// HF paginates tree results with Link: <...>; rel="next".
	endpoint = m.hfBase + "/api/models/" + repo + "/tree/" + meta.SHA + "?recursive=true&expand=true"
	headers := make(map[string]ggufMetadata)
	for page := 0; endpoint != "" && page < 100; page++ {
		var rows []struct {
			Path string `json:"path"`
			Type string `json:"type"`
			Size int64  `json:"size"`
			LFS  struct {
				OID  string `json:"oid"`
				Size int64  `json:"size"`
			} `json:"lfs"`
			GGUF json.RawMessage `json:"gguf"`
		}
		next, err := m.hfPage(ctx, endpoint, &rows)
		if err != nil {
			return protocol.LocalSearchResult{}, err
		}
		for _, row := range rows {
			if row.Type != "file" || !strings.HasSuffix(strings.ToLower(row.Path), ".gguf") || !validName(row.Path) || !sha256Pattern.MatchString(row.LFS.OID) {
				continue
			}
			size := row.LFS.Size
			if size == 0 {
				size = row.Size
			}
			if size <= 0 {
				continue
			}
			role := protocol.LocalFileModel
			if strings.HasPrefix(strings.ToLower(row.Path[strings.LastIndexByte(row.Path, '/')+1:]), "mmproj") {
				role = protocol.LocalFileMmproj
			}
			file := protocol.LocalFile{Name: row.Path, Role: role, Size: size, SHA256: row.LFS.OID}
			if role == protocol.LocalFileModel {
				meta, known := hfGGUFMetadata(row.GGUF)
				if !known && splitPart(row.Path) <= 1 {
					if header, err := m.hfHeader(ctx, repo, result.Revision, file.Name); err == nil {
						meta = header
					}
				}
				headers[file.Name] = meta
				file.Params, file.Context, file.Tools = meta.Params, meta.Context, meta.Tools
				file.Warning = modelWarning(meta)
			}
			result.Files = append(result.Files, file)
		}
		endpoint = next
	}
	if endpoint != "" {
		return protocol.LocalSearchResult{}, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("The repository lists too many files to inspect safely."))
	}
	sort.Slice(result.Files, func(i, j int) bool { return result.Files[i].Name < result.Files[j].Name })
	if selected != "" {
		for _, file := range result.Files {
			if file.Name == selected && file.Role == protocol.LocalFileModel {
				parts := matchGGUFFiles(result.Files, selected)
				if len(parts) == 0 {
					return protocol.LocalSearchResult{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The GGUF split set is incomplete."))
				}
				result.Path = parts[0].Name
				break
			}
		}
		if result.Path == "" {
			return protocol.LocalSearchResult{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That GGUF file is not in the repository's current revision."))
		}
	}
	rankHFFiles(&result, hostHardware(m.Devices(ctx)), headers)
	return result, nil
}

func hfGGUFMetadata(raw json.RawMessage) (ggufMetadata, bool) {
	var fields map[string]any
	if json.Unmarshal(raw, &fields) != nil || len(fields) == 0 {
		return ggufMetadata{}, false
	}
	if inner, ok := fields["metadata"].(map[string]any); ok {
		fields = inner
	}
	number := func(names ...string) int64 {
		for _, key := range names {
			switch n := fields[key].(type) {
			case float64:
				if n > 0 && n <= 1<<62 {
					return int64(n)
				}
			case string:
				if v, err := strconv.ParseInt(n, 10, 64); err == nil {
					return v
				}
			}
		}
		return 0
	}
	var result ggufMetadata
	result.Params = number("general.parameter_count", "parameters", "parameter_count")
	arch, _ := fields["general.architecture"].(string)
	result.Architecture = arch
	scalar := func(key string) int {
		v := number(arch+"."+key, key)
		if v > 0 && v <= math.MaxInt32 {
			return int(v)
		}
		return 0
	}
	result.Context = scalar("context_length")
	result.Blocks = scalar("block_count")
	result.Embedding = scalar("embedding_length")
	result.Heads = scalar("attention.head_count")
	result.KVHeads = scalar("attention.head_count_kv")
	result.KeyLength = scalar("attention.key_length")
	result.ValueLength = scalar("attention.value_length")
	result.SlidingWindow = scalar("attention.sliding_window")
	template, ok := fields["tokenizer.chat_template"].(string)
	if !ok {
		template, ok = fields["chat_template"].(string)
	}
	result.Tools = strings.Contains(template, "tools") && (strings.Contains(template, "tool_call") || strings.Contains(template, "function"))
	return result, ok && result.Context > 0 && result.Params > 0
}

var splitGGUF = regexp.MustCompile(`(?i)^(.+)-([0-9]{5})-of-([0-9]{5})\.gguf$`)

func splitPart(name string) int {
	m := splitGGUF.FindStringSubmatch(name)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(m[2])
	return n
}

func completeSplitNames(names []string) bool {
	if len(names) < 2 {
		return false
	}
	first := splitGGUF.FindStringSubmatch(names[0])
	if first == nil {
		return false
	}
	total, _ := strconv.Atoi(first[3])
	if total != len(names) || total > 99999 {
		return false
	}
	seen := make([]bool, total)
	for _, name := range names {
		part := splitGGUF.FindStringSubmatch(name)
		if part == nil || part[1] != first[1] || part[3] != first[3] {
			return false
		}
		n, _ := strconv.Atoi(part[2])
		if n < 1 || n > total || seen[n-1] {
			return false
		}
		seen[n-1] = true
	}
	return true
}

// matchGGUFFiles selects every part of one complete split set, or a single
// unsplit GGUF. An incomplete split set is never offered for installation.
func matchGGUFFiles(files []protocol.LocalFile, chosen string) []protocol.LocalFile {
	match := splitGGUF.FindStringSubmatch(chosen)
	if match == nil {
		for _, file := range files {
			if file.Name == chosen && file.Role == protocol.LocalFileModel {
				return []protocol.LocalFile{file}
			}
		}
		return nil
	}
	var parts []protocol.LocalFile
	var names []string
	for _, file := range files {
		other := splitGGUF.FindStringSubmatch(file.Name)
		if file.Role == protocol.LocalFileModel && other != nil && other[1] == match[1] && other[3] == match[3] {
			parts = append(parts, file)
			names = append(names, file.Name)
		}
	}
	if !completeSplitNames(names) {
		return nil
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i].Name < parts[j].Name })
	return parts
}

var quantName = regexp.MustCompile(`(?i)(?:^|[-_.])((?:IQ[1-4]|Q[2-8])(?:_[A-Z0-9]+)*|BF16|F16|F32)(?:[-_.]|$)`)

func quantAndQuality(name string) (string, int) {
	match := quantName.FindStringSubmatch(strings.ToUpper(path.Base(name)))
	if match == nil {
		return "", 0
	}
	q := match[1]
	switch {
	case q == "F32":
		return q, 120
	case q == "F16" || q == "BF16":
		return q, 110
	case strings.HasPrefix(q, "Q8"):
		return q, 100
	case strings.HasPrefix(q, "Q6"):
		return q, 90
	case strings.HasPrefix(q, "Q5_K_M"):
		return q, 82
	case strings.HasPrefix(q, "Q5"):
		return q, 80
	case strings.HasPrefix(q, "Q4_K_M"):
		return q, 72
	case strings.HasPrefix(q, "Q4"):
		return q, 71
	case strings.HasPrefix(q, "IQ4"):
		return q, 69
	case strings.HasPrefix(q, "Q3_K_M"), strings.HasPrefix(q, "Q3_K_L"):
		return q, 42
	case strings.HasPrefix(q, "IQ3_XS"):
		return q, 41
	default:
		return q, 0
	}
}

func matchingMmproj(files []protocol.LocalFile, model string) string {
	var projectors []protocol.LocalFile
	for _, file := range files {
		if file.Role == protocol.LocalFileMmproj {
			projectors = append(projectors, file)
		}
	}
	if len(projectors) == 1 {
		return projectors[0].Name
	}
	quant, _ := quantAndQuality(model)
	var generic string
	var genericCount int
	for _, f := range projectors {
		if path.Dir(f.Name) != path.Dir(model) {
			continue
		}
		q, _ := quantAndQuality(f.Name)
		if q != "" && q == quant {
			return f.Name
		}
		if q == "" {
			generic, genericCount = f.Name, genericCount+1
		}
	}
	if genericCount == 1 {
		return generic
	}
	return ""
}

// rankHFFiles chooses usable context before quality, enforcing a quality
// floor even when tiny quants would otherwise fit the graphics card.
func rankHFFiles(result *protocol.LocalSearchResult, h hardwareProfile, headers map[string]ggufMetadata) {
	best, bestScore := "", -1
	fallback, fallbackQuality := "", -1
	scores := make(map[string]int, len(result.Files))
	for _, file := range result.Files {
		if file.Role == protocol.LocalFileModel {
			scores[file.Name] = -2000
		}
	}
	for i := range result.Files {
		f := &result.Files[i]
		if f.Role != protocol.LocalFileModel || !strings.EqualFold(path.Ext(f.Name), ".gguf") {
			continue
		}
		parts := matchGGUFFiles(result.Files, f.Name)
		if len(parts) == 0 || parts[0].Name != f.Name {
			continue
		}
		meta := headers[f.Name]
		meta.Params, meta.Context, meta.Tools = f.Params, f.Context, f.Tools
		var e Entry
		for _, part := range parts {
			meta.ModelBytes += part.Size
			e.Files = append(e.Files, File{Name: part.Name, Role: part.Role, Size: part.Size, SHA256: part.SHA256})
		}
		projector := matchingMmproj(result.Files, f.Name)
		for _, other := range result.Files {
			if other.Name == projector {
				meta.MmprojBytes = other.Size
				break
			}
		}
		plan := planModel(e, meta, h, protocol.LocalModelOverride{})
		f.Plan, f.Verdict = plan, plan.Verdict
		q, quality := quantAndQuality(f.Name)
		f.Quant, f.Parts = q, len(parts)
		if q == "" {
			quality = 60
		}
		fallbackScore := quality
		if quality == 72 {
			fallbackScore = 1000
		} else if quality >= 69 && quality < 80 {
			fallbackScore += 900
		}
		if quality >= 40 && fallbackScore > fallbackQuality {
			fallback, fallbackQuality = f.Name, fallbackScore
		}
		score := -1000
		if quality >= 40 && plan.Verdict != protocol.LocalVerdictTooBig {
			// A split Q4 is preferable to a Q3 that fits entirely on the card.
			tier := 0
			if quality >= 69 {
				tier = 1
			}
			if plan.Verdict == protocol.LocalVerdictSplit && quality > 72 {
				tier = 0
			}
			device := 0
			switch plan.Verdict {
			case protocol.LocalVerdictCard:
				device = 3
			case protocol.LocalVerdictShared, protocol.LocalVerdictSplit:
				device = 2
			case protocol.LocalVerdictCPU:
				device = 1
			}
			score = min(plan.Context, 32768)*1000 + tier*100_000 + device*10_000 + quality
		}
		for _, part := range parts {
			scores[part.Name] = score
		}
		if score > bestScore {
			best, bestScore = f.Name, score
		}
	}
	if best == "" {
		best = fallback
	}
	result.Recommended = best
	if result.Path == "" {
		result.Path = best
	}
	result.Mmproj = matchingMmproj(result.Files, result.Path)
	for i := range result.Files {
		result.Files[i].Recommended = result.Files[i].Name == best
	}
	// Keep split parts together by score and filename. Unfittable models
	// follow runnable ones; projectors follow all model weights.
	sort.SliceStable(result.Files, func(i, j int) bool {
		a, b := result.Files[i], result.Files[j]
		if a.Role != b.Role {
			return a.Role == protocol.LocalFileModel
		}
		if scores[a.Name] != scores[b.Name] {
			return scores[a.Name] > scores[b.Name]
		}
		return a.Name < b.Name
	})
}

// hfHeader reads only the GGUF header at the pinned revision. It never
// downloads weights, even if a remote server ignores the Range header.
func (m *Manager) hfHeader(ctx context.Context, repo, revision, name string) (ggufMetadata, error) {
	e := Entry{Repo: repo, Revision: revision}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.url(m.hfBase, name), nil)
	if err != nil {
		return ggufMetadata{}, err
	}
	req.Header.Set("Range", "bytes=0-"+strconv.Itoa(ggufHeaderLimit-1))
	resp, err := m.client.Do(req)
	if err != nil {
		return ggufMetadata{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return ggufMetadata{}, fmt.Errorf("GGUF header: %s", resp.Status)
	}
	if resp.StatusCode == http.StatusPartialContent && !strings.HasPrefix(resp.Header.Get("Content-Range"), "bytes 0-") {
		return ggufMetadata{}, errGGUFHeader
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, ggufHeaderLimit))
	if err != nil {
		return ggufMetadata{}, err
	}
	return parseGGUF(data)
}
func hexRevision(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' && c < 'a' || c > 'f' {
			return false
		}
	}
	return true
}

func (m *Manager) hfJSON(ctx context.Context, endpoint string, dest any) error {
	_, err := m.hfPage(ctx, endpoint, dest)
	return err
}

func (m *Manager) hfPage(ctx context.Context, endpoint string, dest any) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.Tf("Hugging Face answered %s.", resp.Status))
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(dest); err != nil {
		return "", fmt.Errorf("%s: %w", i18n.T("Hugging Face returned an unreadable model list"), err)
	}
	for _, part := range strings.Split(resp.Header.Get("Link"), ",") {
		if !strings.Contains(part, `rel="next"`) {
			continue
		}
		left := strings.IndexByte(part, '<')
		right := strings.IndexByte(part, '>')
		if left < 0 || right <= left {
			continue
		}
		next := part[left+1 : right]
		u, err := url.Parse(next)
		if err != nil || u.Scheme != "https" || u.Host != "huggingface.co" || !strings.HasPrefix(u.Path, "/api/models/") {
			return "", protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("Hugging Face linked to an unexpected next page."))
		}
		return next, nil
	}
	return "", nil
}

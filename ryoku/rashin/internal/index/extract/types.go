// Package extract turns supported source files into symbols, imports, and chunks.
package extract

import (
	"path/filepath"
	"strings"
)

type Symbol struct {
	Name      string
	Kind      string
	Signature string
	Start     int
	End       int
	Parent    string
	Doc       string
	Depth     int
}

type Import struct {
	Target string
	Line   int
}

type Result struct {
	Language string
	Symbols  []Symbol
	Imports  []Import
	Chunks   []Chunk
}

type Chunk struct {
	Start  int
	End    int
	Text   string
	Symbol string
}

func Detect(path string, head []byte) string {
	base := strings.ToLower(filepath.Base(path))
	ext := strings.ToLower(filepath.Ext(path))
	if base == "go.mod" {
		return "gomod"
	}
	switch ext {
	case ".go":
		return "go"
	case ".py", ".pyw":
		return "python"
	case ".js", ".mjs", ".cjs":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".rs":
		return "rust"
	case ".c", ".h":
		return "c"
	case ".cc", ".cpp", ".cxx", ".hpp", ".hh", ".hxx":
		return "cpp"
	case ".lua":
		return "lua"
	case ".sh", ".bash":
		return "bash"
	case ".fish":
		return "fish"
	case ".qml":
		return "qml"
	case ".kdl":
		return "kdl"
	case ".md", ".markdown":
		return "markdown"
	case ".json", ".jsonc":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".toml":
		return "toml"
	case ".ini", ".conf":
		return "ini"
	case ".css", ".scss":
		return "css"
	}
	if ext == "" {
		first := string(head)
		if strings.HasPrefix(first, "#!") {
			if strings.Contains(first, "python") {
				return "python"
			}
			if strings.Contains(first, "fish") {
				return "fish"
			}
			if strings.Contains(first, "bash") || strings.Contains(first, "/sh") {
				return "bash"
			}
		}
		if base == "dockerfile" {
			return "bash"
		}
	}
	return ""
}

func Extract(path string, source []byte) (result Result, err error) {
	language := Detect(path, source[:min(len(source), 512)])
	ext := strings.ToLower(filepath.Ext(path))
	defer func() {
		if recover() != nil {
			result = Result{Language: language}
			err = nil
		}
	}()
	header := ext == ".h" || ext == ".hh" || ext == ".hpp" || ext == ".hxx"
	if language == "c" && header && looksCpp(source) {
		language = "cpp"
	}
	switch language {
	case "go":
		result, err = extractGo(source)
	case "c", "cpp":
		result = extractCFamily(source, header)
	default:
		result = extractLines(language, source)
	}
	result.Language = language
	result.Chunks = Chunks(string(source), result.Symbols)
	return result, err
}

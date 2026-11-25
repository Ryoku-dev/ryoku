package profile

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

var idPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

func validID(id string) bool { return len(id) <= MaxIDLen && idPattern.MatchString(id) }

// unitFraction is a finite number from 0 to 1.
func unitFraction(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v <= 1 }

// unsafeRune is the first rune in s that could hide or reorder text: control,
// format (bidi overrides, zero-width), line/paragraph separators, or invalid
// UTF-8. Newline and tab pass when multiline.
func unsafeRune(s string, multiline bool) (string, bool) {
	for i, r := range s {
		if r == utf8.RuneError {
			if _, size := utf8.DecodeRuneInString(s[i:]); size <= 1 {
				return "invalid UTF-8", true
			}
		}
		if multiline && (r == '\n' || r == '\t') {
			continue
		}
		if unicode.Is(unicode.Cc, r) || unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r) {
			return fmt.Sprintf("U+%04X", r), true
		}
	}
	return "", false
}

const maxJSONDepth = 8

// scanJSON checks a document's structure before it is decoded: one object,
// no duplicate keys, no authority keys at any depth and only the schema's
// keys in its fixed objects. It returns the top-level scalar values, or nil
// when the document is not JSON at all.
func scanJSON(file string, data []byte, schema map[string][]string, r *Report) map[string]any {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	top := map[string]any{}
	errorsBefore := len(r.Errors)
	var walk func(path string, depth int) (any, error)
	walk = func(path string, depth int) (any, error) {
		if depth > maxJSONDepth {
			return nil, fmt.Errorf("nested deeper than %d", maxJSONDepth)
		}
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		d, ok := tok.(json.Delim)
		if depth == 0 && d != '{' {
			return nil, errors.New("not a JSON object")
		}
		if !ok {
			return tok, nil
		}
		if d == '[' {
			for dec.More() {
				if _, err := walk(path+"[]", depth+1); err != nil {
					return nil, err
				}
			}
			_, err := dec.Token()
			return nil, err
		}
		fixed, isFixed := schema[path]
		seen := map[string]bool{}
		for dec.More() {
			kt, err := dec.Token()
			if err != nil {
				return nil, err
			}
			k, _ := kt.(string)
			lower := strings.ToLower(k)
			switch {
			case seen[lower]:
				r.reject(ReasonDuplicateKey, "%s: %q", file, join(path, k))
			case slices.Contains(authorityKeys, lower):
				r.reject(ReasonAuthorityKey, "%s: %q is not allowed, a character is presentation only and grants nothing", file, join(path, k))
			case isFixed && !slices.Contains(fixed, k):
				r.reject(ReasonUnknownKey, "%s: %q", file, join(path, k))
			}
			seen[lower] = true
			v, err := walk(join(path, k), depth+1)
			if err != nil {
				return nil, err
			}
			if depth == 0 {
				top[k] = v
			}
		}
		_, err = dec.Token()
		return nil, err
	}
	if _, err := walk("", 0); err != nil {
		r.Errors = r.Errors[:errorsBefore]
		r.reject(ReasonBadJSON, "%s: %v", file, err)
		return nil
	}
	if _, err := dec.Token(); err != io.EOF {
		r.Errors = r.Errors[:errorsBefore]
		r.reject(ReasonBadJSON, "%s: data after the object", file)
		return nil
	}
	return top
}

func join(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

// decodeStrict decodes a document scanJSON passed; what is left to fail is a
// value of the wrong type.
func decodeStrict(file string, data []byte, v any, r *Report) bool {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		var te *json.UnmarshalTypeError
		if errors.As(err, &te) && te.Field != "" {
			r.reject(ReasonBadValue, "%s: %s has the wrong type", file, te.Field)
		} else {
			r.reject(ReasonBadValue, "%s: %v", file, err)
		}
		return false
	}
	return true
}

// svgElements is everything a character's SVG may contain: shapes, groups,
// paint servers and in-document references. Anything else (script,
// foreignObject, image, style, animation, links) can run or fetch.
var svgElements = []string{
	"svg", "g", "path", "circle", "ellipse", "rect", "line", "polyline", "polygon",
	"defs", "linearGradient", "radialGradient", "stop", "title", "desc",
	"clipPath", "mask", "symbol", "use",
}

const svgNamespace = "http://www.w3.org/2000/svg"

const maxSVGTokens = 20000

// checkSVG rejects SVG that is malformed or can do more than draw.
func checkSVG(name string, data []byte, r *Report) {
	if !utf8.Valid(data) {
		r.reject(ReasonBadSVG, "%s: not UTF-8", name)
		return
	}
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = true
	root := true
	for n := 0; ; n++ {
		if n > maxSVGTokens {
			r.reject(ReasonTooLarge, "%s: more than %d SVG tokens", name, maxSVGTokens)
			return
		}
		tok, err := dec.Token()
		if err == io.EOF {
			if root {
				r.reject(ReasonBadSVG, "%s: no svg element", name)
			}
			return
		}
		if err != nil {
			r.reject(ReasonBadSVG, "%s: %v", name, err)
			return
		}
		switch t := tok.(type) {
		case xml.ProcInst:
			if t.Target != "xml" {
				r.reject(ReasonActiveSVG, "%s: processing instruction %q", name, t.Target)
				return
			}
		case xml.Directive:
			r.reject(ReasonActiveSVG, "%s: document type or entity declaration", name)
			return
		case xml.StartElement:
			if root && t.Name.Local != "svg" {
				r.reject(ReasonBadSVG, "%s: root element is %q, not svg", name, t.Name.Local)
				return
			}
			root = false
			if (t.Name.Space != "" && t.Name.Space != svgNamespace) || !slices.Contains(svgElements, t.Name.Local) {
				r.reject(ReasonActiveSVG, "%s: element %q", name, t.Name.Local)
				return
			}
			for _, a := range t.Attr {
				if why := activeAttr(a); why != "" {
					r.reject(ReasonActiveSVG, "%s: %s on %q", name, why, t.Name.Local)
					return
				}
			}
		}
	}
}

// activeAttr says why an attribute could run code or reach outside the file.
func activeAttr(a xml.Attr) string {
	local := strings.ToLower(a.Name.Local)
	value := strings.ToLower(strings.TrimSpace(a.Value))
	if a.Name.Space == "xmlns" || (a.Name.Space == "" && local == "xmlns") {
		return ""
	}
	if strings.HasPrefix(local, "on") {
		return fmt.Sprintf("event handler %q", a.Name.Local)
	}
	if local == "href" && !strings.HasPrefix(value, "#") {
		return "external reference"
	}
	if strings.Contains(value, "javascript:") || strings.Contains(value, "@import") {
		return "script or import in " + a.Name.Local
	}
	for rest := value; ; {
		i := strings.Index(rest, "url(")
		if i < 0 {
			break
		}
		rest = strings.TrimLeft(rest[i+4:], " \t\n'\"")
		if !strings.HasPrefix(rest, "#") {
			return "external url() in " + a.Name.Local
		}
	}
	return ""
}

// assetPath checks a path written in a profile: relative, inside the
// character's folder, plainly spelled.
func assetPath(field, p string, r *Report) bool {
	switch {
	case strings.HasPrefix(p, "/") || strings.HasPrefix(p, "~") || (len(p) > 1 && p[1] == ':'):
		r.reject(ReasonAbsolutePath, "%s: %q", field, p)
	case slices.Contains(strings.Split(p, "/"), ".."):
		r.reject(ReasonEscapingPath, "%s: %q leaves the character's folder", field, p)
	case strings.ContainsRune(p, '\\') || !fs.ValidPath(p) || p == ".":
		r.reject(ReasonBadPath, "%s: %q", field, p)
	case len(p) > MaxPathLen || strings.Count(p, "/") >= MaxPathDepth:
		r.reject(ReasonBadPath, "%s: %q is too long or too deep", field, p)
	default:
		if why, bad := unsafeRune(p, false); bad {
			r.reject(ReasonUnsafeText, "%s: path contains %s", field, why)
			return false
		}
		return true
	}
	return false
}

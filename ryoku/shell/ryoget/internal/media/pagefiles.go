package media

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"

	"ryoget/internal/plan"

	i18n "ryoku-i18n"
)

// a page linking more files than this is a directory index, the first ones are offered
const pageFileLimit = 200

var (
	anchorRe = regexp.MustCompile(`(?is)<a\s([^>]*)>(.*?)</a>`)
	tagsRe   = regexp.MustCompile(`(?s)<[^>]*>`)
	quotedRe = regexp.MustCompile(`"[^"]*"|'[^']*'`)
	// a download attribute, bare or with a file name, once quoted values are cut out
	downloadAttrRe = regexp.MustCompile(`(?i)(?:^|\s)download(?:\s*=|\s|/|$)`)
	formRe         = regexp.MustCompile(`(?is)<form\b([^>]*)>(.*?)</form>`)
	fieldRe        = regexp.MustCompile(`(?is)<(button)\b([^>]*)>(.*?)</button>|<(input|select|textarea)\b([^>]*)>`)
	captchaRe      = regexp.MustCompile(`(?i)g-recaptcha|h-captcha|cf-turnstile|frc-captcha|data-sitekey`)
)

func tagAttrs(tag []byte) map[string]string {
	attrs := map[string]string{}
	for _, m := range attrRe.FindAllSubmatch(tag, -1) {
		v := string(m[3])
		if v == "" {
			v = string(m[4])
		}
		if v == "" {
			v = string(m[5])
		}
		k := strings.ToLower(string(m[1]))
		if _, seen := attrs[k]; !seen {
			attrs[k] = html.UnescapeString(v)
		}
	}
	return attrs
}

func innerText(b []byte) string {
	return strings.Join(strings.Fields(html.UnescapeString(string(tagsRe.ReplaceAll(b, []byte(" "))))), " ")
}

// pageFiles lists anchors whose path ends in a file extension or that carry a download attribute.
func pageFiles(base *url.URL, page []byte) []Item {
	self := *base
	self.Fragment = ""
	seen := map[string]bool{self.String(): true}
	var out []Item
	for _, m := range anchorRe.FindAllSubmatch(page, -1) {
		attrs := tagAttrs(m[1])
		ref := strings.TrimSpace(attrs["href"])
		if ref == "" || strings.HasPrefix(ref, "#") {
			continue
		}
		fu, err := base.Parse(ref)
		if err != nil || (fu.Scheme != "http" && fu.Scheme != "https") || fu.Host == "" {
			continue
		}
		fu.Fragment, fu.RawFragment = "", ""
		_, isFile := fileExt(fu.Path)
		if !isFile && !downloadAttrRe.Match(quotedRe.ReplaceAll(m[1], nil)) {
			continue
		}
		key := fu.String()
		if seen[key] {
			continue
		}
		seen[key] = true
		name := path.Base(fu.Path)
		if s, err := url.PathUnescape(name); err == nil {
			name = s
		}
		title := ""
		if isFile {
			title = name
		} else {
			title = firstOf(attrBase(attrs["download"]), innerText(m[2]), name)
		}
		out = append(out, Item{Title: strings.TrimSpace(title), Kind: "file", URL: key})
		if len(out) == pageFileLimit {
			break
		}
	}
	return out
}

func attrBase(s string) string {
	if s = strings.TrimSpace(s); s == "" {
		return ""
	}
	return path.Base(s)
}

// field is one control of a form: kind is input, button, select or textarea.
type field struct {
	kind  string
	attrs []byte
	text  []byte
}

func formFields(body []byte) []field {
	var out []field
	for _, m := range fieldRe.FindAllSubmatch(body, -1) {
		if m[1] != nil {
			out = append(out, field{kind: "button", attrs: m[2], text: m[3]})
			continue
		}
		out = append(out, field{kind: strings.ToLower(string(m[4])), attrs: m[5]})
	}
	return out
}

type pageForm struct {
	method string
	action *url.URL
	values url.Values
	// script fills or sends the form, or it asks a person something
	browser bool
}

// downloadForm finds a form named for downloading or with a download button, and whether only a browser can send it.
func downloadForm(base *url.URL, page []byte) (*pageForm, bool) {
	for _, m := range formRe.FindAllSubmatch(page, -1) {
		fa := tagAttrs(m[1])
		body := m[2]
		named := strings.Contains(strings.ToLower(fa["id"]+" "+fa["name"]+" "+fa["class"]+" "+fa["action"]), "download")
		fields := formFields(body)
		if !named {
			for _, f := range fields {
				a := tagAttrs(f.attrs)
				submit := f.kind == "button" || (f.kind == "input" && (strings.EqualFold(a["type"], "submit") || strings.EqualFold(a["type"], "image")))
				if submit && strings.Contains(strings.ToLower(a["value"]+" "+innerText(f.text)), "download") {
					named = true
					break
				}
			}
		}
		if !named {
			continue
		}
		f := &pageForm{method: strings.ToUpper(strings.TrimSpace(fa["method"])), values: url.Values{}}
		if f.method == "" {
			f.method = http.MethodGet
		}
		act, err := base.Parse(strings.TrimSpace(fa["action"]))
		if err != nil || (act.Scheme != "http" && act.Scheme != "https") {
			f.browser = true
			return f, true
		}
		act.Fragment, act.RawFragment = "", ""
		f.action = act
		if captchaRe.Match(body) || captchaRe.Match(m[1]) || (f.method != http.MethodGet && f.method != http.MethodPost) {
			f.browser = true
			return f, true
		}
		submitted := false
		for _, fd := range fields {
			a := tagAttrs(fd.attrs)
			name := a["name"]
			value, hasValue := a["value"]
			switch fd.kind {
			case "select", "textarea":
				f.browser = true
			case "button":
				if name != "" && !submitted && !strings.EqualFold(a["type"], "button") && !strings.EqualFold(a["type"], "reset") {
					f.values.Add(name, value)
					submitted = true
				}
			default:
				switch strings.ToLower(a["type"]) {
				case "hidden":
					if !hasValue {
						f.browser = true
					} else if name != "" {
						f.values.Add(name, value)
					}
				case "submit", "image":
					if name != "" && !submitted {
						f.values.Add(name, value)
						submitted = true
					}
				case "button", "reset":
				case "checkbox", "radio":
					if name != "" && bytes.Contains(bytes.ToLower(quotedRe.ReplaceAll(fd.attrs, nil)), []byte("checked")) {
						f.values.Add(name, firstOf(value, "on"))
					}
				default:
					if value == "" {
						f.browser = true
					} else if name != "" {
						f.values.Add(name, value)
					}
				}
			}
		}
		return f, true
	}
	return nil, false
}

func needsBrowserErr() error {
	return BrowserCheck(i18n.T("This page needs a browser to hand out the file."))
}

// followForm sends a plain download form as a browser would and resolves the file it hands out.
func followForm(ctx context.Context, page *url.URL, env *plan.Env, f *pageForm, ua string) (*Media, error) {
	if f.method == http.MethodGet {
		target := *f.action
		target.RawQuery = f.values.Encode()
		return probeFile(ctx, &target, env, page.String())
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.action.String(), strings.NewReader(f.values.Encode()))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Referer", page.String())
	req.Header.Set("Accept", "*/*")
	resp, err := httpClient(env).Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%w: %v", ErrNetwork, err)
	}
	defer resp.Body.Close()
	mt, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	isPage := mt == "text/html" || mt == "application/xhtml+xml"
	if isPage || browserCheckHeader(resp.Header) {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, challengeLimit))
		if ChallengePage(resp.Header, body) {
			return nil, BrowserCheckError()
		}
	}
	if resp.StatusCode >= 400 {
		return nil, statusError(resp)
	}
	if isPage {
		return nil, needsBrowserErr()
	}
	// a redirect turned the post into a plain GET for the file, which the engine can repeat
	if resp.Request.Method == http.MethodGet && resp.Request.URL.String() != f.action.String() {
		return probeFile(ctx, resp.Request.URL, env, page.String())
	}
	return nil, fmt.Errorf("%w: %s", ErrUnsupported, i18n.T("This page sends the file only in answer to a form, which cannot be downloaded yet."))
}

func statusError(resp *http.Response) error {
	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		return fmt.Errorf("%w: %s", ErrUnavailable, resp.Status)
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return fmt.Errorf("%w: %s", ErrPrivate, resp.Status)
	case resp.StatusCode == http.StatusTooManyRequests:
		return fmt.Errorf("%w: %s", ErrRateLimited, resp.Status)
	}
	return fmt.Errorf("%w: %s", ErrUnavailable, resp.Status)
}

// probeFile resolves a file a page handed out, never following a page that answers instead.
func probeFile(ctx context.Context, u *url.URL, env *plan.Env, referer string) (*Media, error) {
	m, err := probeWith(ctx, u, env, false, referer, false)
	if m != nil && m.Kind == plan.KindPage {
		return nil, fmt.Errorf("%w: %s", ErrUnavailable, i18n.T("The link answered with a web page instead of the file."))
	}
	if err != nil {
		return nil, err
	}
	if referer != "" {
		if m.Headers == nil {
			m.Headers = map[string]string{}
		}
		m.Headers["Referer"] = referer
	}
	return m, nil
}

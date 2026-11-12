package gateway

import (
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

// Listing shapes: how a provider's own model listing reads.
const (
	shapeOpenAI     = "openai"
	shapeAnthropic  = "anthropic"
	shapeCloudflare = "cloudflare"
	shapeCohere     = "cohere"
	shapeGitHub     = "github"
)

func listingShape(shape string) bool {
	switch shape {
	case "", shapeOpenAI, shapeAnthropic, shapeCloudflare, shapeCohere, shapeGitHub:
		return true
	}
	return false
}

// listedModel is one chat model as a provider's own listing describes it.
type listedModel struct {
	ID         string `json:"id"`
	Name       string `json:"name,omitempty"`
	Context    int    `json:"context,omitempty"`
	Tools      bool   `json:"tools,omitempty"`
	ToolsKnown bool   `json:"toolsKnown,omitempty"`
	Vision     bool   `json:"vision,omitempty"`
	Reasoning  bool   `json:"reasoning,omitempty"`
	// Free is a model the listing marks free: a :free id, a free flag, or a
	// prompt and completion price of zero.
	Free bool `json:"free,omitempty"`
	// Priced is a model the listing gives a price other than zero (a
	// negative one is a router's variable price), or marks paid only.
	Priced bool `json:"priced,omitempty"`
}

// parseListing reads one page of a listing in the given shape. next is the
// page after it, or nil.
func parseListing(shape string, body []byte, page *url.URL) (models []listedModel, next *url.URL, err error) {
	switch shape {
	case "", shapeOpenAI:
		models, err = parseOpenAIListing(body)
	case shapeAnthropic:
		models, next, err = parseAnthropicListing(body, page)
	case shapeCloudflare:
		models, next, err = parseCloudflareListing(body, page)
	case shapeCohere:
		models, next, err = parseCohereListing(body, page)
	case shapeGitHub:
		models, err = parseGitHubListing(body)
	default:
		err = fmt.Errorf("unknown listing shape %q", shape)
	}
	return models, next, err
}

// wireEntry is one model in an OpenAI-shaped listing, read field by field:
// providers add fields of their own and disagree on their types (a context
// as a number or an object, pricing as an object or a list), and one odd
// entry must not cost the rest of the listing.
type wireEntry map[string]json.RawMessage

func parseOpenAIListing(body []byte) ([]listedModel, error) {
	var entries []wireEntry
	// OpenTyphoon answers with the bare list
	if trimmed := strings.TrimSpace(string(body)); !strings.HasPrefix(trimmed, "[") {
		var listing struct {
			Data []json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(body, &listing); err != nil {
			return nil, fmt.Errorf("read model listing: %w", err)
		}
		for _, raw := range listing.Data {
			var entry wireEntry
			if json.Unmarshal(raw, &entry) == nil {
				entries = append(entries, entry)
			}
		}
	} else if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("read model listing: %w", err)
	}
	models := make([]listedModel, 0, len(entries))
	for _, entry := range entries {
		if m, ok := entry.listed(); ok {
			models = appendListed(models, m)
		}
	}
	return models, nil
}

// listed reads every field Rashin knows: OpenRouter's and Kilo's pricing,
// supported_parameters and architecture, Mistral's capabilities, Groq's
// active and context_window, Hugging Face's router providers, OpenCode
// Zen's cost, LLM7's and Api.Airforce's tier, Requesty's prices and
// supports_* flags, and the kind or category others give.
func (w wireEntry) listed() (listedModel, bool) {
	// Google's OpenAI-compatible listing names models as its own API does
	id := strings.TrimPrefix(w.text("id"), "models/")
	if id == "" || !chatID(id) || !w.chat() {
		return listedModel{}, false
	}
	architecture := w.object("architecture")
	modalities := w.object("modalities")
	input := firstNonEmpty(architecture.strings("input_modalities"), w.strings("input_modalities"), modalities.strings("input"))
	output := firstNonEmpty(architecture.strings("output_modalities"), w.strings("output_modalities"), modalities.strings("output"))
	capabilities := w.object("capabilities")
	features := w.strings("supported_features")
	if !takesText(input) || !takesText(output) || slices.Contains(features, "embeddings") {
		return listedModel{}, false
	}
	if chat, known := capabilities.flag("completion_chat"); known && !chat {
		return listedModel{}, false
	}
	m := listedModel{ID: id, Name: firstText(w.text("name"), w.text("title"), w.text("display_name"))}
	m.Context = w.context()
	parameters := w.strings("supported_parameters")
	if len(parameters) > 0 {
		m.ToolsKnown, m.Tools = true, slices.Contains(parameters, "tools")
		m.Reasoning = slices.Contains(parameters, "reasoning")
	}
	if slices.Contains(features, "tools") {
		m.ToolsKnown, m.Tools = true, true
	}
	for _, source := range []wireEntry{w, capabilities} {
		if tools, known := source.flag("supports_tool_calling", "tools", "function_calling", "function_call", "tool_calling", "tool_calls"); known {
			m.ToolsKnown, m.Tools = true, tools
		}
		if reasoning, known := source.flag("supports_reasoning", "reasoning"); known {
			m.Reasoning = reasoning
		}
	}
	m.Vision = hasModality(input, "image")
	for _, source := range []wireEntry{w, capabilities} {
		if vision, known := source.flag("supports_vision", "supports_images", "vision"); known {
			m.Vision = vision
		}
	}
	m.cost(w)
	if providers := w.list("providers"); len(providers) > 0 {
		m.routedBy(providers)
	}
	return m, true
}

// chat reads what an entry says it is, when it says: listings that serve
// pictures, speech and embeddings beside chat mark them in one of these.
func (w wireEntry) chat() bool {
	if active, known := w.flag("active", "available", "supports_chat"); known && !active {
		return false
	}
	if api := w.text("api"); api != "" && api != "chat" {
		return false
	}
	for _, field := range []string{"type", "model_type", "kind", "category", "media_type"} {
		if slices.Contains(nonChatKinds, strings.ToLower(w.text(field))) {
			return false
		}
	}
	return true
}

var nonChatKinds = []string{
	"embedding", "embeddings", "image", "images", "image_generation", "audio", "audio_to_text",
	"speech", "tts", "stt", "transcription", "video", "video_generation", "rerank", "moderation",
	"realtime", "3d", "decision",
}

func (w wireEntry) context() int {
	if tokens := firstNonZero(w.integer("context_length"), w.object("top_provider").integer("context_length")); tokens != 0 {
		return tokens
	}
	if tokens := firstNonZero(w.integer("max_context_length"), w.integer("max_model_len")); tokens != 0 {
		return tokens
	}
	// Groq gives a number, LLM7 {"tokens": n}
	return firstNonZero(w.integer("context_window"), w.object("context_window").integer("tokens"))
}

// cost reads whether the entry costs anything. A free mark is an id ending
// :free or -free, a free flag, tier or tag, or a price of zero; a price of
// anything else (a router's variable price is negative), a tier other than
// free or paid_only marks it priced.
func (m *listedModel) cost(w wireEntry) {
	m.Free = strings.HasSuffix(m.ID, ":free") || strings.HasSuffix(m.ID, "-free") ||
		strings.EqualFold(w.text("tier"), "free") || slices.Contains(w.strings("tags"), "free")
	if free, known := w.flag("isFree", "is_free"); known && free {
		m.Free = true
	}
	pricing := w.object("pricing")
	for _, pair := range []struct {
		from    wireEntry
		in, out string
	}{
		{pricing, "prompt", "completion"},
		{pricing, "input", "output"},
		{w, "input_price", "output_price"},
		{w.object("price"), "input", "output"},
		{w.object("cost").object("text"), "input", "output"},
	} {
		in, inKnown := pair.from.amount(pair.in)
		out, outKnown := pair.from.amount(pair.out)
		if inKnown && outKnown {
			if in == 0 && out == 0 {
				m.Free = true
			} else {
				m.Priced = true
			}
			break
		}
	}
	if tier := strings.ToLower(w.text("tier")); tier != "" && tier != "free" && !m.Free {
		m.Priced = true
	}
	if paid, _ := w.flag("paid_only"); paid {
		m.Free, m.Priced = false, true
	}
	if m.Free {
		m.Priced = false
	}
}

// routedBy reads a model Hugging Face's router serves through several
// providers: free when one serves it free, priced when every live one charges.
func (m *listedModel) routedBy(providers []wireEntry) {
	charged := true
	for _, p := range providers {
		if status := p.text("status"); status != "" && status != "live" {
			continue
		}
		m.Context = max(m.Context, p.integer("context_length"))
		if tools, known := p.flag("supports_tools"); known {
			m.ToolsKnown, m.Tools = true, m.Tools || tools
		}
		var each listedModel
		each.cost(p)
		m.Free = m.Free || each.Free
		charged = charged && each.Priced
	}
	m.Priced = charged && !m.Free
}

// field is the raw value at key, nil when absent or null: a null price is
// not a price of zero.
func (w wireEntry) field(key string) json.RawMessage {
	raw := w[key]
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return raw
}

func (w wireEntry) text(key string) string {
	var value string
	if json.Unmarshal(w.field(key), &value) != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func (w wireEntry) strings(key string) []string {
	var values []string
	if json.Unmarshal(w.field(key), &values) == nil {
		return values
	}
	// Hugging Face and others give tags as objects with a name
	var named []struct{ Name string }
	if json.Unmarshal(w.field(key), &named) == nil {
		for _, value := range named {
			values = append(values, value.Name)
		}
	}
	return values
}

// object reads a nested object; a list stands for its first entry, as
// Requesty gives tiered pricing.
func (w wireEntry) object(key string) wireEntry {
	var object wireEntry
	if json.Unmarshal(w.field(key), &object) == nil {
		return object
	}
	if list := w.list(key); len(list) > 0 {
		return list[0]
	}
	return nil
}

func (w wireEntry) list(key string) []wireEntry {
	var list []wireEntry
	if json.Unmarshal(w.field(key), &list) != nil {
		return nil
	}
	return list
}

// flag reads the first of keys present as a bool, "true" or "false", or an
// object with a supported bool, as OpenCode Zen says what a model can do.
func (w wireEntry) flag(keys ...string) (value, known bool) {
	for _, key := range keys {
		raw := w.field(key)
		if raw == nil {
			continue
		}
		var flag bool
		if json.Unmarshal(raw, &flag) == nil {
			return flag, true
		}
		switch strings.ToLower(w.text(key)) {
		case "true":
			return true, true
		case "false":
			return false, true
		}
		var nested struct{ Supported *bool }
		if json.Unmarshal(raw, &nested) == nil && nested.Supported != nil {
			return *nested.Supported, true
		}
	}
	return false, false
}

func (w wireEntry) number(key string) (float64, bool) {
	var number float64
	if json.Unmarshal(w.field(key), &number) == nil {
		return number, true
	}
	number, err := strconv.ParseFloat(w.text(key), 64)
	return number, err == nil
}

func (w wireEntry) integer(key string) int {
	number, _ := w.number(key)
	return int(number)
}

// amount reads a price: a number, a numeric string, or an object holding
// one (OpenCode Zen's price, Routeway's price_per_million_t, Chutes' usd).
func (w wireEntry) amount(key string) (float64, bool) {
	if number, ok := w.number(key); ok {
		return number, true
	}
	nested := w.object(key)
	for _, inner := range []string{"price", "price_per_million_t", "usd"} {
		if number, ok := nested.number(inner); ok {
			return number, true
		}
	}
	return 0, false
}

// takesText reads a modality list: absent says nothing, present it must
// carry text.
func takesText(modalities []string) bool {
	return len(modalities) == 0 || hasModality(modalities, "text")
}

func hasModality(modalities []string, kind string) bool {
	return slices.ContainsFunc(modalities, func(m string) bool {
		m = strings.ToLower(m)
		return m == kind || strings.HasPrefix(m, kind+"/")
	})
}

// nonChat are words in the id of a model that answers no chat, for listings
// that list everything a provider serves and say nothing of what each is.
var nonChat = []string{
	"embed", "rerank", "whisper", "tts", "transcribe", "speech", "audio", "realtime",
	"moderation", "guard", "bge-", "dall-e", "image", "imagen", "veo", "sora", "lyria",
	"stable-diffusion", "sdxl", "flux", "davinci", "babbage",
}

func chatID(id string) bool {
	lower := strings.ToLower(id)
	return !slices.ContainsFunc(nonChat, func(word string) bool { return strings.Contains(lower, word) })
}

func firstNonEmpty(lists ...[]string) []string {
	for _, list := range lists {
		if len(list) > 0 {
			return list
		}
	}
	return nil
}

// appendListed adds m unless the listing already named it.
func appendListed(models []listedModel, m listedModel) []listedModel {
	if slices.ContainsFunc(models, func(seen listedModel) bool { return strings.EqualFold(seen.ID, m.ID) }) {
		return models
	}
	return append(models, m)
}

// parseAnthropicListing reads GET /v1/models. Every Claude model takes tools
// and pictures.
func parseAnthropicListing(body []byte, page *url.URL) ([]listedModel, *url.URL, error) {
	var listing struct {
		Data []struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
		} `json:"data"`
		HasMore bool   `json:"has_more"`
		LastID  string `json:"last_id"`
	}
	if err := json.Unmarshal(body, &listing); err != nil {
		return nil, nil, fmt.Errorf("read model listing: %w", err)
	}
	var models []listedModel
	for _, entry := range listing.Data {
		if entry.ID != "" {
			models = appendListed(models, listedModel{ID: entry.ID, Name: entry.DisplayName, Tools: true, ToolsKnown: true, Vision: true})
		}
	}
	if !listing.HasMore || listing.LastID == "" {
		return models, nil, nil
	}
	return models, withQuery(page, "after_id", listing.LastID), nil
}

// parseCloudflareListing reads accounts/{id}/ai/models/search, where a
// model's id is its name and what it can do is a list of properties.
func parseCloudflareListing(body []byte, page *url.URL) ([]listedModel, *url.URL, error) {
	var listing struct {
		Result []struct {
			Name       string                `json:"name"`
			Task       struct{ Name string } `json:"task"`
			Properties []struct {
				ID    string `json:"property_id"`
				Value any    `json:"value"`
			} `json:"properties"`
		} `json:"result"`
		Info struct {
			Page    int `json:"page"`
			PerPage int `json:"per_page"`
			Total   int `json:"total_count"`
		} `json:"result_info"`
	}
	if err := json.Unmarshal(body, &listing); err != nil {
		return nil, nil, fmt.Errorf("read model listing: %w", err)
	}
	var models []listedModel
	for _, entry := range listing.Result {
		if entry.Name == "" || (entry.Task.Name != "" && entry.Task.Name != "Text Generation") {
			continue
		}
		m := listedModel{ID: entry.Name}
		for _, property := range entry.Properties {
			value := fmt.Sprint(property.Value)
			switch property.ID {
			case "context_window", "max_input_tokens":
				m.Context, _ = strconv.Atoi(value)
			case "function_calling":
				m.ToolsKnown, m.Tools = true, value == "true"
			case "reasoning":
				m.Reasoning = value == "true"
			case "vision":
				m.Vision = value == "true"
			}
		}
		models = appendListed(models, m)
	}
	if listing.Info.PerPage == 0 || listing.Info.Page*listing.Info.PerPage >= listing.Info.Total {
		return models, nil, nil
	}
	return models, withQuery(page, "page", strconv.Itoa(listing.Info.Page+1)), nil
}

// parseCohereListing reads Cohere's own GET /v1/models?endpoint=chat.
func parseCohereListing(body []byte, page *url.URL) ([]listedModel, *url.URL, error) {
	var listing struct {
		Models []struct {
			Name          string   `json:"name"`
			Endpoints     []string `json:"endpoints"`
			ContextLength float64  `json:"context_length"`
			Features      []string `json:"features"`
		} `json:"models"`
		NextPageToken string `json:"next_page_token"`
	}
	if err := json.Unmarshal(body, &listing); err != nil {
		return nil, nil, fmt.Errorf("read model listing: %w", err)
	}
	var models []listedModel
	for _, entry := range listing.Models {
		if entry.Name == "" || !slices.Contains(entry.Endpoints, "chat") {
			continue
		}
		m := listedModel{ID: entry.Name, Context: int(entry.ContextLength), ToolsKnown: true}
		m.Tools = slices.ContainsFunc(entry.Features, func(f string) bool { return strings.Contains(f, "tool") })
		m.Vision = slices.Contains(entry.Features, "vision")
		models = appendListed(models, m)
	}
	if listing.NextPageToken == "" {
		return models, nil, nil
	}
	return models, withQuery(page, "page_token", listing.NextPageToken), nil
}

// parseGitHubListing reads the GitHub Models catalogue, a bare array.
func parseGitHubListing(body []byte) ([]listedModel, error) {
	var listing []struct {
		ID           string   `json:"id"`
		Name         string   `json:"name"`
		Input        []string `json:"supported_input_modalities"`
		Output       []string `json:"supported_output_modalities"`
		Capabilities []string `json:"capabilities"`
		Limits       struct {
			Input int `json:"max_input_tokens"`
		} `json:"limits"`
	}
	if err := json.Unmarshal(body, &listing); err != nil {
		return nil, fmt.Errorf("read model listing: %w", err)
	}
	var models []listedModel
	for _, entry := range listing {
		if entry.ID == "" || !takesText(entry.Input) || len(entry.Output) == 0 || !takesText(entry.Output) || !chatID(entry.ID) {
			continue
		}
		models = appendListed(models, listedModel{
			ID: entry.ID, Name: entry.Name, Context: entry.Limits.Input,
			Tools: slices.Contains(entry.Capabilities, "tool-calling"), ToolsKnown: true,
			Vision: hasModality(entry.Input, "image"), Reasoning: slices.Contains(entry.Capabilities, "reasoning"),
		})
	}
	return models, nil
}

func withQuery(page *url.URL, key, value string) *url.URL {
	next := *page
	query := next.Query()
	query.Set(key, value)
	next.RawQuery = query.Encode()
	return &next
}

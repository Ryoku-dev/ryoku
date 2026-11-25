package gateway

import (
	"net/url"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func parsed(t *testing.T, shape, body string) map[string]listedModel {
	t.Helper()
	page, _ := url.Parse("https://provider.test/models")
	models, _, err := parseListing(shape, []byte(body), page)
	if err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]listedModel, len(models))
	for _, m := range models {
		byID[m.ID] = m
	}
	return byID
}

// OpenRouter and Kilo price every model: a :free id or a zero price is free,
// anything else (a router's -1 too) is priced, and supported_parameters says
// whether it takes tools.
func TestOpenRouterListingSaysWhatIsFree(t *testing.T) {
	models := parsed(t, shapeOpenAI, `{"data":[
		{"id":"nvidia/nemotron:free","name":"Nemotron (free)","context_length":1000000,"architecture":{"input_modalities":["text","image"],"output_modalities":["text"]},"pricing":{"prompt":"0","completion":"0"},"supported_parameters":["tools","reasoning"]},
		{"id":"openai/gpt-big","pricing":{"prompt":"0.000002","completion":"0.00001"},"supported_parameters":["tools"]},
		{"id":"zero/priced-at-nothing","pricing":{"prompt":"0","completion":"0"},"supported_parameters":["temperature"]},
		{"id":"openrouter/auto","pricing":{"prompt":"-1","completion":"-1"},"supported_parameters":["tools"]},
		{"id":"kilo/flagged","isFree":true,"pricing":{"prompt":"0.1","completion":"0.1"}},
		{"id":"google/lyria","pricing":{"prompt":"0","completion":"0"},"architecture":{"input_modalities":["text"],"output_modalities":["audio"]}},
		{"id":"openai/text-embedding-3","pricing":{"prompt":"0","completion":"0"}}
	]}`)
	free := models["nvidia/nemotron:free"]
	if !free.Free || free.Priced || !free.ToolsKnown || !free.Tools || !free.Reasoning || !free.Vision || free.Context != 1000000 || free.Name != "Nemotron (free)" {
		t.Fatalf(":free model = %#v", free)
	}
	if m := models["openai/gpt-big"]; m.Free || !m.Priced {
		t.Fatalf("priced model = %#v", m)
	}
	if m := models["zero/priced-at-nothing"]; !m.Free || m.Priced || !m.ToolsKnown || m.Tools {
		t.Fatalf("zero-priced model without tools = %#v", m)
	}
	if m := models["openrouter/auto"]; m.Free || !m.Priced {
		t.Fatalf("variable price router = %#v", m)
	}
	if m := models["kilo/flagged"]; !m.Free || m.Priced {
		t.Fatalf("isFree model = %#v", m)
	}
	for _, gone := range []string{"google/lyria", "openai/text-embedding-3"} {
		if _, ok := models[gone]; ok {
			t.Fatalf("%s answers no chat but was listed", gone)
		}
	}
}

// The fields other providers add, each read where it is.
func TestListingsOfOtherShapes(t *testing.T) {
	t.Run("mistral", func(t *testing.T) {
		models := parsed(t, "", `{"data":[
			{"id":"mistral-medium","max_context_length":256000,"capabilities":{"completion_chat":true,"function_calling":true,"vision":true}},
			{"id":"mistral-embed","capabilities":{"completion_chat":false,"function_calling":false}},
			{"id":"mistral-medium","capabilities":{"completion_chat":true}}
		]}`)
		if m := models["mistral-medium"]; len(models) != 1 || m.Context != 256000 || !m.ToolsKnown || !m.Tools || !m.Vision || m.Free || m.Priced {
			t.Fatalf("mistral = %#v", models)
		}
	})
	t.Run("groq and google", func(t *testing.T) {
		models := parsed(t, "", `{"data":[
			{"id":"llama-3.3-70b-versatile","context_window":131072,"active":true},
			{"id":"retired-model","active":false},
			{"id":"models/gemini-3.8-flash","object":"model"},
			{"id":"whisper-large-v3"}
		]}`)
		if len(models) != 2 || models["llama-3.3-70b-versatile"].Context != 131072 || models["llama-3.3-70b-versatile"].ToolsKnown {
			t.Fatalf("groq = %#v", models)
		}
		if _, ok := models["gemini-3.8-flash"]; !ok {
			t.Fatalf("google's models/ prefix kept: %#v", models)
		}
	})
	t.Run("hugging face router", func(t *testing.T) {
		models := parsed(t, "", `{"data":[
			{"id":"Qwen/Qwen3","providers":[{"provider":"a","status":"live","context_length":262144,"pricing":{"input":0.4,"output":3},"supports_tools":true},{"provider":"b","status":"live","pricing":{"input":0.9,"output":1.4},"supports_tools":false}]},
			{"id":"meta/llama","providers":[{"provider":"c","status":"live","pricing":{"input":0.1,"output":0.1},"is_free":true}]},
			{"id":"null/prices","providers":[{"provider":"d","status":"live","pricing":null}]}
		]}`)
		if m := models["Qwen/Qwen3"]; !m.Priced || m.Free || !m.Tools || m.Context != 262144 {
			t.Fatalf("priced router model = %#v", m)
		}
		if m := models["meta/llama"]; !m.Free || m.Priced {
			t.Fatalf("free router model = %#v", m)
		}
		// a null price is not a price of zero
		if m := models["null/prices"]; m.Free || m.Priced {
			t.Fatalf("unpriced router model = %#v", m)
		}
	})
	t.Run("opencode llm7 requesty", func(t *testing.T) {
		models := parsed(t, "", `{"data":[
			{"id":"big-pickle","type":"chat","capabilities":{"tools":{"supported":true}},"cost":{"text":{"input":{"price":0},"output":{"price":0}}}},
			{"id":"claude-fable","type":"chat","cost":{"text":{"input":{"price":10},"output":{"price":50}}}},
			{"id":"Inkling","model_type":"chat","tier":"pro","context_window":{"tokens":512000},"capabilities":{"tools":true}},
			{"id":"flux-image","model_type":"image"},
			{"id":"novita/ling","api":"chat","context_window":262144,"input_price":0,"output_price":0,"pricing":[{"input_price":0,"output_price":0}],"supports_tool_calling":true},
			{"id":"novita/paid","api":"chat","input_price":0.000002,"output_price":0.00001,"supports_tool_calling":false},
			{"id":"embedder","api":"embedding"}
		]}`)
		if m := models["big-pickle"]; !m.Free || !m.Tools {
			t.Fatalf("opencode free = %#v", m)
		}
		if m := models["claude-fable"]; !m.Priced {
			t.Fatalf("opencode priced = %#v", m)
		}
		if m := models["Inkling"]; !m.Priced || m.Context != 512000 || !m.Tools {
			t.Fatalf("llm7 = %#v", m)
		}
		if m := models["novita/ling"]; !m.Free || !m.Tools || m.Context != 262144 {
			t.Fatalf("requesty free = %#v", m)
		}
		if m := models["novita/paid"]; !m.Priced || !m.ToolsKnown || m.Tools {
			t.Fatalf("requesty paid = %#v", m)
		}
		if len(models) != 5 {
			t.Fatalf("non-chat models listed: %#v", models)
		}
	})
	t.Run("bare list", func(t *testing.T) {
		models := parsed(t, "", `[{"id":"typhoon-v2.5","display_name":"Typhoon 2.5"}]`)
		if models["typhoon-v2.5"].Name != "Typhoon 2.5" {
			t.Fatalf("typhoon = %#v", models)
		}
	})
}

func TestNativeListings(t *testing.T) {
	page, _ := url.Parse("https://api.anthropic.com/v1/models?limit=1000")
	models, next, err := parseListing(shapeAnthropic, []byte(`{"data":[{"id":"claude-x","display_name":"Claude X","type":"model"}],"has_more":true,"last_id":"claude-x"}`), page)
	if err != nil || len(models) != 1 || !models[0].Tools || !models[0].ToolsKnown || models[0].Name != "Claude X" || next == nil || next.Query().Get("after_id") != "claude-x" || next.Query().Get("limit") != "1000" {
		t.Fatalf("anthropic = %#v, next %v, %v", models, next, err)
	}

	page, _ = url.Parse("https://api.cloudflare.com/client/v4/accounts/a/ai/models/search?per_page=1")
	models, next, err = parseListing(shapeCloudflare, []byte(`{"result":[
		{"name":"@cf/meta/llama","task":{"name":"Text Generation"},"properties":[{"property_id":"context_window","value":"24000"},{"property_id":"function_calling","value":"true"}]},
		{"name":"@cf/flux","task":{"name":"Text-to-Image"}}
	],"result_info":{"page":1,"per_page":2,"total_count":3}}`), page)
	if err != nil || len(models) != 1 || models[0].ID != "@cf/meta/llama" || models[0].Context != 24000 || !models[0].Tools || next == nil || next.Query().Get("page") != "2" {
		t.Fatalf("cloudflare = %#v, next %v, %v", models, next, err)
	}

	page, _ = url.Parse("https://api.cohere.com/v1/models?endpoint=chat")
	models, next, err = parseListing(shapeCohere, []byte(`{"models":[
		{"name":"command-a","endpoints":["chat","generate"],"context_length":256000,"features":["tools","json_mode"]},
		{"name":"embed-v4","endpoints":["embed"]}
	]}`), page)
	if err != nil || len(models) != 1 || !models[0].Tools || models[0].Context != 256000 || next != nil {
		t.Fatalf("cohere = %#v, next %v, %v", models, next, err)
	}

	models, _, err = parseListing(shapeGitHub, []byte(`[
		{"id":"openai/gpt-4.1","name":"GPT-4.1","supported_input_modalities":["text","image"],"supported_output_modalities":["text"],"capabilities":["streaming","tool-calling"],"limits":{"max_input_tokens":1048576}},
		{"id":"openai/text-embedding-3","supported_input_modalities":["text"],"supported_output_modalities":["embeddings"]}
	]`), nil)
	if err != nil || len(models) != 1 || !models[0].Tools || !models[0].Vision || models[0].Context != 1048576 {
		t.Fatalf("github = %#v, %v", models, err)
	}
}

// Free stays honest: a mixed provider's model is free only when its listing
// says so, a free provider's priced model is paid, a renewed-credits
// provider spends its credits, and a keyless provider offers nothing it
// would charge for.
func TestListedTier(t *testing.T) {
	openrouter := &provider{Tier: protocol.TierCredits, Listing: listingSpec{Mixed: true}}
	groq := &provider{Tier: protocol.TierFree}
	inferbase := &provider{Tier: protocol.TierCredits}
	kilo := &provider{Tier: protocol.TierFree, Keyless: true, Listing: listingSpec{Mixed: true}}
	ovh := &provider{Tier: protocol.TierFree, Keyless: true}
	openai := &provider{Tier: protocol.TierPaid}
	for _, c := range []struct {
		name    string
		p       *provider
		m       listedModel
		tier    string
		offered bool
	}{
		{"openrouter :free", openrouter, listedModel{Free: true}, protocol.TierCredits, true},
		{"openrouter priced", openrouter, listedModel{Priced: true}, protocol.TierPaid, true},
		{"openrouter unpriced", openrouter, listedModel{}, protocol.TierPaid, true},
		{"free provider unpriced", groq, listedModel{}, protocol.TierFree, true},
		{"free provider priced", groq, listedModel{Priced: true}, protocol.TierPaid, true},
		{"credits provider priced", inferbase, listedModel{Priced: true}, protocol.TierCredits, true},
		{"keyless mixed free", kilo, listedModel{Free: true}, protocol.TierFree, true},
		{"keyless mixed priced", kilo, listedModel{Priced: true}, "", false},
		{"keyless priced", ovh, listedModel{Priced: true}, protocol.TierFree, true},
		{"paid provider free mark", openai, listedModel{Free: true}, protocol.TierPaid, true},
	} {
		tier, offered := listedTier(c.p, c.m)
		if tier != c.tier || offered != c.offered {
			t.Errorf("%s: tier %q offered %v, want %q %v", c.name, tier, offered, c.tier, c.offered)
		}
	}
}

// A curated model keeps what the catalogue says of it and only takes the
// context it lacked; the listing's own models follow, with the tier the rules
// give and whether their tools are known.
func TestMergeKeepsCuratedFacts(t *testing.T) {
	p := &provider{ID: "router", Tier: protocol.TierCredits, Listing: listingSpec{Mixed: true}, Models: []model{
		{ID: "Curated:free", Name: "Curated", Context: 32000, Tools: true, QualityRank: 90},
		{ID: "no-context:free", Name: "No context", Tools: true},
	}}
	merged := mergeListing(p, listingRecord{Models: []listedModel{
		{ID: "curated:free", Name: "Listed name", Context: 5, Free: true},
		{ID: "no-context:free", Context: 64000, Free: true},
		{ID: "new:free", Free: true, ToolsKnown: true, Tools: true, Context: 128000},
		{ID: "mystery:free", Free: true},
		{ID: "priced", Priced: true, ToolsKnown: true, Tools: true},
	}})
	if merged == p || len(p.Models) != 2 {
		t.Fatal("the catalogue provider was changed")
	}
	byID := map[string]model{}
	for _, m := range merged.Models {
		byID[m.ID] = m
	}
	if len(merged.Models) != 5 {
		t.Fatalf("merged = %#v", merged.Models)
	}
	if m := byID["Curated:free"]; m.Name != "Curated" || m.Context != 32000 || m.QualityRank != 90 || m.listed {
		t.Fatalf("curated = %#v", m)
	}
	if byID["no-context:free"].Context != 64000 {
		t.Fatalf("context not taken: %#v", byID["no-context:free"])
	}
	if m := byID["new:free"]; !m.listed || m.toolsUnknown || !m.Tools || merged.tierOf(&m) != protocol.TierCredits || m.Name != "new:free" {
		t.Fatalf("listed = %#v", m)
	}
	if m := byID["mystery:free"]; !m.toolsUnknown || m.Tools {
		t.Fatalf("unknown tools = %#v", m)
	}
	if m := byID["priced"]; merged.tierOf(&m) != protocol.TierPaid {
		t.Fatalf("priced = %#v", m)
	}
}

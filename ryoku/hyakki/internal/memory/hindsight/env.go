package hindsight

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
)

// launch is what one start of the child is given. Every secret in it goes
// to the child's environment only: never its arguments, a file or a log.
type launch struct {
	port     int    // the API's loopback port
	dbPort   int    // Postgres's loopback port
	key      string // the key the API asks of every caller
	password string // the database user's password, set when pg0 first makes it
	models   string // the loopback base URL of the gateway's API
	modelKey string // the gateway's API key
	offline  bool   // models are already fetched
}

// args are the child's arguments: a loopback host, never the default
// 0.0.0.0, and nothing secret.
func (l launch) args() []string {
	return []string{"--host", "127.0.0.1", "--port", fmt.Sprint(l.port)}
}

// env is the child's whole environment. It inherits nothing from rashind,
// so no key of Rashin's or the user's reaches it, and it turns off what
// Hindsight does by default that Rashin does not want: no auth, the pg0
// default password, kept LLM traces and source text, the MCP endpoint, file
// uploads and dry-run extraction, and reaching the network for models after
// setup.
func (l launch) env(layout Layout) []string {
	database := url.URL{Scheme: "pg0", User: url.UserPassword(instance, l.password), Host: net.JoinHostPort(instance, strconv.Itoa(l.dbPort))}
	env := []string{
		"PATH=/usr/local/bin:/usr/bin:/bin",
		"LANG=C.UTF-8",
		"HOME=" + layout.home(),
		"XDG_CACHE_HOME=" + layout.Cache,
		"HF_HOME=" + layout.models(),
		"HF_HUB_DISABLE_TELEMETRY=1",
		"DO_NOT_TRACK=1",
		"HINDSIGHT_API_DATABASE_URL=" + database.String(),
		"HINDSIGHT_API_TENANT_EXTENSION=hindsight_api.extensions.builtin.tenant:ApiKeyTenantExtension",
		"HINDSIGHT_API_TENANT_API_KEY=" + l.key,
		"HINDSIGHT_API_LLM_PROVIDER=openai",
		"HINDSIGHT_API_LLM_BASE_URL=" + l.models,
		"HINDSIGHT_API_LLM_API_KEY=" + l.modelKey,
		// the listener puts the learning model into every request, so this only names it
		"HINDSIGHT_API_LLM_MODEL=" + protocolModel,
		"HINDSIGHT_API_SKIP_LLM_VERIFICATION=true",
		"HINDSIGHT_API_EMBEDDINGS_PROVIDER=onnx",
		"HINDSIGHT_API_RERANKER_PROVIDER=rrf",
		"HINDSIGHT_API_LLM_TRACE_ENABLED=false",
		"HINDSIGHT_API_STORE_DOCUMENT_TEXT=false",
		"HINDSIGHT_API_MCP_ENABLED=false",
		"HINDSIGHT_API_ENABLE_FILE_UPLOAD_API=false",
		"HINDSIGHT_API_ENABLE_DRY_RUN_EXTRACT=false",
		"HINDSIGHT_API_OTEL_TRACES_ENABLED=false",
	}
	if l.offline {
		env = append(env, "HF_HUB_OFFLINE=1")
	}
	return env
}

// protocolModel is the model name Hindsight is configured with; the model
// listener replaces it.
const protocolModel = "rashin-memory"

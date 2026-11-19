package protocol

import (
	"maps"
	"net/netip"
	"net/url"
	"strings"
)

// Local providers: model servers on this computer or the local network that
// speak the OpenAI chat API. They need no key; Rashin finds their models by
// asking the server. ProviderRashinModels is llama.cpp run by Rashin itself:
// it has no address setting, since Rashin picks a loopback port each time it
// starts the server.
const (
	ProviderRashinModels = "rashin-models"
	ProviderLlamaCpp     = "llamacpp"
	ProviderOllamaLocal  = "ollama-local"
)

// LocalProviders are the local providers' ids in routing and display order:
// Rashin's own models first, then the user's llama.cpp, then Ollama.
var LocalProviders = []string{ProviderRashinModels, ProviderLlamaCpp, ProviderOllamaLocal}

// Equal includes each model's override when deciding whether to refresh routing.
func (s LocalModelsSettings) Equal(other LocalModelsSettings) bool {
	return s.Binary == other.Binary &&
		s.IdleMinutes == other.IdleMinutes &&
		s.Device == other.Device &&
		s.Light == other.Light &&
		s.RemoteFirst == other.RemoteFirst &&
		maps.Equal(s.Overrides, other.Overrides)
}

// AddressedLocalProviders are the local providers whose address the user
// sets in ProvidersSettings.Local.
var AddressedLocalProviders = []string{ProviderLlamaCpp, ProviderOllamaLocal}

// DefaultLocalAddresses are where each addressed local provider listens out
// of the box.
func DefaultLocalAddresses() map[string]string {
	return map[string]string{
		ProviderLlamaCpp:    "http://127.0.0.1:8080/v1",
		ProviderOllamaLocal: "http://127.0.0.1:11434/v1",
	}
}

// LightParams is the largest parameter count a model may have and still be
// light: light work runs on it and the engine gives it the compact profile.
const LightParams int64 = 4_500_000_000

// Idle stop of Rashin's llama-server: LocalModelsSettings.IdleMinutes.
const (
	DefaultLocalIdleMinutes = 5
	LocalIdleMinutesMax     = 120
)

const (
	LocalDeviceDiscrete    = "discrete"
	LocalDeviceIntegrated  = "integrated"
	LocalDeviceCPU         = "cpu"
	LocalKVF16             = "f16"
	LocalKVQ8              = "q8_0"
	LocalMinContext        = 8192
	LocalVerdictCard       = "card"
	LocalVerdictShared     = "shared"
	LocalVerdictSplit      = "split"
	LocalVerdictCPU        = "cpu"
	LocalVerdictTooBig     = "too_big"
	LocalVerdictUnsuitable = "unsuitable"
	LocalPhaseCopy         = "copy"
)

// Local model classes: LocalModel.Class.
const (
	LocalClassLight = "light"
	LocalClassMain  = "main"
)

// Local model states: LocalModel.State. Paused is a download stopped with
// its part files kept, so it resumes; Broken is an installed model whose
// files no longer match.
const (
	LocalAvailable   = "available"
	LocalDownloading = "downloading"
	LocalPaused      = "paused"
	LocalInstalled   = "installed"
	LocalBroken      = "broken"
)

// Runtime states: LocalRuntime.State. Missing means no llama-server Rashin
// may use; TooOld one below the model's MinBuild.
const (
	RuntimeMissing  = "missing"
	RuntimeTooOld   = "too_old"
	RuntimeStopped  = "stopped"
	RuntimeStopping = "stopping"
	RuntimeStarting = "starting"
	RuntimeReady    = "ready"
	RuntimeFailed   = "failed"
)

// Where the llama-server binary comes from: LocalRuntime.Source.
const (
	RuntimeConfigured = "configured"
	RuntimeManaged    = "managed"
	RuntimeSystem     = "system"
)

// Runtime backends Rashin can install: LocalRuntime.Offer and
// LocalRuntimeParams.Backend.
const (
	BackendVulkan = "vulkan"
	BackendCPU    = "cpu"
	BackendCUDA   = "cuda"
)

// Local file roles: LocalFile.Role.
const (
	LocalFileModel  = "model"
	LocalFileMmproj = "mmproj"
)

// Download phases: LocalProgress.Phase.
const (
	LocalPhaseDownload  = "download"
	LocalPhaseVerify    = "verify"
	LocalPhaseDone      = "done"
	LocalPhaseError     = "error"
	LocalPhaseCancelled = "cancelled"
)

// LocalRuntimeID is LocalProgress.ID while the managed runtime downloads.
const LocalRuntimeID = "runtime"

// LocalFile is one file of a local model, verified against SHA256 before it
// counts as installed. Done is how much of it is on disk.
type LocalFile struct {
	Name        string    `json:"name"`
	Role        string    `json:"role"`
	Size        int64     `json:"size"`
	SHA256      string    `json:"sha256"`
	Params      int64     `json:"params,omitempty"`
	Context     int       `json:"context,omitempty"`
	Tools       bool      `json:"tools,omitempty"`
	Warning     string    `json:"warning,omitempty"`
	Quant       string    `json:"quant,omitempty"`
	Parts       int       `json:"parts,omitempty"`
	Plan        LocalPlan `json:"plan,omitempty"`
	Verdict     string    `json:"verdict,omitempty"`
	Recommended bool      `json:"recommended,omitempty"`
	Done        int64     `json:"done"`
}

// LocalLicense is a model's licence as its card names it; Note is what it
// asks of the user in a sentence, such as a revenue limit.
type LocalLicense struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
	Note string `json:"note,omitempty"`
}

// LocalModel is one model Rashin can run itself: from its catalogue
// (Catalogue) or added from Hugging Face, pinned to Revision. ID is what it
// is routed as, rashin-models/<ID>.
type LocalModel struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Repo      string       `json:"repo"`
	Revision  string       `json:"revision"`
	Files     []LocalFile  `json:"files"`
	License   LocalLicense `json:"license"`
	Params    int64        `json:"params,omitempty"`
	Context   int          `json:"context,omitempty"`
	Warning   string       `json:"warning,omitempty"`
	Class     string       `json:"class,omitempty"`
	Tools     bool         `json:"tools,omitempty"`
	Vision    bool         `json:"vision,omitempty"`
	Audio     bool         `json:"audio,omitempty"`
	Video     bool         `json:"video,omitempty"`
	Reasoning bool         `json:"reasoning,omitempty"`
	Catalogue bool         `json:"catalogue,omitempty"`
	State     string       `json:"state"`
	Error     string       `json:"error,omitempty"`
	Plan      LocalPlan    `json:"plan,omitempty"`
}

// LocalDevice is one device llama-server can run on, as --list-devices
// reports it.
type LocalDevice struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	TotalMiB int    `json:"totalMiB"`
	FreeMiB  int    `json:"freeMiB"`
	Kind     string `json:"kind,omitempty"`
}

// LocalPlan describes a fresh hardware estimate and, when loaded, the observed context.
type LocalPlan struct {
	Device         string `json:"device,omitempty"`
	DeviceName     string `json:"deviceName,omitempty"`
	Kind           string `json:"kind,omitempty"`
	Context        int    `json:"context,omitempty"`
	ActualContext  int    `json:"actualContext,omitempty"`
	GPULayers      int    `json:"gpuLayers,omitempty"`
	Layers         int    `json:"layers,omitempty"`
	KVType         string `json:"kvType,omitempty"`
	EstimatedMiB   int    `json:"estimatedMiB,omitempty"`
	Reason         string `json:"reason,omitempty"`
	Verdict        string `json:"verdict,omitempty"`
	Retry          string `json:"retry,omitempty"`
	ContextOptions []int  `json:"contextOptions,omitempty"`
}

// LocalModelOverride contains only settings the user explicitly replaced.
type LocalModelOverride struct {
	Device  string `json:"device,omitempty"`
	Context int    `json:"context,omitempty"`
	KVType  string `json:"kvType,omitempty"`
}

// LocalRuntime is the llama-server Rashin runs its models with. Address is
// set while it runs; Loaded are the model ids in memory; Log is the path of
// its log. Offer are the backends Rashin can install on this computer.
type LocalRuntime struct {
	State       string               `json:"state"`
	Source      string               `json:"source,omitempty"`
	Path        string               `json:"path,omitempty"`
	Build       int                  `json:"build,omitempty"`
	Version     string               `json:"version,omitempty"`
	MinBuild    int                  `json:"minBuild,omitempty"`
	Backend     string               `json:"backend,omitempty"`
	Devices     []LocalDevice        `json:"devices,omitempty"`
	Address     string               `json:"address,omitempty"`
	ActiveModel string               `json:"activeModel,omitempty"`
	LoadedMiB   int                  `json:"loadedMiB,omitempty"`
	Loaded      []string             `json:"loaded,omitempty"`
	Plans       map[string]LocalPlan `json:"plans,omitempty"`
	Error       string               `json:"error,omitempty"`
	Log         string               `json:"log,omitempty"`
	Offer       []string             `json:"offer,omitempty"`
}

// LocalResult is the local.models result and the local.changed data: the
// catalogue with what is installed, the runtime, and the disk the models
// take.
type LocalResult struct {
	Models    []LocalModel `json:"models"`
	Runtime   LocalRuntime `json:"runtime"`
	DiskBytes int64        `json:"diskBytes"`
}

// LocalDevicesResult is the runtime's cached devices after a deliberate UI probe.
type LocalDevicesResult struct {
	Devices []LocalDevice `json:"devices"`
}

// LocalProgress is the data of local.progress: one download's progress. ID
// is a model id or LocalRuntimeID; Phase is one of the Phase* values.
type LocalProgress struct {
	ID    string `json:"id"`
	File  string `json:"file,omitempty"`
	Done  int64  `json:"done"`
	Total int64  `json:"total"`
	Phase string `json:"phase"`
	Error string `json:"error,omitempty"`
}

// LocalModelParams names one local model; for local.start and local.stop ID
// may be empty.
type LocalModelParams struct {
	ID string `json:"id"`
}

// LocalAddParams accepts a repository, a Hugging Face URL, or an absolute
// local GGUF path. File and Mmproj override the ranked recommendations.
type LocalAddParams struct {
	Repo     string `json:"repo"`
	File     string `json:"file"`
	Mmproj   string `json:"mmproj,omitempty"`
	Revision string `json:"revision,omitempty"`
}

// LocalSearchParams previews a repository, Hugging Face URL, or local GGUF.
type LocalSearchParams struct {
	Repo string `json:"repo"`
}

// LocalSearchResult pins a remote revision; Path is the selected local source
// or a GGUF filename supplied by a Hugging Face URL.
type LocalSearchResult struct {
	Repo        string      `json:"repo"`
	Revision    string      `json:"revision"`
	License     string      `json:"license,omitempty"`
	Gated       bool        `json:"gated,omitempty"`
	Files       []LocalFile `json:"files"`
	Mmproj      string      `json:"mmproj,omitempty"`
	Path        string      `json:"path,omitempty"`
	Recommended string      `json:"recommended,omitempty"`
}

// LocalRuntimeParams picks the backend local.runtime_install downloads; ""
// takes BackendVulkan.
type LocalRuntimeParams struct {
	Backend string `json:"backend,omitempty"`
}

// ParseLocalAddress normalises the address of a local provider's API and
// reports whether it may be used: http or https, no credentials, query or
// fragment, and a host that is localhost or a loopback or private-network IP
// address. A name other than localhost is refused, since what it resolves to
// can change. An address without a path gets /v1, the API's usual root.
func ParseLocalAddress(raw string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return "", false
	}
	host := u.Hostname()
	if !strings.EqualFold(host, "localhost") {
		ip, err := netip.ParseAddr(host)
		if err != nil || !PrivateIP(ip) {
			return "", false
		}
	}
	u.Host = strings.ToLower(u.Host)
	u.Path = strings.TrimRight(u.Path, "/")
	if u.Path == "" {
		u.Path = "/v1"
	}
	u.RawPath = ""
	return u.String(), true
}

// PrivateIP reports a loopback address or one on a private network (RFC 1918,
// RFC 4193). Link-local addresses are not private here: a cloud machine's
// metadata service lives there.
func PrivateIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	return ip.IsLoopback() || ip.IsPrivate()
}

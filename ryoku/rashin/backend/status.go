package main

import "time"

// Status is the full daemon report for `status --json`, the Hub page, and
// /api/status.
type Status struct {
	Enabled bool `json:"enabled"`
	Running bool `json:"running"`
	Port    int  `json:"port"`
	// Ready is true when the needle can actually answer: the local agent is
	// configured, or a direct quick-ask provider resolves. Drives the first-run
	// setup prompt in the needle.
	Ready  bool        `json:"ready"`
	Prowl  ProwlStatus `json:"prowl"`
	Vault  VaultStatus `json:"vault"`
	Hermes HermesInfo  `json:"hermes"`
	Agents []Agent     `json:"agents"`
}

type ProwlStatus struct {
	Installed bool   `json:"installed"`
	Bin       string `json:"bin"`
	Version   string `json:"version"`
	Running   bool   `json:"running"`
	Port      int    `json:"port"`
	URL       string `json:"url"`
	Repo      string `json:"repo"`
	Error     string `json:"error"`
}

// VaultStatus is the vault summary embedded in Status.
type VaultStatus struct {
	Path        string    `json:"path"`
	Exists      bool      `json:"exists"`
	Files       int       `json:"files"`
	LastIndexed time.Time `json:"lastIndexed"`
}

// BuildStatus assembles the report. Running is probed over the loopback API
// (pingDaemon, in server.go) so the answer reflects a live daemon, not just the
// enabled gate.
func BuildStatus(cfg Config) Status {
	files, last, exists := VaultStats()
	h := HermesStatus()
	ready := needleReady(h, cfg)
	p := prowlStatusNow()
	return Status{
		Enabled: cfg.Enabled,
		Running: pingDaemon(cfg.Port),
		Port:    cfg.Port,
		Ready:   ready,
		Prowl:   p,
		Vault: VaultStatus{
			Path:        VaultDir(),
			Exists:      exists,
			Files:       files,
			LastIndexed: last,
		},
		Hermes: h,
		Agents: DetectAgents(),
	}
}

func prowlStatusNow() ProwlStatus {
	status := ProwlStatus{
		Port: prowlGatewayPort(),
		URL:  prowlGatewayBase(),
		Repo: prowlRepo(),
	}
	bin, installed := findProwl()
	status.Installed = installed
	if !installed {
		status.Error = "Prowl is not installed"
		return status
	}
	status.Bin = bin
	status.Version = prowlVersion(bin)
	status.Running = prowlGatewayRunning()
	if !status.Running {
		status.Error = "Prowl's gateway is not running"
	}
	return status
}

// needleReady reports whether the needle can answer through a configured
// Hermes session or a routable Prowl gateway.
func needleReady(h HermesInfo, cfg Config) bool {
	if h.Configured {
		return true
	}
	_, err := resolveQuickTarget(cfg)
	return err == nil
}

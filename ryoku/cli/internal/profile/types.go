package profile

import "time"

const API = 1
const SystemRoot = "/var/lib/ryoku/profiles"
const WordlistsRoot = "/usr/share/wordlists"

var AllowedRepos = []string{"blackarch"}
var AllowedGroups = []string{"wireshark"}
var AllowedHostSuffixes = []string{"htb", "thm", "vl"}
var AllowedCapabilities = map[string]string{
	"nmap": "cap_net_raw,cap_net_admin,cap_net_bind_service=eip",
}

func ProfileBinDir(id string) string { return "/usr/local/lib/ryoku-profiles/" + id + "/bin" }
func GroupName(id string) string     { return "profile-" + id }

type Manifest struct {
	Schema       int              `json:"schema"`
	ID           string           `json:"id"`
	Version      string           `json:"version"`
	Name         string           `json:"name"`
	Requires     Requires         `json:"requires"`
	Account      AccountSpec      `json:"account"`
	Repos        []string         `json:"repos"`
	Components   []Component      `json:"components"`
	Capabilities []CapabilitySpec `json:"capabilities"`
	Wordlists    Wordlists        `json:"wordlists"`
	Hosts        HostsSpec        `json:"hosts"`
	Env          []EnvVar         `json:"env"`
	Path         []string         `json:"path"`
	Launcher     LauncherSpec     `json:"launcher"`
	Expand       []string         `json:"expand"`
	SeedOnce     []string         `json:"seedOnce"`
	UserData     []string         `json:"userData"`
	Rice         string           `json:"rice"`
	FirstRun     FirstRun         `json:"firstRun"`
}

type Requires struct {
	ProfileAPI      int      `json:"profileApi"`
	PackageManagers []string `json:"packageManagers"`
	Capabilities    []string `json:"capabilities"`
}

type AccountSpec struct {
	SuggestedName string   `json:"suggestedName"`
	FullName      string   `json:"fullName"`
	Role          string   `json:"role"`
	Permissions   []string `json:"permissions"`
	Groups        []string `json:"groups"`
	Shell         string   `json:"shell"`
}

type Component struct {
	Type           string   `json:"type"`
	Name           string   `json:"name"`
	Detect         string   `json:"detect"`
	Tier           string   `json:"tier"`
	Group          string   `json:"group"`
	Summary        string   `json:"summary"`
	DesktopEntries []string `json:"desktopEntries"`
}

type CapabilitySpec struct {
	Tool string `json:"tool"`
	Caps string `json:"caps"`
}
type Wordlists struct {
	Links   []WordlistLink    `json:"links"`
	Extract []WordlistExtract `json:"extract"`
}
type WordlistLink struct {
	Name   string `json:"name"`
	Target string `json:"target"`
}
type WordlistExtract struct {
	Archive string `json:"archive"`
	Member  string `json:"member"`
	Name    string `json:"name"`
}
type HostsSpec struct {
	Suffixes []string `json:"suffixes"`
}
type EnvVar struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}
type LauncherSpec struct {
	Allow []string `json:"allow"`
}
type FirstRun struct {
	Command []string `json:"command"`
	Message string   `json:"message"`
}
type Selection struct {
	Components []string `json:"components"`
	Without    []string `json:"without"`
	Extras     []string `json:"extras"`
}
type Vars struct{ Home, User, ID string }

type PackageRef struct {
	Name        string `json:"name"`
	Repo        string `json:"repo"`
	Version     string `json:"version"`
	Download    int64  `json:"download"`
	Installed   int64  `json:"installed"`
	Target      bool   `json:"target"`
	PreExisting bool   `json:"preExisting"`
	Component   string `json:"component,omitempty"`
	Group       string `json:"group,omitempty"`
}
type RemovalPackage struct {
	Name      string `json:"name"`
	Repo      string `json:"repo"`
	Installed int64  `json:"installed"`
}

type Resolver interface {
	Resolve(targets []string, extraRepos []string) ([]PackageRef, error)
	Installed(names []string) (map[string]bool, error)
	Required(names []string) (map[string][]string, error)
}

type PacmanResolverOptions struct{ ScratchDir, PacmanConf, DBPath, BlackArchServer string }
type HostFacts struct {
	PackageManager   string
	ManagedHomes     bool
	HomeRoot         string
	BlackArchEnabled bool
	Capabilities     map[string]bool
	Shells           []string
}
type FileHash struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type Source struct {
	Dir            string     `json:"dir"`
	ManifestSHA256 string     `json:"manifestSha256"`
	Files          []FileHash `json:"files"`
}
type AccountTarget struct {
	Name      string
	FullName  string
	Home      string
	Exists    bool
	UID       int
	ProfileID string
}

type PlanProfile struct {
	ID             string `json:"id"`
	Version        string `json:"version"`
	Name           string `json:"name"`
	ManifestSHA256 string `json:"manifestSha256"`
	Source         string `json:"source"`
}
type PlanAccount struct {
	Name        string   `json:"name"`
	FullName    string   `json:"fullName"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
	Groups      []string `json:"groups"`
	Shell       string   `json:"shell"`
	Exists      bool     `json:"exists"`
	Home        string   `json:"home"`
	ManagedHome bool     `json:"managedHome"`
}
type RepoPlan struct {
	ID     string `json:"id"`
	Action string `json:"action"`
	Detail string `json:"detail,omitempty"`
	Reason string `json:"reason,omitempty"`
}
type PlanTotals struct {
	Download     int64 `json:"download"`
	Installed    int64 `json:"installed"`
	Targets      int   `json:"targets"`
	Dependencies int   `json:"dependencies"`
}
type SystemPlan struct {
	Kind   string `json:"kind"`
	Path   string `json:"path"`
	Detail string `json:"detail,omitempty"`
	Action string `json:"action,omitempty"`
}
type FilesPlan struct {
	Managed  int    `json:"managed"`
	SeedOnce int    `json:"seedOnce,omitempty"`
	Home     string `json:"home,omitempty"`
}
type BleedEntry struct {
	DesktopID string `json:"desktopId"`
	Name      string `json:"name"`
	Package   string `json:"package"`
}

type Plan struct {
	Schema   int              `json:"schema"`
	Profile  PlanProfile      `json:"profile"`
	Account  PlanAccount      `json:"account"`
	Repos    []RepoPlan       `json:"repos"`
	Packages []PackageRef     `json:"packages"`
	Remove   []RemovalPackage `json:"remove"`
	Totals   PlanTotals       `json:"totals"`
	System   []SystemPlan     `json:"system"`
	Files    FilesPlan        `json:"files"`
	Env      []EnvVar         `json:"env"`
	Bleed    []BleedEntry     `json:"bleed"`
	Warnings []string         `json:"warnings"`
	Blockers []string         `json:"blockers"`
	Digest   string           `json:"digest"`
	manifest *Manifest
	source   Source
}

type Pinned struct {
	Profile struct {
		ID             string `json:"id"`
		Version        string `json:"version"`
		ManifestSHA256 string `json:"manifestSha256"`
	} `json:"profile"`
	Account struct {
		Name        string   `json:"name"`
		Role        string   `json:"role"`
		Permissions []string `json:"permissions"`
		Groups      []string `json:"groups"`
		Shell       string   `json:"shell"`
	} `json:"account"`
	Repos        []string         `json:"repos"`
	Targets      []string         `json:"targets"`
	Removals     []string         `json:"removals"`
	Capabilities []CapabilitySpec `json:"capabilities"`
	Wordlists    Wordlists        `json:"wordlists"`
	Hosts        HostsSpec        `json:"hosts"`
	Env          []EnvVar         `json:"env"`
	Path         []string         `json:"path"`
	Files        []FileHash       `json:"files"`
}

type Checkpoint struct {
	ID        string    `json:"id"`
	Label     string    `json:"label"`
	CreatedAt time.Time `json:"createdAt"`
}
type RepoClaim struct {
	ID               string `json:"id"`
	EnabledByProfile bool   `json:"enabledByProfile"`
}
type PackageClaims struct {
	Introduced  []string `json:"introduced"`
	PreExisting []string `json:"preExisting"`
}
type SystemFile struct {
	Kind   string `json:"kind"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type SystemReceipt struct {
	Schema  int `json:"schema"`
	Profile struct {
		ID             string `json:"id"`
		Version        string `json:"version"`
		ManifestSHA256 string `json:"manifestSha256"`
	} `json:"profile"`
	Account struct {
		Name    string `json:"name"`
		UID     int    `json:"uid"`
		Created bool   `json:"created"`
	} `json:"account"`
	AppliedAt     time.Time     `json:"appliedAt"`
	Staged        string        `json:"staged"`
	Repos         []RepoClaim   `json:"repos"`
	Packages      PackageClaims `json:"packages"`
	Groups        []string      `json:"groups"`
	GroupsCreated []string      `json:"groupsCreated"`
	System        []SystemFile  `json:"system"`
	Wordlists     []string      `json:"wordlists"`
	Hosts         *HostsSpec    `json:"hosts,omitempty"`
	Selection     Selection     `json:"selection"`
	Bleed         []string      `json:"bleed"`
	Checkpoints   []Checkpoint  `json:"checkpoints"`
}

type ManagedFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Mode   string `json:"mode"`
}
type HiddenEntry struct {
	ID           string `json:"id"`
	SourceSHA256 string `json:"sourceSha256"`
}
type AccountReceipt struct {
	Schema  int `json:"schema"`
	Profile struct {
		ID      string `json:"id"`
		Version string `json:"version"`
	} `json:"profile"`
	Payload string        `json:"payload"`
	Files   []ManagedFile `json:"files"`
	Seeded  []string      `json:"seeded"`
	EnvKeys []string      `json:"envKeys"`
	EnvFile string        `json:"envFile"`
	Scope   struct {
		Hidden []HiddenEntry `json:"hidden"`
	} `json:"scope"`
	UserData []string `json:"userData"`
}

type RemoveOptions struct {
	KeepAccount bool
	EraseFiles  bool
}
type RemovalAccount struct {
	Name    string `json:"name"`
	Action  string `json:"action"`
	Home    string `json:"home"`
	Archive string `json:"archive,omitempty"`
}
type KeptPackage struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}
type RemovalCheckpoint struct {
	ID     string `json:"id"`
	Action string `json:"action"`
}
type RemovalPlan struct {
	Schema       int                 `json:"schema"`
	Account      RemovalAccount      `json:"account"`
	Packages     []RemovalPackage    `json:"packages"`
	KeptPackages []KeptPackage       `json:"keptPackages"`
	Repos        []RepoPlan          `json:"repos"`
	System       []SystemPlan        `json:"system"`
	Files        FilesPlan           `json:"files"`
	UserData     []string            `json:"userData"`
	Checkpoints  []RemovalCheckpoint `json:"checkpoints"`
	DigestValue  string              `json:"digest"`
}

type Change struct {
	Path   string `json:"path"`
	Action string `json:"action"`
}
type DesktopEntry struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	Name      string `json:"name"`
	NoDisplay bool   `json:"noDisplay"`
	Hidden    bool   `json:"hidden"`
}
type Finding struct {
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
}
type CheckResult struct {
	OK       bool      `json:"ok"`
	At       time.Time `json:"at"`
	Failures []Finding `json:"failures"`
	Warnings []Finding `json:"warnings"`
}

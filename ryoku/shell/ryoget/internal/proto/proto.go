// Package proto is the ryoget socket wire: one compact JSON object per line.
// The bar's DownloadsClient.qml and the CLI speak it; docs live in the
// module README.
package proto

import "encoding/json"

const Version = 1

const MaxFrame = 1 << 20

type Request struct {
	ID     int64           `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

type Response struct {
	ID     int64  `json:"id"`
	Result any    `json:"result,omitempty"`
	Error  *Error `json:"error,omitempty"`
}

type Event struct {
	Event string `json:"event"`
	Data  any    `json:"data"`
}

const (
	CodeUnsupported = "unsupported"
	CodeUnavailable = "unavailable"
	CodePrivate     = "private"
	CodeRateLimited = "rate_limited"
	CodeNetwork     = "network"
	CodeInvalid     = "invalid"
	CodeBusy        = "busy"
	CodeInternal    = "internal"
	CodeNotFound    = "not_found"
	// the disk holding the folder has too little room left for the file
	CodeNoSpace = "no_space"
	// the site answers with a page that wants a browser to prove itself first
	CodeBrowserCheck = "browser_check"
	// the finished file does not match the checksum it came with; it is deleted
	CodeChecksum = "checksum"
)

const (
	StateQueued    = "queued"
	StateResolving = "resolving"
	StateActive    = "active"
	StatePaused    = "paused"
	StateMerging   = "merging"
	StateSeeding   = "seeding"
	StateDone      = "done"
	StateError     = "error"
	StateStopped   = "stopped"
)

type Source struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type File struct {
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Done     int64  `json:"done"`
	Selected bool   `json:"selected"`
}

type FormatRef struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type Task struct {
	ID       string     `json:"id"`
	URL      string     `json:"url"`
	Source   Source     `json:"source"`
	Kind     string     `json:"kind"`
	Title    string     `json:"title"`
	Name     string     `json:"name"`
	Dir      string     `json:"dir"`
	Path     string     `json:"path"`
	Thumb    string     `json:"thumb"`
	Mime     string     `json:"mime"`
	Size     int64      `json:"size"`
	Done     int64      `json:"done"`
	Speed    int64      `json:"speed"`
	ETA      int64      `json:"eta"`
	State    string     `json:"state"`
	Stage    string     `json:"stage"`
	Error    *Error     `json:"error"`
	Created  string     `json:"created"`
	Started  string     `json:"started"`
	Finished string     `json:"finished"`
	Conns    int        `json:"conns"`
	Pieces   string     `json:"pieces"`
	Peers    int        `json:"peers"`
	Seeds    int        `json:"seeds"`
	Up       int64      `json:"up"`
	UpSpeed  int64      `json:"upSpeed"`
	Ratio    float64    `json:"ratio"`
	Files    []File     `json:"files"`
	Format   *FormatRef `json:"format"`
	Mode     string     `json:"mode"`
	// when a home list first showed it settled; cleared when it runs again
	Seen string `json:"seen,omitempty"`
	// KiB/s for this download alone, 0 for none
	Limit int64 `json:"limit,omitempty"`
	// the checksum the landed file matched: "sha256", "sha1" or "md5"
	Verified string `json:"verified,omitempty"`
	// the server takes ranged requests, so a pause or a crash loses nothing
	Resumable bool `json:"resumable,omitempty"`
}

const PieceBuckets = 64

type Stats struct {
	Down    int64   `json:"down"`
	Up      int64   `json:"up"`
	Active  int     `json:"active"`
	Queued  int     `json:"queued"`
	Seeding int     `json:"seeding"`
	History []int64 `json:"history"`
}

type Format struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Label   string `json:"label"`
	Height  int    `json:"height"`
	FPS     int    `json:"fps"`
	Bitrate int    `json:"bitrate"`
	Ext     string `json:"ext"`
	Size    int64  `json:"size"`
	Codec   string `json:"codec"`
}

type Item struct {
	Title string `json:"title"`
	Thumb string `json:"thumb"`
	Kind  string `json:"kind"`
	URL   string `json:"url"`
}

type Resolved struct {
	Kind          string   `json:"kind"`
	URL           string   `json:"url"`
	Source        Source   `json:"source"`
	Title         string   `json:"title"`
	Name          string   `json:"name"`
	Size          int64    `json:"size"`
	Mime          string   `json:"mime"`
	Thumb         string   `json:"thumb"`
	Duration      float64  `json:"duration"`
	Author        string   `json:"author"`
	Resumable     bool     `json:"resumable"`
	Formats       []Format `json:"formats"`
	Items         []Item   `json:"items"`
	Files         []File   `json:"files"`
	DefaultFormat string   `json:"defaultFormat"`
	Note          string   `json:"note"`
	// the task already in the list for this link, so a card can say so before adding it twice
	Existing *Existing `json:"existing,omitempty"`
}

type Existing struct {
	ID    string `json:"id"`
	State string `json:"state"`
	Path  string `json:"path,omitempty"`
}

type Settings struct {
	Dir           string  `json:"dir"`
	MaxActive     int     `json:"maxActive"`
	Connections   int     `json:"connections"`
	MinSplitMiB   int     `json:"minSplitMiB"`
	DownLimit     int64   `json:"downLimit"`
	UpLimit       int64   `json:"upLimit"`
	VideoQuality  string  `json:"videoQuality"`
	AudioFormat   string  `json:"audioFormat"`
	SeedRatio     float64 `json:"seedRatio"`
	SeedMinutes   int     `json:"seedMinutes"`
	Seed          bool    `json:"seed"`
	DHT           bool    `json:"dht"`
	Port          int     `json:"port"`
	Notify        bool    `json:"notify"`
	ResumeOnStart bool    `json:"resumeOnStart"`
	YtdlpFallback bool    `json:"ytdlpFallback"`
	// days a finished, failed or stopped download stays in the list; 0 keeps it until cleared
	KeepDays int `json:"keepDays"`
	// new files land in a folder per kind (Videos, Music, ...) inside Dir
	SortByType bool `json:"sortByType"`
	// an audio download carries its cover picture
	CoverArt bool `json:"coverArt"`
}

type Hello struct {
	Version  int      `json:"version"`
	Settings Settings `json:"settings"`
	Tasks    []Task   `json:"tasks"`
	Stats    Stats    `json:"stats"`
}

type AddParams struct {
	URL    string `json:"url"`
	Mode   string `json:"mode,omitempty"`
	Format string `json:"format,omitempty"`
	Dir    string `json:"dir,omitempty"`
	Name   string `json:"name,omitempty"`
	Files  []int  `json:"files,omitempty"`
	// what the client already resolved; shown until the task's own resolution names it
	Title  string `json:"title,omitempty"`
	Paused bool   `json:"paused,omitempty"`
	// request headers a site needs (Cookie, Referer, User-Agent); dropped once the file lands
	Headers map[string]string `json:"headers,omitempty"`
	// "sha256:<hex>", "sha1:<hex>" or "md5:<hex>"; a "#sha256=<hex>" fragment on the URL says the same
	Checksum string `json:"checksum,omitempty"`
}

type IDParams struct {
	ID  string `json:"id,omitempty"`
	All bool   `json:"all,omitempty"`
}

// Link is one link found in pasted text, after cleaning.
type Link struct {
	// as it appeared in the text
	Raw string `json:"raw"`
	URL string `json:"url"`
	// "url", "magnet" or "torrent"
	Kind string `json:"kind"`
	Host string `json:"host"`
	// https-added, trimmed, unwrapped, tracking-removed, canonical
	Notes     []string `json:"notes"`
	Duplicate bool     `json:"duplicate"`
}

type Links struct {
	Links      []Link `json:"links"`
	Total      int    `json:"total"`
	Cleaned    int    `json:"cleaned"`
	Duplicates int    `json:"duplicates"`
}

type AddManyItem struct {
	URL    string `json:"url"`
	Mode   string `json:"mode,omitempty"`
	Format string `json:"format,omitempty"`
	Files  []int  `json:"files,omitempty"`
	Title  string `json:"title,omitempty"`
}

type AddManyParams struct {
	Items  []AddManyItem `json:"items"`
	Dir    string        `json:"dir,omitempty"`
	Paused bool          `json:"paused,omitempty"`
}

type ItemError struct {
	Index   int    `json:"index"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type AddManyResult struct {
	Tasks  []Task      `json:"tasks"`
	Errors []ItemError `json:"errors"`
}

package protocol

import "time"

type DiskUsageParams struct {
	Refresh bool `json:"refresh,omitempty"`
}
type DiskFile struct {
	Path     string    `json:"path"`
	Bytes    int64     `json:"bytes"`
	Modified time.Time `json:"modified"`
	Device   uint64    `json:"-"`
	Inode    uint64    `json:"-"`
	Task     string    `json:"-"`
}
type DiskItem struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Bytes     int64      `json:"bytes"`
	Partial   bool       `json:"partial,omitempty"`
	Cleanable bool       `json:"cleanable"`
	Files     []DiskFile `json:"files,omitempty"`
	Plan      string     `json:"plan,omitempty"`
}
type DiskUsage struct {
	Free     uint64     `json:"free"`
	Total    uint64     `json:"total"`
	Items    []DiskItem `json:"items"`
	Measured time.Time  `json:"measured"`
}

// DiskCleanParams confirms one immutable file manifest returned by disk.usage.
type DiskCleanParams struct {
	Plan string `json:"plan"`
}

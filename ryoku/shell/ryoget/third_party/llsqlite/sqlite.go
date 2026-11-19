// Package sqlite stands in for github.com/go-llsqlite/adapter, which
// anacrolix/torrent imports only in a file built with cgo and without the
// nosqlite tag. ryoget builds neither, so the real adapter (and the pure-Go
// SQLite behind it) never compiles and stays out of the vendor tree.
package sqlite

# go-libutp

[![Go Reference](https://pkg.go.dev/badge/github.com/anacrolix/go-libutp.svg)](https://pkg.go.dev/github.com/anacrolix/go-libutp)
[![Go](https://github.com/anacrolix/go-libutp/actions/workflows/go.yml/badge.svg)](https://github.com/anacrolix/go-libutp/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/anacrolix/go-libutp)](https://goreportcard.com/report/github.com/anacrolix/go-libutp)

A Go wrapper for [libutp](https://github.com/bittorrent/libutp), BitTorrent's reference
implementation of the Micro Transport Protocol (µTP). µTP is a reliable, ordered, stream-oriented
transport that runs over UDP and backs off in the presence of competing traffic, so bulk transfers
don't saturate the link they share with interactive traffic.

The libutp sources are vendored in this repository, so there's no external C library to install.
Building requires cgo and a C++ compiler.

## Install

```sh
go get github.com/anacrolix/go-libutp
```

The import path is `github.com/anacrolix/go-libutp`; the package name is `utp`.

## Usage

`Socket` implements both `net.Listener` and `net.PacketConn`, and the connections it hands out
implement `net.Conn`, so µTP mostly drops into code written against TCP.

Dialling:

```go
s, err := utp.NewSocket("udp", ":0")
if err != nil {
	return err
}
defer s.Close()

c, err := s.DialContext(ctx, "udp", "example.com:4242")
if err != nil {
	return err
}
defer c.Close()

_, err = io.WriteString(c, "hello")
```

Accepting:

```go
s, err := utp.NewSocket("udp", ":4242")
if err != nil {
	return err
}
defer s.Close()

for {
	c, err := s.Accept()
	if err != nil {
		return err
	}
	go handle(c)
}
```

`Socket.Dial` and `Socket.DialTimeout` are also available for the simpler cases.

### Wrapping an existing PacketConn

`NewSocketFromPacketConn` runs µTP over a `net.PacketConn` you already have, which is useful when
the port is shared with another protocol or comes from elsewhere:

```go
pc, err := net.ListenPacket("udp", ":4242")
if err != nil {
	return err
}
s, err := utp.NewSocketFromPacketConn(pc)
```

### Sharing a port with non-µTP traffic

Packets that aren't µTP are not dropped: `Socket` implements `net.PacketConn`, and its `ReadFrom`
and `WriteTo` carry exactly those packets. That's how a single UDP port can serve µTP connections
and something else — a DHT, say — at the same time.

Two caveats on that `net.PacketConn`: non-µTP packets are buffered and dropped if the reader
doesn't keep up, and the deadline methods on `Socket` are unimplemented (they panic). Deadlines on
an accepted or dialled `Conn` work normally.

### Options

- `utp.WithLogger(l)` — pass to `NewSocket`/`NewSocketFromPacketConn` to give a socket its own
  `*slog.Logger`, instead of the package-level `utp.Logger`. That one is nil until you set it,
  which means sockets log to `slog.Default()`.
- `Socket.SetFirewallCallback` and `Socket.SetSyncFirewallCallback` — reject incoming connections
  before they're acknowledged, so the peer sees no response at all rather than an accept followed
  by a close. Prefer the synchronous variant; it's called under the package lock and is consulted
  for every incoming connection.
- `Socket.SetOption` — set the underlying libutp context options directly.

## Pure Go implementation, and choosing at build time

[go-utp](https://github.com/anacrolix/go-utp) is a companion module with a µTP implementation
written in Go — a port of the same libutp sources vendored here, needing no cgo and no C++
compiler — and one interface over both, so the implementation can be chosen when you build:

```go
import "github.com/anacrolix/go-utp"

s, err := utp.NewSocket("udp", ":4242")   // the implementation the build selected
```

Its `utp.Default` is this package. The `purego` build tag, or cgo being off, makes it the pure Go
implementation instead, and the C++ sources here are then not compiled at all. Its interop tests
run the two against each other on the wire, in both roles, over a link that drops, delays and
duplicates packets.

## ucat

`cmd/ucat` is a netcat-alike over µTP, handy for smoke-testing:

```sh
go run ./cmd/ucat -l :4242         # listen
go run ./cmd/ucat localhost:4242   # dial, then pipe stdin/stdout
```

## Development

The [justfile](justfile) mirrors the CI jobs, so what you run locally is what CI runs:

```sh
just test          # go test -race -count 2 ./...
just bench         # build/smoke the benchmarks
just asan          # tests under LeakSanitizer
```

`just asan` is clean on Linux and macOS; see [lsan_suppressions.txt](lsan_suppressions.txt) for the
macOS system-library allocations it has to ignore.

## Release history

See [CHANGELOG.md](CHANGELOG.md).

## License

MIT, inherited from libutp. See [LICENSE](LICENSE).

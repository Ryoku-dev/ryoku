# go-utp

[![Go Reference](https://pkg.go.dev/badge/github.com/anacrolix/go-utp.svg)](https://pkg.go.dev/github.com/anacrolix/go-utp)
[![Go](https://github.com/anacrolix/go-utp/actions/workflows/go.yml/badge.svg)](https://github.com/anacrolix/go-utp/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/anacrolix/go-utp)](https://goreportcard.com/report/github.com/anacrolix/go-utp)

The Micro Transport Protocol (µTP) for Go: one interface over two implementations, with the choice
made when you build.

µTP is a reliable, ordered, stream-oriented transport that runs over UDP and backs off in the
presence of competing traffic, so bulk transfers don't saturate the link they share with
interactive traffic. Its congestion controller, LEDBAT, measures the one way queuing delay towards
the peer and grows the window towards a fixed delay target rather than until something is dropped.

The two implementations are:

- [libutp](https://github.com/anacrolix/go-libutp), BitTorrent's C++ reference implementation
  reached through cgo. It's what the rest of the µTP network is running.
- [`purego`](purego), a port of those same sources to Go. No cgo, no C++ compiler, and nothing
  outside the standard library.

## Install

```sh
go get github.com/anacrolix/go-utp
```

The import path is `github.com/anacrolix/go-utp`; the package name is `utp`.

## Usage

`Socket` implements both `net.Listener` and `net.PacketConn`, and the connections it hands out
implement `net.Conn`, so µTP mostly drops into code written against TCP.

Dialling:

```go
import "github.com/anacrolix/go-utp"

s, err := utp.NewSocket("udp", ":0")
if err != nil {
	return err
}
defer s.Close()

c, err := s.DialContext(ctx, "", "example.com:4242")
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

`Socket.Dial` and `Socket.DialTimeout` are also available for the simpler cases, and
`NewSocketFromPacketConn` runs µTP over a `net.PacketConn` you already have.

## Picking an implementation

`utp.Default` is libutp. Either of two build tags, or cgo being off, makes it `utp.PureGo`
instead, and the C++ sources are then not compiled at all:

```sh
go build -tags purego ./...       # the package this tag selects shares its name
go build -tags purego_utp ./...   # the same choice, for this module alone
CGO_ENABLED=0 go build ./...
```

`purego` is the tag a build sets to say it wants no cgo anywhere, and plenty of modules answer to
it. `purego_utp` says it about µTP and nothing else — for a build that wants the pure
implementation here without making that choice for everything it depends on, or that wants to keep
cgo out of µTP while leaving it on elsewhere.

`utp.NewSocket` and `utp.NewSocketFromPacketConn` use `Default` and have the same signatures as the
constructors of those names in both underlying packages, so moving a caller onto this package is an
import change.

`utp.Socket` is an interface, so code can be handed one without caring which is underneath, and so
is `utp.Implementation` — `utp.PureGo` and `utp.LibUtp` are values of it, and `utp.Default` is
whichever the build picked. `utp.PureGo` is always available; `utp.LibUtp` only exists where libutp
is being compiled. That's how the interop tests run each implementation against the other, and each
against itself.

An `Implementation` has one method: it makes a `Socket` over a `net.PacketConn`. Opening a port
isn't its job, so `utp.Listen` does that for it, and code holding a PacketConn already can hand it
straight over:

```go
s, err := utp.Listen(utp.PureGo, "udp", ":4242")
s, err := utp.PureGo.NewSocket(pc)
```

## Sharing a port with non-µTP traffic

Packets that aren't µTP are not dropped: `Socket` implements `net.PacketConn`, and its `ReadFrom`
and `WriteTo` carry exactly those packets. That's how a single UDP port can serve µTP connections
and something else — a DHT, say — at the same time. Packets that aren't read promptly are dropped.

## Options

The tunables both implementations share are options on the constructors: `utp.WithLogger`,
`utp.WithBufferSizes` and `utp.WithTargetDelay`, plus `Socket.SetLogging` for the protocol's own
log categories and `Socket.SetFirewallCallback` for rejecting incoming connections before they're
acknowledged. Anything an implementation offers beyond that stays in its own package, and both
sockets can be asserted for.

Two differences to know about. `utp.Listen` listens with `net.ListenPacket`, so it takes UDP
networks only — the cgo package's own `NewSocket` also understands the in-process test network, and
through this package you'd reach that by opening it yourself and calling
`Implementation.NewSocket`. And deadlines on the `Socket` itself, which only affect `ReadFrom` and
`WriteTo`, aren't supported by libutp: it returns an error wrapping `errors.ErrUnsupported`, where
the pure implementation honours them. Deadlines on connections work in both.

## The pure Go implementation

[`purego`](purego) can be used on its own, without this package, if the choice at build time isn't
wanted:

```go
import "github.com/anacrolix/go-utp/purego"

s, err := purego.NewSocket("udp", ":4242")
```

The state machine, the LEDBAT congestion controller, selective acknowledgements and fast resend,
the retransmission timers and the MTU search all follow the reference implementation, down to the
constants they're tuned with. It passes `golang.org/x/net/nettest`'s `TestConn` conformance suite,
and the [interop](interop) package tests it against libutp itself: in both directions, in both
roles, and over a link that drops, delays and duplicates packets.

Two of libutp's inputs aren't available portably from Go. ICMP fragmentation-needed reports aren't
fed back in, and the don't-fragment bit isn't set on MTU probes, so path MTU is inferred from
timeouts and duplicate acks rather than reported outright. Neither affects correctness; the search
just tends to settle high. The cgo package doesn't act on ICMP either.

## Development

The [justfile](justfile) mirrors the CI jobs, so what you run locally is what CI runs:

```sh
just test             # go test -race -count 2 ./...
just bench            # build/smoke the benchmarks
just test-purego      # the pure Go implementation selected via the build tag
just test-purego-utp  # the same, via the µTP-only tag
just test-nocgo       # everything with cgo off entirely
```

## Release history

See [CHANGELOG.md](CHANGELOG.md).

## License

MIT, inherited from libutp. See [LICENSE](LICENSE).

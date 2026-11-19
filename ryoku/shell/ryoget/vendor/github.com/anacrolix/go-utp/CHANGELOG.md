# Changelog

## Unreleased

- Split out of [go-libutp](https://github.com/anacrolix/go-libutp), where these packages were added
  but never released. The `utp` package is now this module's root, so the import path is
  `github.com/anacrolix/go-utp` rather than `github.com/anacrolix/go-libutp/utp`, and the pure Go
  implementation is `github.com/anacrolix/go-utp/purego`. go-libutp keeps the cgo wrapper around
  the vendored C++ sources, and this module depends on it for `utp.LibUtp`
- `utp` is one interface over both implementations, selected at build time. `utp.Default` is
  libutp; the `purego` build tag or `CGO_ENABLED=0` makes it `utp.PureGo` and leaves the C++
  sources uncompiled. `utp.Socket` and `utp.Implementation` are both interfaces, so code can be
  handed either implementation as a value: `utp.PureGo` is always available, `utp.LibUtp` wherever
  libutp is compiled. An `Implementation` makes a `Socket` over a `net.PacketConn`, and
  `utp.Listen` opens a port for one. `utp.NewSocket` and `utp.NewSocketFromPacketConn` use
  `utp.Default` and match the signatures of the constructors of those names in both underlying
  packages. The shared tunables are `WithLogger`, `WithBufferSizes`, `WithTargetDelay` and
  `Socket.SetLogging`
- `purego` is µTP in pure Go, ported from the libutp sources. It needs no cgo and no C++ compiler,
  and offers the same API shape as the wrapper: `Socket` implements `net.Listener` and
  `net.PacketConn`, and its connections implement `net.Conn`. The state machine, LEDBAT congestion
  control, selective acknowledgements and fast resend, retransmission timers and MTU search follow
  libutp closely, including its constants. It imports nothing outside the standard library, logs
  through `log/slog`, and passes `nettest.TestConn`
- The `purego_utp` build tag selects the pure Go implementation too, for builds that want to say
  that about µTP alone rather than setting the widely answered `purego` tag for everything they
  depend on, or that want cgo out of µTP while leaving it on elsewhere. `CGO_ENABLED=0` and either
  tag are equivalent here
- `interop` tests the two implementations against each other on the wire: transfers in both
  directions, bidirectional transfers, ping-pong, and transfers over a link that drops, delays and
  duplicates packets. Every case runs over each ordered pair of implementations, so each is also
  tested against itself, with libutp against libutp as the control
- `FirewallCallback` is an alias for `func(net.Addr) bool` in both `utp` and `purego`, so a
  `*purego.Socket` satisfies `utp.Socket` outright and is handed back as itself: a caller can
  assert for it to reach the implementation's own API

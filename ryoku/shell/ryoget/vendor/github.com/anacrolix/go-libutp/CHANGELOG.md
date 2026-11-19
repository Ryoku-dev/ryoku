# Changelog

## Unreleased

- **Breaking**: log through `log/slog`. `Logger` and the `WithLogger` option now take a
  `*slog.Logger` instead of an `anacrolix/log.Logger`. The package-level `Logger` starts out nil,
  which means a Socket created without `WithLogger` logs to `slog.Default()` as it stands when the
  Socket is created. Records carry attributes rather than formatted values, and go out at a level
  that suits them: read errors at warn, giving up on a socket at error, and everything else at
  debug. The categories `Socket.SetLogging` turns on are logged at debug, with a `category`
  attribute, and keep their libutp-shaped message text
- The pure Go implementation and the package that chooses between the two at build time now live
  in [go-utp](https://github.com/anacrolix/go-utp), which depends on this module for the cgo
  wrapper. They were added here after v1.5.1 and never released, so there is nothing to migrate
  unless you were tracking master: `github.com/anacrolix/go-libutp/utp` is now
  `github.com/anacrolix/go-utp`, and `github.com/anacrolix/go-libutp/purego` is now
  `github.com/anacrolix/go-utp/purego`. This module is unchanged as a libutp wrapper
- CI: the `purego` job moved with them, along with `just test-purego` and `just test-nocgo`

## v1.5.1 — 2026-08-17

- Clamp the peer-supplied uTP delay sample before it reaches the average-delay
  accumulator. A crafted `reply_micro` exactly half the 32-bit space from the
  baseline previously produced a sample of `-2^31` (or `+2^31-1` on the other
  branch) that tripped the `current_delay_sum` assertion and aborted the whole
  process. Reported against Erigon (erigontech/security#78); the same
  arithmetic is present in upstream bittorrent/libutp.
- Repair the MTU floor/ceiling range instead of asserting on it. A dropped MTU
  probe in steady state, or an interface/ICMP-reported MTU below the search
  floor, could leave `mtu_floor > mtu_ceiling` and abort on
  `assert(mtu_floor <= mtu_ceiling)` (bittorrent/libutp #105, #130). Both fixes
  mirror the corresponding handling in libtorrent's uTP implementation.
- Add a `justfile` mirroring the CI jobs, so `just test`, `just bench` and `just asan` run
  locally exactly what CI runs
- CI: drive the test, benchmark and asan jobs through the justfile
- asan: leak checks are now clean on macOS too. Tests under the `lsan` build tag also build with
  `netgo` to skip the C resolver, and `lsan_suppressions.txt` suppresses the remaining
  libdispatch/XPC allocations that macOS never frees. Both are inert on Linux.

## v1.5.0 — 2026-07-23

- Add `NewSocketFromPacketConn`, for wrapping an existing `net.PacketConn` (#34)
- Fix writes hanging on freshly accepted connections. An accepted socket is handed to the accept
  callback while still in `CS_SYN_RECV` and only becomes writable once the initiator's first data
  packet arrives, but nothing signalled `UTP_STATE_WRITABLE`, so a `Write` on the accepted `Conn`
  blocked until the `Conn` was destroyed
- CI: stop skipping the nettest read timeout test (fixed by the above)
- CI: quote the benchmark `-run` pattern so Windows works
- Add `CHANGELOG.md` and link to it from the README

## v1.4.0 — 2025-11-21

- Fix unsynchronized event checks and signalling in `waitForConnect`
- Check for port overflow in inproc packetconn
- Make `utp_types.h` safer and compatible with cgo/Go builds (include `<stdint.h>`/`<stdbool.h>`, avoid retypedefs, normalize macros)
- Upgrade `golang.org/x/net` for latest nettest compatibility
- Reduce CI flakiness: skip known-bad test, run nettests sequentially

## v1.3.2 — 2024-09-10

- Upgrade to mmsg v1.0.1
- Fix: `bool`, `true`, and `false` are keywords in modern C
- Run `gorond`
- Bump `golang.org/x/net` from 0.7.0 to 0.17.0

## v1.3.1 — 2023-07-19

- Fix build errors in Go 1.21
- Switch CI to GitHub Actions

## v1.3.0 — 2023-03-04

- Bump `golang.org/x/net` from 0.0.0-20180524181706 to 0.7.0
- Move `ucat` command to `cmd/` directory
- Lower send callback error logging to debug level

## v1.2.0 — 2022-01-31

- Add optional function pattern for `NewSocket`
- Configure logging on a per-socket basis

## v1.1.0 — 2021-11-26

- Add synchronous firewall callback
- Run tests with leak sanitizer

## v1.0.x

### v1.0.5 — 2021-11-26

- Fix memory leak when connect fails to resolve
- Fix race in `sendtoCallback`; add race test
- Reduce `net.UDPAddr` allocations in `sendtoCallback`
- Reduce address allocations in `Socket.utpProcessUdp`
- Minimize time spent holding lock in `NewSocket`
- Run `utp_issue_deferred_acks` and `utp_check_timeouts` using timers
- Merge duplicate timeout checkers
- Tidy up error types
- CI: check for races in benchmarks

### v1.0.0 — 2019-04-26

- Initial release

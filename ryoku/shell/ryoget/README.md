# ryoget

Ryoku's download engine. `ryoget serve` is the on-demand user daemon that
runs every download; everything else `ryoget` does is a command line client
of that daemon.

Build with `CGO_ENABLED=0 go build -tags nosqlite,purego_utp`.

## Files

| What | Where |
|---|---|
| Socket | `$XDG_RUNTIME_DIR/ryoget.sock` (else `/tmp/ryoget-<uid>.sock`), mode 0600 |
| Lock | beside the socket, `ryoget.lock`; a second daemon exits |
| Settings | `$XDG_CONFIG_HOME/ryoku/downloads.json`, written only by the daemon |
| Tasks | `$XDG_STATE_HOME/ryoku/ryoget/tasks.json` |
| Torrent state | `$XDG_STATE_HOME/ryoku/ryoget/torrents/` |
| Log of a spawned daemon | `$XDG_STATE_HOME/ryoku/ryoget/ryoget.log` |
| Thumbnails | `$XDG_CACHE_HOME/ryoku/ryoget/thumbs/` |
| Unfinished file | `<dir>/<name>.part` with its resume sidecar `<name>.part.ryoget` |
| Unfinished media | `<dir>/.ryoget-<task id>/` |

## Lifetime

`ryoget ensure` dials the socket; when nothing answers it runs
`ryoku-host svc --user start ryoget`, and where no unit answers it starts a detached
`ryoget serve`. Exit 0 ready, 1 failed, 3 nothing to start.

The daemon exits after 60 s (`RYOGET_IDLE_SECONDS`) with no client connected, nothing
queued, running, merging or seeding, and no notification waiting for a press. The
torrent client exists only while a torrent runs or seeds.

## Socket protocol

One compact JSON object per line, at most 1 MiB; only the daemon's own user may connect.

Requests `{"id":N,"method":"...","params":{...}}` are answered by
`{"id":N,"result":...}` or `{"id":N,"error":{"code":"...","message":"..."}}`.
Error codes: `unsupported unavailable private rate_limited network invalid busy internal not_found`.

| method | params | result |
|---|---|---|
| `hello` | `{client, version:1}` | `{version, settings, tasks, stats}`; the connection then receives events |
| `resolve` | `{url}` | `Resolved` |
| `add` | `{url, mode?:"auto"\|"audio"\|"mute", format?, dir?, name?, files?:[int], title?, paused?}`; `title` is shown until the task's own resolution names it | `Task` |
| `addMany` | `{items:[{url, mode?, format?, files?, title?}], dir?, paused?}`, at most 500 | `{tasks:[Task], errors:[{index, code, message}]}`; each item on its own, `index` into `items`, both lists always present |
| `links` | `{text}`: any pasted text, no network | `{links:[{raw, url, kind:"url"\|"magnet"\|"torrent", host, notes, duplicate}], total, cleaned, duplicates}` (see below) |
| `pause` / `resume` | `{id}` or `{all:true}` | `{}` |
| `cancel` | `{id}`: stop and delete partial data | `{}` |
| `remove` | `{id, deleteFile?}` | `{}` |
| `retry` | `{id}` | `Task` |
| `move` | `{id, index}`: position among queued tasks | `{}` |
| `files` | `{id, selected:[int]}`: torrent file selection | `Task` |
| `open` / `reveal` | `{id}` | `{}` |
| `clear` | `{states?}` default `["done","error","stopped"]`; files stay | `{}` |
| `history` | `{id}` | `{down:[...], up:[...]}`: the task's B/s, one sample per second, oldest first, up to 60; kept in memory only while it runs or seeds, empty otherwise |
| `settings.get` / `settings.set` | `{}` / `{patch:{...}}` | `Settings` |

Events `{"event":"...","data":...}`: `task` (whole Task, at most 5 per second per task),
`removed` `{id}`, `stats` (every second while anything runs, once when it stops),
`settings`.

The shapes of `Task`, `Stats`, `Resolved` and `Settings` are the Go types in
`internal/proto`. `Task.state` is one of
`queued resolving active paused merging seeding done error stopped`; `Task.stage` is
`fetch metadata merge remux seed` or empty. `Task.pieces` is base64 of 64 bytes, each the
fill (0 to 255) of one 64th of the file, the torrent's pieces or the stream's segments.

`links` finds plain links, Markdown `[text](url)` and `<url>` links, magnets, and bare `www.`
or `host.tld/path` addresses, in text order. Each is cleaned: trailing punctuation and
unbalanced brackets trimmed, `https://` added, redirectors unwrapped (Google, Facebook,
Instagram, YouTube, Reddit, Outlook Safe Links, DuckDuckGo, Steam, Tumblr, VK, LinkedIn),
tracking parameters removed (`utm_*`, `fbclid`, `gclid`, `si` on YouTube and Spotify, ...),
known sites moved to their usual address (`youtu.be` to the watch link, mobile and mirror
hosts to the main one, Twitter to `x.com`), and a fragment dropped unless the address needs
it. Signed links (`X-Amz-*`, `token`, `sig`, `Signature`, `Expires`, ...) and magnets stay
untouched. `notes` says what changed: `https-added trimmed unwrapped tracking-removed
canonical`. Every link found is listed; a later one equal to an earlier one after cleaning
has `duplicate:true`. `total` counts all of them, `cleaned` those with notes, `duplicates`
the marked ones.

## Command line

```
ryoget add <url>... | - [--audio|--mute] [--dir DIR] [--paused]
ryoget list [--json]
ryoget pause|resume|cancel|remove|retry <id|all>
ryoget resolve <url> [--json]
ryoget ensure
ryoget serve
ryoget version
```

A URL is an http(s) link, a magnet, or a local `.torrent` file. With `-`, every link
found in standard input is cleaned and added once (as `links` would list it).

## HTTP downloads

A file is probed with HEAD and a one-byte ranged GET, split into
min(connections, size / minimum split) ranges written into one sparse `.part`, and a
connection that finishes early takes half of the largest range still running. The
sidecar is rewritten every second and on stop; a resume checks the size and ETag or
Last-Modified, sends `If-Range`, and starts over cleanly when the file changed.
Servers without ranges get one stream. Each range retries five times with back-off;
a server that refuses extra connections with 429 or 503 is downloaded with fewer.
The finished file never replaces an existing one: it lands as `name (2).ext`.

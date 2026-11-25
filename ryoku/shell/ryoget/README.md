# ryoget

Ryoku's download engine and search daemon. Two binaries from one module:

- `ryoget serve` is the on-demand user daemon behind Kit's Downloads and the
  Hyakki bar's quick downloads. Everything else `ryoget` does is a command line
  client of that daemon.
- `ryoseek serve` is the on-demand search daemon behind the Hyakki bar's Web
  view (see [ryoseek](#ryoseek)). It never links the download engine.

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
Error codes: `unsupported unavailable private rate_limited network invalid busy internal not_found
no_space browser_check checksum`.

| method | params | result |
|---|---|---|
| `hello` | `{client, version:1}` | `{version, settings, tasks, stats}`; the connection then receives events |
| `resolve` | `{url}` | `Resolved`; `existing:{id, state, path?}` names the newest download already in the list for that link (`path` only when it finished and the file is still there) |
| `add` | `{url, mode?:"auto"\|"audio"\|"mute", format?, dir?, name?, files?:[int], title?, paused?, headers?:{...}, checksum?}`; `title` is shown until the task's own resolution names it. `headers` keeps only `Cookie`, `Referer`, `User-Agent`, `Authorization` and `Origin`; a link added with them is fetched as one plain file with them, and they are forgotten once it finishes. `checksum` is `"sha256:<hex>"`, `"sha1:<hex>"` or `"md5:<hex>"` (a `#sha256=<hex>` fragment on the link says the same and is stripped); a finished file that matches gets `Task.verified`, one that does not is deleted and fails with `checksum`. Torrents ignore both, media ignores the checksum | `Task` |
| `addMany` | `{items:[{url, mode?, format?, files?, title?}], dir?, paused?}`, at most 500 | `{tasks:[Task], errors:[{index, code, message}]}`; each item on its own, `index` into `items`, both lists always present |
| `links` | `{text}`: any pasted text, no network | `{links:[{raw, url, kind:"url"\|"magnet"\|"torrent", host, notes, duplicate}], total, cleaned, duplicates}` (see below) |
| `pause` / `resume` | `{id}` or `{all:true}` | `{}` |
| `cancel` | `{id}`: stop and delete partial data | `{}` |
| `remove` | `{id, deleteFile?}` | `{}` |
| `retry` | `{id}` | `Task` |
| `limit` | `{id, kib}`: this web download's own speed limit in KiB/s, `0` for none, on top of `downLimit` (a torrent is refused, torrents follow the global limits); a running one changes at once, except a yt-dlp fetch, which takes it from its next start | `Task` |
| `refresh` | `{id, url}`: a new link for a failed, stopped, paused or queued web download (a running one must be paused first); it resolves again and keeps its partial data and folder when the file is the same; headers it was added with go along only to the same host or its subdomains | `Task` |
| `move` | `{id, index}`: position among queued tasks | `{}` |
| `files` | `{id, selected:[int]}`: torrent file selection | `Task` |
| `open` / `reveal` | `{id}` | `{}` |
| `clear` | `{states?}` default `["done","error","stopped"]`; files stay, a failed download's partial data goes | `{}` |
| `seen` | `{ids:[...], at?}`: stamps finished, failed, stopped and paused tasks with when a home list first showed them (`at`, RFC 3339, default now); an earlier stamp wins, so a client may backdate one to drop it from its list at once | `{}` |
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
`Task.finished` is when it last stopped for good (finished, failed or stopped) and is
cleared when it runs again, as is `Task.seen`.

Settled tasks (finished, failed or stopped) are forgotten `keepDays` days after they
stopped (`0` keeps them until cleared): when the daemon starts, after a task settles,
on `hello` and when the setting changes, so nothing wakes the daemon for it. A finished
file stays on disk; a failed web download's partial data goes with its entry, and a
torrent's data always stays.

With `sortByType` on, a web download added without `dir` lands in a folder per kind inside
`dir`: a file by the extension of the link it is served from (`Videos`, `Music`, `Documents`, `Programs`,
`Archives`, `Pictures`, in the reader's language; anything else stays in `dir`), a video in
`Videos` and an audio download in `Music`. Torrents stay in `dir`. A download that already
holds data keeps its folder.

`links` finds plain links, Markdown `[text](url)` and `<url>` links, magnets, and bare `www.`
or `host.tld/path` addresses, in text order. Each is cleaned: trailing punctuation and
unbalanced brackets trimmed, `https://` added, redirectors unwrapped (Google, Facebook,
Instagram, YouTube, Reddit, Outlook Safe Links, DuckDuckGo, Steam, Tumblr, VK, LinkedIn),
tracking parameters removed (`utm_*`, `fbclid`, `gclid`, `si` on YouTube and Spotify, ...),
known sites moved to their usual address (`youtu.be` to the watch link, mobile and mirror
hosts to the main one, Twitter to `x.com`), and a fragment dropped unless the address needs
it. Signed links (`X-Amz-*`, `token`, `sig`, `Signature`, `Expires`, ...) and magnets stay
untouched. Ranges expand first: `file[001-100].zip` gives `file001.zip` to `file100.zip`
(padding kept), `[1-10:2]` steps, `[10-1]` counts down, `[a-z]` and `[A-F]` run letters,
and several ranges give every combination, leftmost slowest; one paste grows to at most
500 links, and brackets that hold no range (`[::1]`) stay as they are. Each expanded link
keeps the pattern as `raw`. `notes` says what changed: `https-added trimmed unwrapped
tracking-removed canonical expanded`. Every link found is listed; a later one equal to an
earlier one after cleaning has `duplicate:true`. `total` counts all of them, `cleaned`
those with notes other than `expanded`, `duplicates` the marked ones.

## Command line

```
ryoget add <url>... | - [--audio|--mute] [--dir DIR] [--paused]
ryoget list [--json]
ryoget pause|resume|cancel|remove|retry <id|all>
ryoget clear [--done] [--failed]
ryoget resolve <url> [--json]
ryoget ensure
ryoget serve
ryoget version
```

A URL is an http(s) link, a magnet, or a local `.torrent` file. With `-`, every link
found in standard input is cleaned and added once (as `links` would list it).

## Sites

Native resolvers read public, keyless endpoints: YouTube, Vimeo, Twitch, X, Instagram,
TikTok, Facebook, Reddit (`v.redd.it` links), Dailymotion, SoundCloud, Bilibili,
Streamable, VK, OK.ru, Bluesky, Pinterest, Tumblr, Loom, Rutube, Bandcamp (a track, or an
album listing its tracks), TED, Odysee, Rumble and LinkedIn (public posts; feeds,
profiles and company pages are refused). Reddit post pages answer anonymous clients with
a script check, so they are left to yt-dlp. Any other link goes to an installed yt-dlp
when `ytdlpFallback` is on, then to the generic probe: an HLS playlist, a page's
`og:video` or `og:audio`, or a plain file. yt-dlp gets `--js-runtimes <deno|node|bun>:<path>`
when it knows that option (asked once per daemon run) and one of them is installed.

A page with no media lists the files it links to (archives, disk images, installers,
documents, ROMs, audio and video files, or any link with a `download` attribute) as
items of kind `file`, read against the address the page was finally served from or its
`<base href>`; adding it queues one task per file, each resolved against its own host
with the page as `Referer`, and a page with just one such link resolves straight to that
file. A file on another host is sent only the page's origin as `Referer`, and one an
https page links over plain http none. A download form with fixed fields is sent the way
a browser would, leaving out disabled controls; one behind a captcha, filled in by
script, asking for typed input, posting in another encoding than
`application/x-www-form-urlencoded`, answering with another page or with the file as
the post's own answer fails with `browser_check`, as does a Cloudflare or DDoS-Guard
check page (the `cf-mitigated: challenge` header or a challenge page under 256 KiB)
where a page or file was expected.

With `coverArt` on, an audio-only download carries its thumbnail as the cover: a picture
stream in mp3, m4a and flac, a `METADATA_BLOCK_PICTURE` comment in opus and ogg. A
thumbnail that cannot be fetched or read leaves the file without one.

## HTTP downloads

The first request is `GET Range: bytes=0-<4 MiB>`: its answer names the file
(Content-Disposition, then the URL, then the type) and its body is already the first
connection's data, so there is no HEAD and no one-byte probe. A server that answers 200
gets one plain stream. Later requests go to the address the redirects ended at, and back
to the original link once when that address answers 403, 404 or 410.

Four connections open first, the rest once those deliver bytes. Each asks for about two
seconds of data at its own speed (4 to 16 MiB) and keeps its socket between requests. A
connection that runs out of work takes half of the largest range in flight, split on a
4 KiB boundary behind that range's current request so the other socket survives. When
nothing is left to split, a range over 3 s from done and over 4x slower than a typical
connection is fetched a second time from its frontier, and whichever copy finishes first
wins.

A connection retries 408, 425, 429, 5xx and network errors with a back-off of 0.5 s
doubling to 30 s with jitter, or the server's `Retry-After` (at most 300 s). Ten failures
in a row without a byte and it gives up: the others carry on, and after 10 s new
connections take over its range. The download fails only when such a round brings
nothing. A server that turns extra connections away (429, 503, 403, or resets before
the first byte) gets as many as were working, one more is tried after 20 s, then after
twice as long up to 300 s, and never sooner than its `Retry-After`. That limit is kept
per host for a day in `$XDG_STATE_HOME/ryoku/ryoget/hosts.json`, read when a download
starts and written only when it changes.

Before a byte is written the free space is checked (`no_space` names what is needed and
what is free) and the whole `.part` is reserved with fallocate. The sidecar is written
every 5 s and on stop, each time after the `.part` is synced. A resume needs the sidecar's
ranges to cover the file exactly and the `.part` to have the file's size; it trusts the
size and the ETag or Last-Modified (sent as `If-Range`), not the link, so a refreshed
address keeps the partial data. Without either validator it fetches the 64 KiB before the
longest range's end and compares them with the disk. Anything that does not match starts
over cleanly. The finished file never replaces an existing one: it lands as
`name (2).ext`.

## ryoseek

The search daemon behind the Hyakki bar's Web view: one per user, started when the bar
first searches. It owns every engine's request budget and block timer, so they outlive
the bar. It imports `internal/search` and never the download engine.

Build with `CGO_ENABLED=0 go build -trimpath -tags nosqlite,purego_utp ./cmd/ryoseek`.

### Files

| What | Where |
|---|---|
| Socket | `$XDG_RUNTIME_DIR/ryoseek.sock` (else `/tmp/ryoseek-<uid>.sock`), mode 0600 |
| Lock | beside the socket, `ryoseek.lock`; a second daemon exits |
| Engine health and budgets | `$XDG_STATE_HOME/ryoku/ryoseek/health.json`, written on change |
| Cache (results, icons, thumbnails) | `$XDG_CACHE_HOME/ryoku/ryoseek/` |
| Log of a spawned daemon | `$XDG_STATE_HOME/ryoku/ryoseek/ryoseek.log`; under a unit, the journal |

### Lifetime

`ryoseek ensure` dials the socket; when nothing answers it runs
`ryoku-host svc --user start ryoseek`, and where no unit answers it starts a detached
`ryoseek serve` (its own session) and polls the socket for up to 3 s. Exit 0 ready,
1 failed, 3 nothing to start.

Every client shares one search service. The daemon exits after 5 minutes
(`RYOSEEK_IDLE_SECONDS`) with no client connected and no request on the wire; SIGTERM
stops it cleanly, letting requests already sent finish into the cache and flushing the
health file.

### Command line

```
ryoseek serve
ryoseek ensure
ryoseek query <text> [--scope S] [--mode suggest|full] [--json]
ryoseek version
```

`query` starts the daemon if needed, asks once (scope `all` and mode `full` by default)
and prints each source's rows by rank; `--json` prints every protocol line instead.

### Socket protocol (v2)

One compact JSON object per line, at most 1 MiB; only the daemon's own user may connect.

Client to daemon:

- `{"hello":{"client":"bar"|"hyakki"|"cli","version":2,"lang":"en-US"}}` first. `lang` is the
  full locale; it picks Wikipedia's and the dictionary's language and the Accept-Language
  every engine sees. A query sent before hello runs with `en-US`.
- `{"id":N,"q":"...","scope":"all"|<scope>|<sourceId>,"mode":"suggest"|"full"}`. A newer
  query cancels everything older from the same client; an empty `q` answers only the final
  done.
- `{"id":N,"detail":{"source":"...","key":"..."}}`: `key` is the row's `key`. A newer detail
  cancels older details, not the running query.
- `{"cancel":N}` stops the lines for N. Requests already sent still finish into the cache;
  a request still held by its budget is dropped once no client wants it.

Daemon to client:

- After hello: `{"ready":true,"version":2,"sources":[Source],"engines":[Engine]}`, no network.
  `Source = {id, name, scope, glyph, typing}`; `typing` sources answer in suggest mode. The
  order is the section order in `all` when ranks tie.
- Per source, only for the sources that run in the query's mode and scope (the dictionary
  runs only for a lookup):
  - `{"id":N,"source":S,"state":"loading"}` first,
  - `{"id":N,"source":S,"state":"waiting"}` while its budget holds the request (it will run),
  - then one of `{"id":N,"source":S,"state":"done","items":[Item],"more":bool,"rank":int}`,
    `{"id":N,"source":S,"state":"paused","until":"RFC3339"}` (every engine behind the source is
    blocked; `until` is the soonest resume) or `{"id":N,"source":S,"state":"error","error":"text"}`.
  - In `all`, rows that do not match the query well enough are left out (`more:true` then
    says the source's own view has rows); a scope or a single source gets the whole list.
    Sections order by `rank`, higher first: dictionary 7, web and suggestions 6, others 1 to
    5 from relevance, empty 0.
- `{"id":N,"state":"done"}` once every source of the query settled.
- `{"engines":[Engine]}` to every client whenever an engine is paused or resumes.
  `Engine = {id, name, sources:[sourceId], state:"ok"|"paused", until}`. The web lane's
  engines are `keenable`, `ddg-lite` and `bing-rss` (`sources` holds `web`).
- `{"id":N,"detail":{source, key, title, url, text, site, thumb, facts:[{label,value}], error?}}`.

`Item = {key, source, kind, title, url, snippet, site, date, thumb, meta}`; `thumb` is a local
file or empty, `date` RFC3339 or empty. Kinds: `page article repo question video package
thread doc suggestion definition`. A `suggestion` has the completion as `title`, no `url`,
`meta.engine`. A `definition` has the word as `title`, the sense as `snippet`, the Wiktionary
page as `url` and `meta: {partOfSpeech, tags, examples, attribution, attributionUrl}`
(the attribution must stay visible: CC BY-SA 4.0). A web `page` has `meta.engines`, the
engines that found it, in the order `keenable ddg-lite bing-rss`.

Modes: `suggest` (while typing) runs suggestions (DuckDuckGo autocomplete, Bing's when
DuckDuckGo fails or is paused), Wikipedia title search and the dictionary card
(FreeDictionaryAPI.com, Wiktionary as fallback) for one or two words or "what does X mean",
"X meaning", "define X"; never a web engine. `full` (on Enter, on picking a suggestion, or
after 1.2 s of stillness with 3+ characters) runs every source in scope; its `web` source
asks Keenable and DuckDuckGo lite in parallel and merges them by reciprocal rank fusion,
with Bing RSS only when both failed or are paused.

Every engine has a request budget and a health record in the state file: a block (202 or
DuckDuckGo's challenge, captcha, Anubis or JavaScript interstitial pages, 403, 429, 503)
pauses it for its Retry-After, else 15 min doubling per strike up to 6 h, and nothing is sent
to a paused engine. Results stay 30 min (memory and disk), suggestions 10 min, empty
answers 5 min. Every request says `User-Agent: ryoseek/<version> (+https://ryoku.dev)
Go-http-client/2.0`.

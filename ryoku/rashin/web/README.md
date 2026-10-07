# ryoku/rashin/web

The Rashin console: one Svelte app that is both the companion window
(`rashin-app`, a WebKitGTK window pointed at it) and the loopback dashboard
(`http://127.0.0.1:3600`). It is built here and the build output is committed
under `../backend/web/dist`, which `ryoku-rashin` embeds; `go build` alone
ships it and no node runs on an installed box.

## Stack

- Svelte 5 (runes), TypeScript strict, Vite. Plain CSS on design tokens; no
  utility framework.
- [bits-ui](https://www.bits-ui.com) for every headless control: tooltips,
  selects, dialogs, popovers, tabs, switches, toggle groups, collapsibles.
  Wrapped once under `src/lib/ui/`; a sheet never imports `bits-ui` directly.
- [Libraries.dev](https://github.com/Jakubantalik/Libraries.dev) for the
  effects that make the console feel alive, under `src/lib/fx/`:
  `ThinkingOrb` (the `thinking-orbs/engine` painters on a plain canvas) and
  `BorderBeam` (the package's React component, mounted in a Svelte-owned
  element through `island.ts`; that file is the whole React seam).
- [blobatar](https://github.com/Alain00/blobatar) for the Needle's face
  (`NeedleFace`): a creature seeded by the conversation id, hue-locked to the
  palette's warm family, with an expression per agent state, blobatar's idle
  motion, and eyes that follow the pointer through its `gaze` attachment.
- The chat reducer is the shell's (`ryoku/shell/quickshell/shell/services/lib/
  chatstate.js`), aliased as `$chatstate`. The Ryoku and Chat stores each fold
  their own `/ws/chat?lane=` stream through that reducer.

## Map

|Path|Owns|
|---|---|
|`src/app.css`|Tokens from `docs/ui-ux.md` (paper, ink ramp, bone, type roles, spacing, motion), fonts, resets, the `.t-*` text roles.|
|`src/lib/ui/kit.css`|Shared markup idioms: `table.data`, `.prose`, `.plates`/`.plate`, `.grid`, `.kpi`, `.bar`.|
|`src/lib/ui/`|The primitives kit on bits-ui: `Button`, `IconButton`, `Tooltip`, `Seg`, `Switch`, `Select`, `Dialog`, `Popover`, `Tabs`, `Fold`, `Chip`, `Card`, `Field`, `Empty`, `Lamp`, `Kbd`, `Icon` (+ `icons.ts`).|
|`src/lib/fx/`|The Libraries.dev wrappers.|
|`src/lib/app/`|The frame: `TopBar` (brand, sheets, combined lane presence), `Page`, `Offline`, and `router.svelte.ts` (hash routes and sheet order).|
|`src/lib/api/`|`client.ts` owns Rashin daemon routes, `prowl.ts` is the only client for proxied Prowl gateway routes, and `socket.ts` owns reconnecting JSON WebSockets.|
|`src/lib/state/`|`theme.svelte.ts` (wears the wallpaper palette), `machine.svelte.ts` (status + vitals stream).|
|`src/lib/chat/`|`protocol.ts` (wire shapes), `store.svelte.ts` (the separate Ryoku and Chat lane connections).|
|`src/lib/content/`|`markdown.ts` (escape-first renderer, tested) and `Markdown.svelte`.|
|`src/lib/pages/chat/ChatWorkspace.svelte`|The shared sessions, transcript, approval, composer and inspector workspace used by both conversation sheets.|
|`src/lib/pages/wiki/`|Wiki-specific helpers and reader parts; `WikiSheet.svelte` owns the route and lazy document loading.|
|`src/lib/pages/prowl/`|The Prowl section: gateway overview, providers, routing, activity, project indexes, harness setup and command toolkit behind `ProwlSheet.svelte`.|
|`src/pages/<Name>Sheet.svelte`|One sheet per route. Its parts live under `src/lib/pages/<name>/`.|

## Conversation lanes

- **Ryoku** (`#/ryoku`, `/ws/chat?lane=ryoku`) is the machine lane. Its agent
  can use the vault, skills, code index and approval flow. The composer can
  send a full agent turn or a Quick request through `POST /api/ask?q=`.
- **Chat** (`#/chat`, `/ws/chat?lane=chat`) is the plain harness lane. It has
  its own transcript, sessions, draft and pane preferences, and deliberately
  receives no machine context. Both lanes show the daemon-wide approval mode.
- **Wiki** (`#/wiki/<page>`) lists `wiki/*.md` from `GET /api/vault` and reads
  a page through `GET /api/vault/file?p=wiki/<page>.md`. `#/wiki` opens
  `wiki/README.md`.

## Rules a sheet follows

- Reads `docs/ui-ux.md` first. Bone on black, emphasis by inversion (a bone
  plate, never a colour), hairlines not shadows, one spacing scale, a hair of
  rounding, Latin named and kanji sealed. Colour only where it is data; the
  red is the seal and the alert.
- Uses the kit. A control that is not in `src/lib/ui/` is added there once,
  not styled ad hoc in a sheet. Shared files (`app.css`, `kit.css`, the kit,
  the stores, the router) change through the integrator, not from a sheet.
- The daemon is the authority. A sheet renders what the API or the stream
  says; it may render optimistically what the user just did and nothing
  more. Rashin-owned mutations go through `api.*`; Prowl operations go
  through `prowl.*` in `src/lib/api/prowl.ts`, never raw `fetch`.
  The Prowl proxy requires `X-Rashin-Client: console` on every request.
- Motion answers an action: a reveal, a fold, a confirm. Nothing loops at
  rest except what reports live state (an orb while working, a lamp while
  busy). `prefers-reduced-motion` is honoured by the tokens already.
- Content anchors to the top and takes the width it is given; only prose
  keeps a measure (`--measure`). Empty is an invitation, errors say what to
  do, copy is sentence case and plain.

## Work on it

```sh
pnpm install
RASHIN_DEV_TARGET=http://127.0.0.1:3611 pnpm dev --port 5173   # against a preview daemon
pnpm check        # svelte-check, strict, warnings fail
pnpm test         # vitest
pnpm build        # writes ../backend/web/dist (commit it)
```

A change reaches a running daemon only after `pnpm build`, a Go rebuild and
a restart: the dist is `go:embed`-ed. The preview lane in
`skill://verify-rashin-dashboard-changes` keeps the user's live daemon on
3600 untouched.

## Gates

`pnpm check`, `pnpm test` and a clean `pnpm build` whose output matches the
committed dist (pnpm's own settings, such as the one build script esbuild is
allowed to run, live in `pnpm-workspace.yaml`) (`git diff --exit-code -- ryoku/rashin/backend/web/dist`), run
by `.github/workflows/rashin-web.yml`. The Go side runs its own tests.

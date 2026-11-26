pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io

// The matugen scheme each wallpaper would give the desktop, cached by key and
// mtime. Ported from kwepaperpicker's WallpaperSwatches service: the card
// dressing, the ribbon's source colours and the colour sort all read one set
// of roles. Ryogami already owns matugen, so the colours come from the
// daemon's preview generator instead of a second dry-run script.
QtObject {
    id: palettes

    // key -> { m: mtime, s: bytes, w, h, c: { role: "#rrggbb" } }
    property var entries: ({})
    property int revision: 0
    readonly property bool scanning: palettes._pending > 0

    property int _pending: 0
    property var _specs: ({})

    signal changed

    function baseName(path) {
        if (!path)
            return ""
        const text = String(path)
        const cut = Math.max(text.lastIndexOf("/"), text.lastIndexOf("\\"))
        return cut >= 0 ? text.slice(cut + 1) : text
    }

    function camel(name) {
        return String(name).replace(/_([a-z])/g, function (_, c) {
            return c.toUpperCase()
        })
    }

    function normalize(colors) {
        const out = ({})
        if (!colors)
            return out
        for (const key of Object.keys(colors)) {
            const value = colors[key]
            if (typeof value === "string" && value.length > 0)
                out[palettes.camel(key)] = value
        }
        return out
    }

    function cachePath() {
        const root = Quickshell.env("XDG_CACHE_HOME") || (Quickshell.env("HOME") + "/.cache")
        return root + "/ryoku/deck-palettes.json"
    }

    function entryFor(item) {
        if (!item)
            return null
        const names = [item.key, palettes.baseName(item.path), palettes.baseName(item.fullImage), palettes.baseName(item.thumb)]
        for (const name of names) {
            if (!name)
                continue
            const found = palettes.entries[name]
            if (found && found.c)
                return found
        }
        return null
    }

    function scheme(item) {
        const found = palettes.entryFor(item)
        return found ? found.c : null
    }

    // The colour a wallpaper stands for; a grey for matugen's Google-blue seed.
    function swatch(item) {
        const s = palettes.scheme(item)
        if (!s)
            return ""
        const source = s.sourceColor || s.source_color || ""
        const highest = s.surfaceContainerHighest || s.surfaceContainerHigh || s.surfaceContainer || s.surface || ""
        return source === "#4285f4" ? highest : source
    }

    function hueKey(item) {
        const s = palettes.scheme(item)
        if (!s)
            return 3
        const source = Qt.color(s.sourceColor || s.source_color || "#4285f4")
        if (source.hslSaturation < 0.12 || source.hslHue < 0)
            return 1 + Qt.color(s.surfaceContainerHighest || s.surface || "#1d100e").hslLightness
        return (source.hslHue + 0.035) % 1
    }

    function dressOf(s) {
        if (!s)
            return palettes.fallbackDress()
        function pick(a, b, fallback) {
            return s[a] || s[b] || fallback
        }
        const f = palettes.fallbackDress()
        return {
            surface: pick("surfaceContainer", "surface_container", f.surface),
            raised: pick("surfaceContainerHigh", "surface_container_high", f.raised),
            low: pick("surfaceContainerLow", "surface_container_low", f.low),
            ink: pick("onSurface", "on_surface", f.ink),
            muted: pick("onSurfaceVariant", "on_surface_variant", f.muted),
            line: pick("outlineVariant", "outline_variant", f.line),
            accent: s.primary || f.accent,
            accentInk: pick("onPrimary", "on_primary", f.accentInk)
        }
    }

    function dressFrom(cols) {
        if (!cols || cols.length === 0)
            return palettes.fallbackDress()
        function vividness(q) {
            return q.hslSaturation * (1 - Math.abs(q.hslLightness - 0.5))
        }
        const main = Qt.color(cols[0])
        let vivid = main
        for (const c of cols) {
            const q = Qt.color(c)
            if (vividness(q) > vividness(vivid))
                vivid = q
        }
        const h = main.hslSaturation > 0.08 && main.hslHue >= 0 ? main.hslHue : Math.max(0, vivid.hslHue)
        const s = Math.min(0.32, main.hslSaturation)
        const va = vivid.hslHue >= 0 ? vivid.hslHue : h
        return {
            surface: Qt.hsla(h, s, 0.13, 1),
            raised: Qt.hsla(h, s, 0.19, 1),
            low: Qt.hsla(h, s, 0.1, 1),
            ink: Qt.hsla(h, 0.16, 0.9, 1),
            muted: Qt.hsla(h, 0.12, 0.72, 1),
            line: Qt.hsla(h, Math.min(0.24, s + 0.06), 0.32, 1),
            accent: Qt.hsla(va, Math.min(0.72, vivid.hslSaturation + 0.18), 0.74, 1),
            accentInk: Qt.hsla(va, 0.5, 0.16, 1)
        }
    }

    function fallbackDress() {
        return {
            surface: Theme.surfaceContainer,
            raised: Theme.surfaceVariant,
            low: Theme.background,
            ink: Theme.surfaceText,
            muted: Theme.surfaceVariant,
            line: Theme.outline,
            accent: Theme.primary,
            accentInk: Theme.primaryText
        }
    }

    // Only the cards in hand are asked for. The daemon caches each key by
    // mtime, so sweeping the deck does not re-run matugen on every frame.
    function ensure(items) {
        const missing = []
        for (const item of items || []) {
            if (!item || !item.key)
                continue
            palettes._specs[item.key] = item
            const found = palettes.entryFor(item)
            const mtime = Number(item.mtime || 0)
            if (!found || !found.c || (mtime > 0 && found.m !== mtime))
                missing.push(item.key)
        }
        if (missing.length === 0)
            return
        const size = 12
        for (let at = 0; at < missing.length; at += size) {
            const chunk = missing.slice(at, at + size)
            palettes._pending++
            Daemon.call("palette.previewColors", { keys: chunk }, function (result, error) {
                palettes._pending--
                if (error || !result)
                    return
                palettes._apply(result, chunk)
            })
        }
    }

    function _apply(result, keys) {
        const doc = result.colors || {}
        let changed = false
        const next = Object.assign({}, palettes.entries)
        for (const key of keys) {
            const colors = palettes.normalize(doc[key])
            if (Object.keys(colors).length === 0)
                continue
            const spec = palettes._specs[key] || ({})
            next[key] = {
                m: Number(spec.mtime || 0),
                c: colors
            }
            changed = true
        }
        if (!changed)
            return
        palettes.entries = next
        palettes.revision++
        palettes.changed()
        saveTimer.restart()
    }

    function _parseCache(text) {
        try {
            const doc = JSON.parse(text)
            if (doc && doc.version === 1 && doc.entries) {
                const next = ({})
                for (const key of Object.keys(doc.entries)) {
                    const e = doc.entries[key]
                    next[key] = { m: e.m, c: palettes.normalize(e.c) }
                }
                palettes.entries = next
                palettes.revision++
                palettes.changed()
            }
        } catch (e) {
            // A half-written cache just means the daemon is asked again.
        }
    }

    function _parseSeed(text) {
        let changed = false
        const next = Object.assign({}, palettes.entries)
        for (const line of String(text).split("\n")) {
            if (!line.trim())
                continue
            try {
                const row = JSON.parse(line)
                const name = row.n
                if (!name)
                    continue
                next[name] = { m: row.m || -1, c: palettes.normalize(row.c) }
                changed = true
            } catch (e) {
                // One bad line does not hide the rest.
            }
        }
        if (changed) {
            palettes.entries = next
            palettes.revision++
            palettes.changed()
        }
    }

    property FileView cache: FileView {
        path: palettes.cachePath()
        printErrors: false
        onLoaded: palettes._parseCache(text())
    }

    // Local testing bridge: kwepaperpicker's JSONL palette dump can seed the
    // same cache before the rebuilt daemon is running.
    property FileView seed: FileView {
        path: Quickshell.env("RYOGAMI_DECK_PALETTES") || ""
        printErrors: false
        onLoaded: palettes._parseSeed(text())
    }

    property Timer saveTimer: Timer {
        id: saveTimer
        interval: 1500
        onTriggered: cache.setText(JSON.stringify({ version: 1, entries: palettes.entries }))
    }
}

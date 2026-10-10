import QtQuick
import Ryoku.Ui.Singletons

BarMenu {
    id: dl

    required property PickerState state

    glyph: "\u{f01da}"
    label: I18n.tr("Download")
    tooltip: I18n.tr("Browse wallpaper sources")
    panelWidth: 286
    active: dl.state && dl.state.sheet === "browser"
    alignRight: true
    onOpenChanged: if (dl.open) dl._refresh()

    readonly property var _order: [
        { key: "wallhaven", label: I18n.tr("Wallhaven"), glyph: "\uf03e" },
        { key: "steam", label: I18n.tr("Steam Workshop"), glyph: "\uf1b6" },
        { key: "unsplash", label: I18n.tr("Unsplash"), glyph: "\uf030" },
        { key: "pexels", label: I18n.tr("Pexels"), glyph: "\uf083" },
        { key: "youtube", label: I18n.tr("YouTube"), glyph: "\uf16a" },
        { key: "bing", label: I18n.tr("Bing Daily"), glyph: "\uf002" },
        { key: "moewalls", label: I18n.tr("MoeWalls"), glyph: "\uf008" },
        { key: "motionbgs", label: I18n.tr("MotionBGs"), glyph: "\uf008" },
        { key: "ryostore", label: I18n.tr("Ryostore"), glyph: "\uf290" },
        { key: "repos", label: I18n.tr("Repos"), glyph: "\uf126" }
    ]
    property var _rows: []
    readonly property var _srcs: BrowserSources {}

    function _build(rpc) {
        var byKey = ({})
        if (rpc)
            for (var i = 0; i < rpc.length; ++i) byKey[rpc[i].key] = rpc[i]
        var out = []
        for (var j = 0; j < dl._order.length; ++j) {
            var p = dl._order[j]
            var r = byKey[p.key]
            var av = dl._srcs.availability(p.key, Settings, r)
            out.push({ key: p.key, label: (r && r.label) ? r.label : p.label, glyph: p.glyph,
                       enabled: av.enabled, reason: av.reason, code: av.code || "" })
        }
        return out
    }
    function _refresh() {
        dl._rows = dl._build(null)
        Daemon.call("source.providers", ({}), function (result, error) {
            if (!error && result)
                dl._rows = dl._build(result)
        })
    }

    // A source that is off opens the setting that turns it on instead of an empty browser.
    function _open(row) {
        if (dl.state) {
            if (row.enabled)
                dl.state.openBrowser(row.key)
            else
                dl.state.openSheet("settings", { tab: "sources", section: 0,
                                                 control: dl._srcs.settingsControl(row.key, row.code) })
        }
        dl.close()
    }

    Repeater {
        model: dl._rows
        delegate: MenuRow {
            required property var modelData
            glyph: modelData.glyph
            label: modelData.label
            detail: modelData.enabled ? "" : modelData.reason
            available: modelData.enabled
            onChosen: dl._open(modelData)
        }
    }
}

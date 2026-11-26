import QtQuick
import Lithic
import Ryoku.Ui.Singletons

// What a result is, read at a glance on its card
Item {
    id: badges

    property var field: null
    property var source: null
    property bool live: false
    property int columns: 6

    readonly property real s: Theme.scale

    function _clock(secs) {
        var n = Math.round(Number(secs) || 0)
        if (n <= 0) return ""
        var m = Math.floor(n / 60), r = n % 60
        return m + ":" + (r < 10 ? "0" : "") + r
    }
    function _sharpness(res) {
        var m = String(res || "").match(/(\d+)\s*[x\u00d7]\s*(\d+)/i)
        if (!m) return ""
        var w = Number(m[1]), h = Number(m[2]), longSide = Math.max(w, h)
        if (longSide >= 7680) return "8K"
        if (longSide >= 5120) return "5K"
        if (longSide >= 3840) return "4K"
        if (longSide >= 2560) return "2K"
        if (longSide >= 1920) return "1080p"
        return w + "\u00d7" + h
    }

    readonly property var rows: {
        var f = badges.field, src = badges.source
        if (!f || !src || src.count === 0) return []
        f.scrollPosition; f.width; f.height
        var end = Math.min(src.count - 1, Math.max(0, f.visibleEnd))
        var start = Math.max(0, end - badges.columns * 8)
        var out = []
        for (var i = start; i <= end; ++i) out.push(i)
        return out
    }

    Repeater {
        model: badges.rows
        Item {
            id: card
            required property int modelData
            readonly property var item: { badges.source.count; return badges.source.get(card.modelData) }
            readonly property rect box: {
                badges.field.scrollPosition
                var r = badges.field.rectOf(card.modelData)
                var o = badges.field.mapToItem(badges, Qt.point(r.x, r.y))
                return Qt.rect(o.x, o.y, r.width, r.height)
            }
            readonly property bool isLive: badges.live || (card.item && (String(card.item.type) === "video" || String(card.item.type) === "we"))
            readonly property string length: card.item ? badges._clock(card.item.durationSecs) : ""
            readonly property string sharp: card.item ? badges._sharpness(card.item.resolution) : ""
            readonly property bool saved: !!card.item && card.item.downloaded === true
            x: box.x
            y: box.y
            width: box.width
            height: box.height
            visible: box.width > 40 && box.y + box.height > 0 && box.y < badges.height

            Row {
                x: 8 * badges.s
                y: 8 * badges.s
                spacing: 4 * badges.s
                Badge {
                    visible: card.isLive
                    glyph: "play_arrow"
                    text: card.length.length > 0 ? card.length : I18n.tr("Live")
                    accent: true
                }
                Badge {
                    visible: card.sharp.length > 0 && !card.isLive
                    text: card.sharp
                }
            }
            Badge {
                visible: card.saved
                anchors.right: parent.right
                anchors.top: parent.top
                anchors.margins: 8 * badges.s
                glyph: "check"
                text: I18n.tr("Saved")
            }
        }
    }

    component Badge: CutRect {
        id: b
        property string text: ""
        property string glyph: ""
        property bool accent: false
        height: 22 * badges.s
        width: row.implicitWidth + 12 * badges.s
        cut: 3 * badges.s
        fill: b.accent ? Lithic.sel : Qt.rgba(0, 0, 0, 0.62)
        Row {
            id: row
            anchors.centerIn: parent
            spacing: 3 * badges.s
            Glyph {
                visible: b.glyph.length > 0
                anchors.verticalCenter: parent.verticalCenter
                name: b.glyph
                size: 13
                ink: b.accent ? Lithic.onSel : "white"
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: b.text
                font.family: Lithic.font.mono
                font.pixelSize: Theme.fs(9.5)
                font.weight: Font.Bold
                color: b.accent ? Lithic.onSel : "white"
                renderType: Text.NativeRendering
            }
        }
    }
}

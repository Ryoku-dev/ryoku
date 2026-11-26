import QtQuick
import Ryoku.Ui.Singletons
import "Singletons"

// F1 reference for internal modes, provider prefixes, and the progressive
// action shelf. It documents both apps with desktop actions and the compact
// zero-secondary case, so expansion never reads like missing UI.
Item {
    id: root

    property real s: 1
    implicitHeight: col.implicitHeight

    readonly property var searchRows: [
        { k: "type",   d: I18n.tr("federated apps, open windows, web and quick math") },
        { k: "ALL",    d: I18n.tr("browse apps alphabetically; typing stays apps-only") },
        { k: "IMG FILE REC", d: I18n.tr("scope to images, files, or standard recent files") },
        { k: "/",      d: I18n.tr("actions: lock, screenshot, media, settings") },
        { k: "/file",  d: I18n.tr("find files (also /folder /image /video)") },
        { k: ">",      d: I18n.tr("packages: >install, >remove, >search") },
        { k: "=",      d: I18n.tr("calculator") },
        { k: "?",      d: I18n.tr("web search (supports !bangs)") },
        { k: "@",      d: I18n.tr("live radio: @lofi tunes in, @stop tunes out") }
    ]
    readonly property var keyRows: [
        { k: "Enter",  d: I18n.tr("run the selected result's primary action") },
        { k: "Up Down", d: I18n.tr("move selection") },
        { k: "Ctrl+K", d: I18n.tr("open real secondary actions; no-op when there are none") },
        { k: "Ctrl+A", d: I18n.tr("open ALL mode") },
        { k: "Tab / arrows", d: I18n.tr("walk an open shelf; the query keeps input focus") },
        { k: "F1",     d: I18n.tr("open or close this reference") },
        { k: "Esc",    d: I18n.tr("cancel preedit, close shelf, leave mode, then close") }
    ]
    readonly property var actionRows: [
        { k: "APP OPTIONS", d: I18n.tr("only real .desktop actions appear; apps without them never open a shelf") },
        { k: "0 MORE", d: I18n.tr("no extra option and no empty space; Enter runs the primary directly") },
        { k: "1 MORE", d: I18n.tr("one extra option in a full-width 38px row") },
        { k: "2-3 MORE", d: I18n.tr("one equal-width 38px action row") },
        { k: "4+ MORE", d: I18n.tr("two per row; an odd final action spans full width") },
        { k: "7+ MORE", d: I18n.tr("three rows stay visible and the shelf scrolls") }
    ]

    Column {
        id: col
        width: parent.width
        spacing: 10 * root.s

        Repeater {
            model: [{ title: I18n.tr("SEARCH + MODES"), rows: root.searchRows },
                    { title: I18n.tr("KEYS"), rows: root.keyRows },
                    { title: I18n.tr("EXPANDED OPTIONS"), rows: root.actionRows }]
            delegate: Column {
                id: group
                required property var modelData
                width: col.width
                spacing: 5 * root.s

                Row {
                    width: parent.width
                    spacing: 8 * root.s
                    Text {
                        text: group.modelData.title
                        color: Theme.faint
                        font.family: Theme.font
                        font.pixelSize: Metrics.fontEyebrow * root.s
                        font.letterSpacing: 1
                    }
                    Rectangle {
                        anchors.verticalCenter: parent.verticalCenter
                        width: col.width - 70 * root.s
                        height: 1
                        color: Theme.hair
                    }
                }

                Repeater {
                    model: group.modelData.rows
                    delegate: Row {
                        required property var modelData
                        width: col.width
                        spacing: 12 * root.s
                        Text {
                            width: 120 * root.s
                            text: modelData.k
                            color: Theme.vermLit
                            font.family: Theme.mono
                            font.pixelSize: Metrics.fontSubtitle * root.s
                        }
                        Text {
                            width: Math.max(0, col.width - 132 * root.s)
                            text: modelData.d
                            color: Theme.subtle
                            font.family: Theme.font
                            font.pixelSize: Metrics.fontSubtitle * root.s
                            wrapMode: Text.WordWrap
                        }
                    }
                }
            }
        }
    }
}

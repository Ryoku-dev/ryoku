import QtQuick
import Quickshell
import Ryoku.Ui.Singletons

Item {
    id: back

    required property PickerState state
    readonly property CardField field: state.field

    readonly property int flipped: back.field ? back.field.flippedIndex : -1
    readonly property rect cardRect: {
        if (!back.field || back.flipped < 0)
            return Qt.rect(0, 0, 0, 0)
        return (back.flipped === back.field.currentIndex)
            ? back.field.currentRect : back.field.rectOf(back.flipped)
    }

    readonly property var viewRow: (back.state.view && back.flipped >= 0 && back.cardRect.width > 0)
        ? back.state.view.get(back.flipped) : ({})
    readonly property string cardKey: back.viewRow.key || ""
    readonly property var meta: back.cardKey ? Library.entry(back.state.collection, back.cardKey) : ({})

    readonly property bool isWe: back.meta.type === "we"
    readonly property bool isStatic: back.meta.type === "static"
    readonly property bool isWallpaper: back.state.collection === "wallpapers" || back.state.collection === "workshop"

    visible: back.field && back.flipped >= 0 && back.cardRect.width > 4 && back.cardRect.height > 4 && back.cardKey

    function _typeLabel(t) {
        switch (t) {
        case "video": return I18n.tr("Video")
        case "we": return I18n.tr("Wallpaper Engine")
        default: return I18n.tr("Image")
        }
    }
    function _humanSize(n) {
        if (!n || n <= 0)
            return ""
        var units = ["B", "KB", "MB", "GB", "TB"]
        var v = n, i = 0
        while (v >= 1024 && i < units.length - 1) { v /= 1024; ++i }
        return (i === 0 ? v : v.toFixed(1)) + " " + units[i]
    }
    function _modified(mtime) {
        if (!mtime || mtime <= 0)
            return ""
        var ms = mtime < 100000000000 ? mtime * 1000 : mtime
        return new Date(ms).toLocaleDateString(Qt.locale(), Locale.ShortFormat)
    }
    function _openInFiles() {
        var p = back.meta.path || ""
        if (!p)
            return
        var cut = p.lastIndexOf("/")
        Quickshell.execDetached(["xdg-open", cut > 0 ? p.substring(0, cut) : p])
    }

    Rectangle {
        x: back.cardRect.x
        y: back.cardRect.y
        width: back.cardRect.width
        height: back.cardRect.height
        color: Theme.withAlpha(Theme.surface, 0.94)
        border.width: 1
        border.color: Theme.withAlpha(Theme.outline, 0.72)
        clip: true

        MouseArea { anchors.fill: parent }   // swallow clicks so they don't fall through to the scene

        FixedButton {
            id: favBtn
            anchors.top: parent.top
            anchors.right: parent.right
            anchors.margins: 14 * Theme.scale
            mode: "toggle"
            active: (back.viewRow.favourite === 1) || (back.viewRow.favourite === true)
            glyph: favBtn.active ? "\u{f02d1}" : "\u{f02d5}"
            fixedWidth: 40 * Theme.scale
            onTriggered: back.state.toggleFavourite(back.flipped)
        }

        Flickable {
            anchors.fill: parent
            anchors.margins: 20 * Theme.scale
            anchors.rightMargin: 66 * Theme.scale
            contentHeight: body.implicitHeight
            clip: true
            boundsBehavior: Flickable.StopAtBounds

            Column {
                id: body
                width: parent.width
                spacing: 10 * Theme.scale

                Text {
                    text: I18n.tr("Library") + " / " + back._typeLabel(back.meta.type).toUpperCase()
                    font.family: Theme.sans
                    font.weight: Font.Medium
                    font.pixelSize: Theme.fontSmall
                    color: Theme.withAlpha(Theme.surfaceText, 0.55)
                    renderType: Text.NativeRendering
                }

                Text {
                    width: parent.width
                    text: back.meta.name || back.viewRow.name || back.cardKey
                    elide: Text.ElideRight
                    maximumLineCount: 2
                    wrapMode: Text.WrapAtWordBoundaryOrAnywhere
                    font.family: Theme.display
                    font.pixelSize: Theme.fontTitle
                    color: Theme.surfaceText
                    renderType: Text.NativeRendering
                }

                Column {
                    width: parent.width
                    spacing: 6 * Theme.scale

                    Repeater {
                        model: [
                            { label: I18n.tr("Resolution"), value: (back.meta.width > 0 && back.meta.height > 0) ? (back.meta.width + " \u00d7 " + back.meta.height) : "" },
                            { label: I18n.tr("Size"), value: back._humanSize(back.meta.filesize) },
                            { label: I18n.tr("Modified"), value: back._modified(back.meta.mtime) },
                            { label: I18n.tr("Applied"), value: (back.meta.applyCount > 0) ? I18n.tr("%1 times").arg(back.meta.applyCount) : "" }
                        ]
                        delegate: Row {
                            required property var modelData
                            visible: modelData.value.length > 0
                            width: body.width
                            spacing: 8 * Theme.scale
                            Text {
                                width: 96 * Theme.scale
                                text: modelData.label.toUpperCase()
                                font.family: Theme.sans
                                font.weight: Font.Medium
                                font.pixelSize: Theme.fontTiny
                                color: Theme.withAlpha(Theme.surfaceText, 0.5)
                                renderType: Text.NativeRendering
                            }
                            Text {
                                text: modelData.value
                                font.family: Theme.display
                                font.pixelSize: Theme.fontBody
                                color: Theme.withAlpha(Theme.surfaceText, 0.9)
                                renderType: Text.NativeRendering
                            }
                        }
                    }
                }

                Row {
                    spacing: 8 * Theme.scale
                    visible: back.viewRow.fill !== undefined
                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("Colour").toUpperCase()
                        font.family: Theme.sans
                        font.weight: Font.Medium
                        font.pixelSize: Theme.fontTiny
                        color: Theme.withAlpha(Theme.surfaceText, 0.5)
                        renderType: Text.NativeRendering
                    }
                    Rectangle {
                        anchors.verticalCenter: parent.verticalCenter
                        width: 34 * Theme.scale
                        height: 16 * Theme.scale
                        color: back.viewRow.fill || Theme.surfaceVariant
                        border.width: 1
                        border.color: Theme.withAlpha(Theme.outline, 0.4)
                    }
                }

                FolioRule { width: parent.width; visible: !!(back.meta.tags && back.meta.tags.length > 0) }
                Text {
                    visible: !!(back.meta.tags && back.meta.tags.length > 0)
                    text: I18n.tr("Tags / %1").arg(back.meta.tags ? back.meta.tags.length : 0)
                    font.family: Theme.sans
                    font.weight: Font.Medium
                    font.pixelSize: Theme.fontTiny
                    color: Theme.withAlpha(Theme.surfaceText, 0.52)
                    renderType: Text.NativeRendering
                }
                Flow {
                    width: parent.width
                    spacing: 6 * Theme.scale
                    visible: !!(back.meta.tags && back.meta.tags.length > 0)
                    Repeater {
                        model: back.meta.tags || []
                        delegate: Rectangle {
                            required property string modelData
                            radius: Theme.radius
                            color: Theme.withAlpha(Theme.background, 0.5)
                            border.width: 1
                            border.color: Theme.withAlpha(Theme.outline, 0.46)
                            implicitWidth: tagText.implicitWidth + 14 * Theme.scale
                            implicitHeight: tagText.implicitHeight + 6 * Theme.scale
                            Text {
                                id: tagText
                                anchors.centerIn: parent
                                text: modelData.toUpperCase()
                                font.family: Theme.sans
                                font.weight: Font.Medium
                                font.pixelSize: Theme.fontFine
                                color: Theme.withAlpha(Theme.surfaceText, 0.84)
                                renderType: Text.NativeRendering
                            }
                        }
                    }
                }

                FolioRule { width: parent.width }
                Text {
                    text: I18n.tr("Actions").toUpperCase()
                    font.family: Theme.sans
                    font.weight: Font.Medium
                    font.pixelSize: Theme.fontTiny
                    color: Theme.withAlpha(Theme.surfaceText, 0.52)
                    renderType: Text.NativeRendering
                }
                Flow {
                    width: parent.width
                    spacing: 7 * Theme.scale

                    FolioAction {
                        label: I18n.tr("Apply")
                        onTriggered: back.state.applyRow(back.flipped, false)
                    }
                    FolioAction {
                        visible: back.isWallpaper
                        label: I18n.tr("Choose target")
                        onTriggered: back.state.applyRow(back.flipped, true)
                    }
                    FolioAction {
                        visible: back.isStatic
                        label: I18n.tr("Effects")
                        onTriggered: back.state.openSheet("effects", { mode: "studio", row: back.flipped })
                    }
                    FolioAction {
                        visible: back.isWe && back.cardKey
                        label: I18n.tr("Scene")
                        onTriggered: back.state.openSheet("sceneProperties", { row: back.flipped })
                    }
                    FolioAction {
                        visible: (back.meta.path || "").length > 0
                        label: I18n.tr("Set as overview")
                        onTriggered: {
                            Settings.set("overviewBackdrop.path", back.meta.path || "")
                            Settings.set("overviewBackdrop.followWallpaper", false)
                            Settings.set("overviewBackdrop.enabled", true)
                            back.state.runAction("refreshBackdrop")
                        }
                    }
                    FolioAction {
                        label: I18n.tr("Add to playlist")
                        onTriggered: back.state.openSheet("playlists", { addKey: back.cardKey, addName: back.meta.name || "" })
                    }
                    FolioAction {
                        visible: (back.meta.path || "").length > 0
                        label: I18n.tr("Open folder")
                        onTriggered: back._openInFiles()
                    }
                    FolioDestructiveAction {
                        visible: back.isWallpaper
                        label: I18n.tr("Delete")
                        confirm: true
                        onTriggered: {
                            Daemon.call("wall.delete", { key: back.cardKey }, function(result, error) {
                                if (error)
                                    back.state.toast(error.message || I18n.tr("This wallpaper could not be deleted."), "error")
                            })
                            if (back.field)
                                back.field.unflip()
                        }
                    }
                }
            }
        }
    }
}

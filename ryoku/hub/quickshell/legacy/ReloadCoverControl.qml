pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Quickshell
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "ReloadCoverModel.js" as ReloadCoverModel

Item {
    id: control

    property var descriptor: ReloadCoverModel.empty()
    property string errorText: ""
    property bool busy: false
    readonly property var media: ReloadCoverModel.normalize(descriptor)
    readonly property url rendererSource: ReloadCoverModel.rendererSource(
        Quickshell.env("RYOKU_SHELL_DIR"),
        Quickshell.env("XDG_CONFIG_HOME"),
        Quickshell.env("HOME"))

    signal addRequested()
    signal defaultRequested()
    signal enabledToggled(bool enabled)

    implicitHeight: grid.implicitHeight

    GridLayout {
        id: grid
        anchors.fill: parent
        columns: control.width >= Tokens.px(720) ? 2 : 1
        columnSpacing: Tokens.s5
        rowSpacing: Tokens.s4

        Item {
            id: preview
            readonly property real previewWidth: Math.min(Tokens.px(420), grid.columns === 1
                ? grid.width : Math.max(0, (grid.width - grid.columnSpacing) / 2))

            Layout.maximumWidth: Tokens.px(420)
            Layout.preferredWidth: previewWidth
            Layout.preferredHeight: previewWidth * 9 / 16
            Layout.fillWidth: grid.columns === 1
            Layout.alignment: Qt.AlignHCenter | Qt.AlignTop
            Accessible.ignored: true

            Rectangle {
                anchors.fill: parent
                color: Tokens.keycapDark
                border.width: Tokens.border
                border.color: Tokens.lineStrong
                radius: Tokens.radius
                clip: true

                Loader {
                    id: renderer
                    anchors.fill: parent
                    active: control.visible
                    asynchronous: true
                    source: active ? control.rendererSource : ""
                    onLoaded: {
                        if (status === Loader.Ready && item) {
                            item.descriptor = Qt.binding(() => control.media)
                            item.active = Qt.binding(() => control.visible)
                            item.forceDefault = false
                        }
                    }
                }

                Text {
                    anchors.centerIn: parent
                    visible: renderer.status === Loader.Error
                    text: UiPreferences.brandMark("力")
                    color: Tokens.inkFaint
                    font.family: Tokens.jp
                    font.pixelSize: Tokens.fHero
                }
            }
        }

        ColumnLayout {
            id: details
            Layout.fillWidth: true
            Layout.alignment: Qt.AlignTop
            spacing: Tokens.s2

            Rectangle {
                readonly property string kindLabel: control.media.path !== "" && !control.media.enabled
                    ? I18n.tr("CUSTOM OFF")
                    : (control.media.kind === "video"
                    ? I18n.tr("VIDEO · MUTED")
                    : (control.media.kind === "animated" ? I18n.tr("ANIMATED IMAGE")
                    : (control.media.kind === "image" ? I18n.tr("IMAGE") : I18n.tr("DEFAULT"))))
                Layout.alignment: Qt.AlignLeft
                implicitWidth: kind.implicitWidth + Tokens.s3 * 2
                implicitHeight: Tokens.ctlH
                color: Tokens.tint10
                border.width: Tokens.border
                border.color: Tokens.line
                radius: Tokens.radius

                Text {
                    id: kind
                    anchors.centerIn: parent
                    text: parent.kindLabel
                    color: Tokens.ink
                    font.family: Tokens.mono
                    font.pixelSize: Tokens.fMicro
                    font.weight: Font.Medium
                    font.letterSpacing: Tokens.trackLabel
                }
            }

            RowLayout {
                Layout.fillWidth: true
                spacing: Tokens.s2

                Text {
                    text: I18n.tr("CUSTOM ASSET")
                    color: Tokens.ink
                    font.family: Tokens.mono
                    font.pixelSize: Tokens.fMicro
                    font.weight: Font.Medium
                    font.letterSpacing: Tokens.trackLabel
                }

                Item { Layout.fillWidth: true }

                Sw {
                    on: control.media.enabled
                    enabled: !control.busy && control.media.path !== ""
                    Accessible.name: I18n.tr("CUSTOM ASSET")
                    onToggled: (enabled) => control.enabledToggled(enabled)
                }
            }

            Text {
                Layout.fillWidth: true
                // the kind chip above already says DEFAULT when nothing custom is
                // set, so the asset line only speaks when there is an asset
                visible: control.media.path !== ""
                text: control.media.name + (ReloadCoverModel.formatBytes(control.media.bytes) === "" ? "" : " · " + ReloadCoverModel.formatBytes(control.media.bytes))
                color: Tokens.ink
                font.family: Tokens.ui
                font.pixelSize: Tokens.fRow
                font.weight: Font.Medium
                elide: Text.ElideMiddle
            }

            Text {
                Layout.fillWidth: true
                text: I18n.tr("BEST RESULTS · MATCH DISPLAY RATIO · 1920x1080 FOR 16:9 · 2-4 S LOOP · UNDER 20 MB")
                color: Tokens.inkDim
                font.family: Tokens.mono
                font.pixelSize: Tokens.fMicro
                font.letterSpacing: Tokens.trackLabel
                wrapMode: Text.WordWrap
            }

            Text {
                Layout.fillWidth: true
                text: I18n.tr("Transparent PNG or SVG works best for a centered mark. Use 24-30 FPS H.264 MP4 or WebM for motion.")
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                wrapMode: Text.WordWrap
            }

            Text {
                Layout.fillWidth: true
                text: I18n.tr("The same asset is fitted independently on every monitor.")
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                wrapMode: Text.WordWrap
            }

            Text {
                Layout.fillWidth: true
                text: I18n.tr("The backend rejects files over 64 MiB.")
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                wrapMode: Text.WordWrap
            }

            Text {
                Layout.fillWidth: true
                visible: control.errorText !== ""
                text: control.errorText
                color: Tokens.ink
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                wrapMode: Text.WordWrap
            }

            Text {
                Layout.fillWidth: true
                visible: renderer.status === Loader.Error
                text: I18n.tr("Preview unavailable; reload still falls back to the Ryoku wordmark.")
                color: Tokens.ink
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                wrapMode: Text.WordWrap
            }

            Text {
                Layout.fillWidth: true
                visible: renderer.status === Loader.Ready && renderer.item && renderer.item.mediaError
                text: I18n.tr("Couldn't preview this asset; shell reload will use the Ryoku wordmark.")
                color: Tokens.ink
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                wrapMode: Text.WordWrap
            }

            RowLayout {
                Layout.fillWidth: true
                spacing: Tokens.s2

                Btn {
                    text: I18n.tr("DEFAULT")
                    armed: !control.busy
                    onAct: control.defaultRequested()
                }
                Btn {
                    text: I18n.tr("ADD ASSET…")
                    primary: true
                    armed: !control.busy
                    onAct: control.addRequested()
                }
                Item { Layout.fillWidth: true }
            }
        }
    }
}

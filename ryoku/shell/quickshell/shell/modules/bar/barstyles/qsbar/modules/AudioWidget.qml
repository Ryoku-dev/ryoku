import QtQuick
import Quickshell
import Quickshell.Io
import shell.services
import Ryoku.Ui.Singletons

Item {
    id: rootMod
    required property var root

    AudioData { id: audio; poll: true }
    readonly property int    volume:   audio.volume
    readonly property bool   muted:    audio.muted
    readonly property string volumeIcon: "graphic_eq"
    readonly property color contentColor: root.widgetContentColor("G6", root.widgetIconColor)

    readonly property string tooltipText: muted
        ? I18n.tr("Muted · %1%").arg(volume)
        : I18n.tr("Audio %1%").arg(volume)

    visible: implicitWidth > 0.5
    implicitWidth: root.modVolume ? row.implicitWidth + 18 : 0
    implicitHeight: 28
    opacity: root.modVolume ? 1 : 0
    Behavior on opacity      { NumberAnimation { duration: 140; easing.type: Easing.OutCubic } }

    Row {
        id: row
        anchors.centerIn: parent
        spacing: 4

        IconText {
            anchors.verticalCenter: parent.verticalCenter
            text: rootMod.volumeIcon
            color: rootMod.muted
                ? Qt.rgba(rootMod.contentColor.r, rootMod.contentColor.g, rootMod.contentColor.b, 0.35)
                : rootMod.contentColor
            font.pixelSize: 15
            font.weight: Font.Medium
            fill: 1
            Behavior on color { ColorAnimation { duration: 160 } }
        }

        UiText {
            visible: !root.iconOnly("G6")
            anchors.verticalCenter: parent.verticalCenter
            text: String(rootMod.volume).padStart(2, '0') + "%"
            color: rootMod.muted
                ? Qt.rgba(rootMod.contentColor.r, rootMod.contentColor.g, rootMod.contentColor.b, 0.35)
                : rootMod.contentColor
            font.family: root.mono
            font.pixelSize: 12
            Behavior on color { ColorAnimation { duration: 160 } }
        }

    }

    TooltipMixin { id: tip; root: rootMod.root; owner: rootMod; text: rootMod.tooltipText }

    // Native PipeWire control through the tracked default sink (AudioData binds
    // it): setting volume/mute writes straight to the node, so the bar readout and
    // the hardware never disagree. No wpctl/pactl/pamixer shell-out.
    function stepVolume(up) {
        var s = audio.sink
        if (!s || !s.audio) return
        var max = root.audioBoost ? 1.5 : 1.0
        var cur = Math.round(s.audio.volume * 100)
        var step = Audio.volumeStep
        var next = step === 5
            ? (up
                ? (cur === 65 ? 67 : cur === 67 ? 70 : Math.floor(cur / 5) * 5 + 5)
                : (cur === 70 ? 67 : cur === 67 ? 65 : Math.ceil(cur / 5) * 5 - 5))
            : cur + (up ? step : -step)
        s.audio.volume = Math.max(0, Math.min(max, next / 100))
    }
    function toggleMute() {
        var s = audio.sink
        if (s && s.audio) s.audio.muted = !s.audio.muted
    }

    MouseArea {
        anchors.fill: parent
        hoverEnabled: true
        cursorShape: Qt.PointingHandCursor
        acceptedButtons: Qt.LeftButton | Qt.RightButton
        onEntered: tip.show()
        onExited: { tip.hide() }
        onWheel: (e) => rootMod.stepVolume(e.angleDelta.y > 0)
        onClicked: (e) => {
            tip.hide()
            if (e.button === Qt.RightButton) rootMod.toggleMute()
            else                             root.volVisible = !root.volVisible
        }
    }
}

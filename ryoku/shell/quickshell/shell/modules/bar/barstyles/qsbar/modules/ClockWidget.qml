import QtQuick
import Ryoku.Ui.Singletons
import Quickshell
import Quickshell.Io
import shell.services as Svc

Item {
    id: rootMod
    required property var root
    signal activated()

    readonly property date now: clk.date
    readonly property color contentColor: root.widgetContentColor("G8", root.ink)

    function pad(n) { return n < 10 ? "0" + n : String(n) }

    readonly property string timeStr: {
        if (root.clock12h) {
            var h = now.getHours() % 12; if (h === 0) h = 12
            return h + ":" + pad(now.getMinutes()) + " " + (now.getHours() < 12 ? "AM" : "PM")
        }
        return pad(now.getHours()) + ":" + pad(now.getMinutes())
    }

    // weekday/month names follow the shell's regional-formats locale (Hub ->
    // Region), so a Brazilian desktop reads its own names with an English UI.
    readonly property string tooltipText: rootMod.now.toLocaleDateString(Svc.Config.formatLoc, Locale.LongFormat)

    implicitWidth: label.implicitWidth
    implicitHeight: 28

    SystemClock {
        id: clk
        precision: SystemClock.Minutes
    }

    UiText {
        id: label
        anchors.centerIn: parent
        text: rootMod.timeStr
        color: rootMod.contentColor
        font.family: root.mono
        font.pixelSize: 12
        font.letterSpacing: 1
    }

    TooltipMixin { id: tip; root: rootMod.root; owner: rootMod; text: rootMod.tooltipText }


    MouseArea {
        id: mouse
        anchors.fill: parent
        hoverEnabled: true
        cursorShape: Qt.PointingHandCursor
        acceptedButtons: Qt.LeftButton | Qt.RightButton
        onEntered: { tip.show(); }
        onExited: { tip.hide(); }
        onClicked: (e) => {
            tip.hide();
            if (e.button === Qt.RightButton) TimeFormat.set24h(root.clock12h);
            else rootMod.activated();
        }
    }
}

import QtQuick
import Ryoku.Ui.Singletons
import "../kit"

Column {
    id: page

    required property var cfg
    property bool live: true
    property date now: new Date()

    spacing: Md.seam

    Timer {
        running: page.visible && page.live
        repeat: true
        triggeredOnStart: true
        interval: 1000
        onTriggered: page.now = new Date()
    }

    Segment {
        position: "only"
        height: 112

        Label {
            anchors.centerIn: parent
            role: "display"
            font.family: Md.mono
            font.pixelSize: 44
            text: TimeFormat.format(page.now, page.cfg.clockSeconds)
        }
    }

    SectionLabel { text: I18n.tr("Time") }
    ListItem {
        objectName: "clock12h"
        width: parent.width
        position: "first"
        icon: "schedule"
        headline: I18n.tr("12-hour clock")
        supporting: I18n.tr("Read the time as 1 to 12 instead of 00 to 23")
        onClicked: page.cfg.set("clock12h", !page.cfg.clock12h)

        Switch {
            anchors.verticalCenter: parent.verticalCenter
            checked: page.cfg.clock12h
            onToggled: function (v) { page.cfg.set("clock12h", v); }
        }
    }
    ListItem {
        objectName: "clockSeconds"
        width: parent.width
        position: "last"
        icon: "timer"
        headline: I18n.tr("Show seconds")
        supporting: I18n.tr("Tick the clock every second")
        onClicked: page.cfg.set("clockSeconds", !page.cfg.clockSeconds)

        Switch {
            anchors.verticalCenter: parent.verticalCenter
            checked: page.cfg.clockSeconds
            onToggled: function (v) { page.cfg.set("clockSeconds", v); }
        }
    }
}

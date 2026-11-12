import QtQuick
import Ryoku.Ui.Singletons
import "../kit"

Column {
    id: page

    required property var cfg

    spacing: Md.seam

    SectionLabel { topPadding: 4; text: I18n.tr("Size and position") }
    SliderItem {
        objectName: "scale"
        width: parent.width
        position: "first"
        icon: "open_in_full"
        headline: I18n.tr("Island size")
        supporting: I18n.tr("Scales the clock island, resting and expanded")
        from: 80
        to: 130
        suffix: "%"
        value: Math.round(page.cfg.scale * 100)
        onCommitted: function (v) { page.cfg.set("scale", v / 100); }
    }
    SliderItem {
        objectName: "topGap"
        width: parent.width
        position: "last"
        icon: "vertical_align_top"
        headline: I18n.tr("Top offset")
        supporting: I18n.tr("How far the island floats below the top edge of the screen")
        from: 0
        to: 24
        suffix: " px"
        value: Math.round(page.cfg.topGap)
        onCommitted: function (v) { page.cfg.set("topGap", v); }
    }

    SectionLabel { text: I18n.tr("What the island shows") }
    ListItem {
        objectName: "islandOsd"
        width: parent.width
        position: "first"
        icon: "volume_up"
        headline: I18n.tr("Volume and brightness")
        supporting: I18n.tr("Show the new level in the island as you change it")
        wrapSupporting: true
        onClicked: page.cfg.set("islandOsd", !page.cfg.islandOsd)

        Switch {
            anchors.verticalCenter: parent.verticalCenter
            checked: page.cfg.islandOsd
            onToggled: function (v) { page.cfg.set("islandOsd", v); }
        }
    }
    ListItem {
        objectName: "islandNotifications"
        width: parent.width
        position: "middle"
        icon: "notifications"
        headline: I18n.tr("Show notifications in the island")
        supporting: I18n.tr("New notifications drop out of the clock as cards. Turn it off for Ryoku's banners")
        wrapSupporting: true
        onClicked: page.cfg.set("islandNotifications", !page.cfg.islandNotifications)

        Switch {
            anchors.verticalCenter: parent.verticalCenter
            checked: page.cfg.islandNotifications
            onToggled: function (v) { page.cfg.set("islandNotifications", v); }
        }
    }
    ListItem {
        objectName: "tray"
        width: parent.width
        position: "last"
        icon: "expand_circle_down"
        headline: I18n.tr("App tray")
        supporting: I18n.tr("A caret under the clock opens the system tray")
        onClicked: page.cfg.set("tray", !page.cfg.tray)

        Switch {
            anchors.verticalCenter: parent.verticalCenter
            checked: page.cfg.tray
            onToggled: function (v) { page.cfg.set("tray", v); }
        }
    }
}

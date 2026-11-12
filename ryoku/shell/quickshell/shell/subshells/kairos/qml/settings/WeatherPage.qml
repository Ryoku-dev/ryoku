pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import Ryoku.Ui.Singletons
import "../kit"

Column {
    id: page

    required property var cfg
    // hands focus back to the window so the next Escape closes it
    signal fieldLeft()

    property bool searchMode: Config.weatherLocation.length > 0
    readonly property string typed: place.text.trim()

    spacing: Md.seam

    function usePlace() {
        if (page.typed.length > 0 && page.typed !== Config.weatherLocation)
            page.cfg.setGlobal("weatherLocation", page.typed);
    }

    SectionLabel { topPadding: 4; text: I18n.tr("In quick settings") }
    ListItem {
        objectName: "unit"
        width: parent.width
        position: "first"
        clickable: false
        icon: "thermostat"
        headline: I18n.tr("Temperature unit")
        supporting: I18n.tr("How quick settings reads temperature")

        Chip {
            anchors.verticalCenter: parent.verticalCenter
            text: "\u00B0C"
            selected: Config.weatherUnit !== "fahrenheit"
            onClicked: Weather.setUnit("celsius")
        }
        Chip {
            anchors.verticalCenter: parent.verticalCenter
            text: "\u00B0F"
            selected: Config.weatherUnit === "fahrenheit"
            onClicked: Weather.setUnit("fahrenheit")
        }
    }
    ListItem {
        objectName: "showLocation"
        width: parent.width
        position: "last"
        icon: "location_on"
        headline: I18n.tr("Show location")
        supporting: I18n.tr("Name the place after the temperature in quick settings")
        wrapSupporting: true
        onClicked: page.cfg.set("showLocation", !page.cfg.showLocation)

        Switch {
            anchors.verticalCenter: parent.verticalCenter
            checked: page.cfg.showLocation
            onToggled: function (v) { page.cfg.set("showLocation", v); }
        }
    }

    SectionLabel { text: I18n.tr("Location") }
    ListItem {
        objectName: "location"
        width: parent.width
        position: "first"
        icon: "my_location"
        headline: I18n.tr("Automatic")
        supporting: I18n.tr("Find the place from your network address")
        onClicked: {
            page.searchMode = false;
            if (Config.weatherLocation.length > 0)
                page.cfg.setGlobal("weatherLocation", "");
        }

        Icon {
            anchors.verticalCenter: parent.verticalCenter
            name: page.searchMode ? "radio_button_unchecked" : "radio_button_checked"
            size: 24
            filled: !page.searchMode
            color: page.searchMode ? Md.inkMuted : Md.accent
        }
    }
    ListItem {
        width: parent.width
        position: page.searchMode ? "middle" : "last"
        icon: "search"
        headline: I18n.tr("Choose a place")
        supporting: Config.weatherLocation.length > 0
            ? I18n.tr("Showing weather for %1").arg(Config.weatherLocation)
            : I18n.tr("Search for a city by name")
        onClicked: {
            page.searchMode = true;
            Qt.callLater(function () { place.field.forceActiveFocus(); });
        }

        Icon {
            anchors.verticalCenter: parent.verticalCenter
            name: page.searchMode ? "radio_button_checked" : "radio_button_unchecked"
            size: 24
            filled: page.searchMode
            color: page.searchMode ? Md.accent : Md.inkMuted
        }
    }
    Item {
        width: parent.width
        height: 84
        visible: page.searchMode

        Rectangle {
            anchors.fill: parent
            topLeftRadius: Md.rXs
            topRightRadius: Md.rXs
            bottomLeftRadius: Md.rXl
            bottomRightRadius: Md.rXl
            color: Md.container
        }

        TextField {
            id: place
            anchors { left: parent.left; right: apply.left; verticalCenter: parent.verticalCenter; leftMargin: 16; rightMargin: 12 }
            text: Config.weatherLocation
            placeholder: I18n.tr("City, for example Lisbon")
            onAccepted: page.usePlace()
            onEscaped: page.fieldLeft()
        }
        Button {
            id: apply
            anchors { right: parent.right; verticalCenter: parent.verticalCenter; rightMargin: 16 }
            variant: "filled"
            text: I18n.tr("Use this place")
            enabled: page.typed.length > 0 && page.typed !== Config.weatherLocation
            onClicked: page.usePlace()
        }
    }
}

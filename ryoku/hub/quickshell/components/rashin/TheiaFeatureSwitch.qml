import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"

Item {
    id: root

    property var row: null
    property var value: false
    property var commit: null
    property bool available: true
    readonly property var shows: root.row && root.row.shows ? root.row.shows : []

    width: LithicTokens.size.settingsControlMax
    implicitHeight: Math.max(places.implicitHeight, controlSlot.implicitHeight)

    function activate() {
        if (root.available && root.enabled && root.visible && root.commit)
            root.commit(root.value !== true);
    }
    function placeKey(place) {
        return String(place);
    }

    function placeLabel(place) {
        switch (root.placeKey(place)) {
        case "look": return I18n.tr("Look");
        case "highlight": return I18n.tr("Highlight");
        case "inplace": return I18n.tr("In place");
        case "panel": return I18n.tr("Panel");
        case "bubble": return I18n.tr("Bubble");
        case "settings": return I18n.tr("Settings");
        default: return I18n.tr(String(place));
        }
    }
    function placeIcon(place) {
        switch (root.placeKey(place)) {
        case "look": return "frame_inspect";
        case "highlight": return "ink_highlighter";
        case "inplace": return "edit_note";
        case "panel": return "select_window";
        case "bubble": return "chat_bubble";
        case "settings": return "tune";
        default: return "chat";
        }
    }

    Flow {
        id: places
        anchors.left: parent.left
        anchors.right: controlSlot.left
        anchors.rightMargin: LithicTokens.space.s3
        anchors.verticalCenter: parent.verticalCenter
        spacing: LithicTokens.space.s1
        Repeater {
            model: root.shows
            delegate: CutRect {
                id: chip
                required property var modelData
                implicitWidth: Math.min(places.width, contents.implicitWidth + LithicTokens.space.s2 * 2)
                implicitHeight: contents.implicitHeight + LithicTokens.space.s1 * 2
                cut: LithicTokens.cut.small
                fill: Lithic.field
                strokeColor: Lithic.vein
                strokeWidth: LithicTokens.size.hairline
                Accessible.role: Accessible.StaticText
                Accessible.name: I18n.tr("Shows in %1").arg(root.placeLabel(chip.modelData))

                Row {
                    id: contents
                    anchors.centerIn: parent
                    spacing: LithicTokens.space.s1
                    Glyph {
                        anchors.verticalCenter: parent.verticalCenter
                        name: root.placeIcon(chip.modelData)
                        size: 16
                        ink: Lithic.fg3
                    }
                    Label {
                        anchors.verticalCenter: parent.verticalCenter
                        text: root.placeLabel(chip.modelData)
                        typeRole: "label"
                        inkRole: "fg2"
                    }
                }
            }
        }
    }

    Item {
        id: controlSlot
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        implicitWidth: root.available ? toggle.implicitWidth : unknown.implicitWidth
        implicitHeight: Math.max(toggle.implicitHeight, unknown.implicitHeight)

        Toggle {
            id: toggle
            anchors.centerIn: parent
            visible: root.available
            checked: root.value === true
            Accessible.name: root.row ? I18n.tr(root.row.label) : ""
            onToggled: {
                var want = toggle.checked;
                toggle.checked = Qt.binding(() => root.value === true);
                if (root.available && root.enabled && root.visible && root.commit)
                    root.commit(want);
            }
        }
        Label {
            id: unknown
            anchors.centerIn: parent
            visible: !root.available
            text: I18n.tr("Not loaded")
            typeRole: "detail"
            inkRole: "fg3"
        }
    }
}

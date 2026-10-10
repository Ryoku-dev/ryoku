import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons

// A subtab strip under the page head. Themes uses it to browse colour schemes
// per provider; the Decor category switches between its picture catalogues.
// Entries are plain names, or {key, label} pairs when the label and the value
// differ. The leading All plate and the trailing plate are optional, so a strip
// can be just its entries.
Item {
    id: tabs

    property var providers: []        // names, or {key, label} objects
    property string active: ""        // the picked key
    property string allLabel: I18n.tr("ALL")   // "" hides the leading plate
    property string trailingLabel: "" // "" hides the trailing plate
    property string trailingKey: ""
    property int installableCount: 0  // uninstalled items in the focused entry
    property bool busy: false
    property bool reducedMotion: false

    signal picked(string filter)
    signal installAll()
    implicitHeight: 44

    component Plate: Rectangle {
        id: plate
        property string label: ""
        property bool on: false
        signal chose()
        width: plateText.implicitWidth + Tokens.s4
        height: 30
        radius: Tokens.radius
        color: plate.on ? Tokens.bone
              : (plateTap.pressed ? Tokens.tint16 : (ph.hovered ? Tokens.tint5 : "transparent"))
        border.width: Tokens.border
        border.color: plate.on ? Tokens.bone : (ph.hovered ? Tokens.lineStrong : Tokens.line)
        scale: plateTap.pressed ? 0.97 : 1
        activeFocusOnTab: true
        Accessible.role: Accessible.Button
        Behavior on color {
            enabled: !tabs.reducedMotion
            ColorAnimation { duration: Tokens.snap }
        }
        Behavior on border.color {
            enabled: !tabs.reducedMotion
            ColorAnimation { duration: Tokens.snap }
        }
        Behavior on scale {
            enabled: !tabs.reducedMotion
            NumberAnimation { duration: Tokens.snap; easing.type: Tokens.easeSnap }
        }
        Accessible.name: plate.label
        Accessible.onPressAction: plate.chose()
        Keys.onPressed: event => {
            if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter || event.key === Qt.Key_Space) {
                plate.chose();
                event.accepted = true;
            }
        }
        Text {
            id: plateText
            anchors.centerIn: parent
            text: I18n.tr(plate.label)
            color: plate.on ? Tokens.inkOnBone : (plate.activeFocus ? Tokens.ink : Tokens.inkDim)
            font.family: Tokens.ui
            font.pixelSize: 11
            font.weight: Font.Medium
            font.letterSpacing: Tokens.trackLabel
            Behavior on color {
                enabled: !tabs.reducedMotion
                ColorAnimation { duration: Tokens.snap }
            }
        }
        HoverHandler { id: ph; cursorShape: Qt.PointingHandCursor }
        TapHandler { id: plateTap; onTapped: plate.chose() }
    }

    Flickable {
        id: strip
        anchors {
            left: parent.left; leftMargin: Tokens.s6
            right: installAllBtn.left; rightMargin: Tokens.s4
            verticalCenter: parent.verticalCenter
        }
        height: 30
        contentWidth: plateRow.width
        contentHeight: height
        clip: true
        flickableDirection: Flickable.HorizontalFlick
        boundsBehavior: Flickable.StopAtBounds

        Row {
            id: plateRow
            height: parent.height
            spacing: Tokens.s2

            Plate {
                objectName: "ryostore-provider-all"
                visible: tabs.allLabel !== ""
                label: tabs.allLabel
                on: tabs.active === ""
                onChose: tabs.picked("")
            }
            Repeater {
                model: tabs.providers
                delegate: Plate {
                    required property var modelData
                    readonly property string entryKey: (modelData && modelData.key !== undefined)
                            ? String(modelData.key) : String(modelData)
                    readonly property string entryLabel: (modelData && modelData.label !== undefined)
                            ? String(modelData.label) : String(modelData)
                    objectName: "ryostore-provider-" + entryKey
                    label: entryLabel.toUpperCase()
                    on: tabs.active === entryKey
                    onChose: tabs.picked(entryKey)
                }
            }
            Plate {
                objectName: "ryostore-provider-mine"
                visible: tabs.trailingLabel !== ""
                label: tabs.trailingLabel
                on: tabs.active === tabs.trailingKey && tabs.trailingKey !== ""
                onChose: tabs.picked(tabs.trailingKey)
            }
        }
    }

    Btn {
        id: installAllBtn
        objectName: "ryostore-provider-install-all"
        anchors { right: parent.right; rightMargin: Tokens.s6; verticalCenter: parent.verticalCenter }
        visible: tabs.active !== "" && tabs.active !== "__mine__" && tabs.installableCount > 0
        text: tabs.busy ? I18n.tr("INSTALLING") : (I18n.tr("INSTALL ALL / ") + tabs.installableCount)
        armed: !tabs.busy
        onAct: tabs.installAll()
        Accessible.role: Accessible.Button
        Accessible.name: installAllBtn.text
        Accessible.onPressAction: tabs.installAll()
    }

    Rectangle {
        anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
        height: Tokens.border
        color: Tokens.line
    }
}

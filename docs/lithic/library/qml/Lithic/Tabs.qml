import QtQuick
import QtQuick.Templates as T

T.Control {
    id: root

    property var model: []
    property int currentIndex: 0
    property string previewState: ""
    property string accessibleName: Lithic.tr("Tabs")

    signal activated(int index)

    readonly property Item _selectedItem: currentIndex >= 0 ? tabRepeater.itemAt(currentIndex) : null

    focusPolicy: Qt.StrongFocus
    implicitWidth: contentItem.implicitWidth
    implicitHeight: LithicTokens.size.tabsHeight
    opacity: enabled ? 1 : Lithic.disabledOpacity

    Accessible.role: Accessible.PageTabList
    Accessible.name: accessibleName

    function move(delta) {
        if (!enabled || tabRepeater.count === 0)
            return;
        const next = (currentIndex + delta + tabRepeater.count) % tabRepeater.count;
        currentIndex = next;
        const item = tabRepeater.itemAt(next);
        if (item)
            item.forceActiveFocus();
        activated(next);
    }

    Keys.onLeftPressed: function(event) {
        move(-1);
        event.accepted = true;
    }
    Keys.onRightPressed: function(event) {
        move(1);
        event.accepted = true;
    }

    contentItem: Item {
        implicitWidth: tabRow.implicitWidth
        implicitHeight: LithicTokens.size.tabsHeight

        Rectangle {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            height: LithicTokens.size.tabsBaseline
            color: Lithic.vein
        }

        Row {
            id: tabRow
            height: parent.height

            Repeater {
                id: tabRepeater
                model: root.model.slice(0, LithicTokens.size.tabsMaxCount)

                delegate: T.AbstractButton {
                    id: tab

                    required property int index
                    required property var modelData
                    readonly property string tabText: typeof modelData === "string" ? modelData : String(modelData.label || "")
                    readonly property bool selected: index === root.currentIndex
                    readonly property bool _hoverActive: enabled && (hovered || (index === 0 && root.previewState === "hover"))
                    readonly property bool _pressActive: enabled && (pressed || (index === 0 && root.previewState === "pressed"))
                    readonly property bool _focusActive: enabled && (visualFocus || (selected && root.previewState === "focus"))

                    hoverEnabled: true
                    focusPolicy: Qt.StrongFocus
                    implicitWidth: tabLabel.implicitWidth + LithicTokens.size.tabsLabelExtra
                    implicitHeight: LithicTokens.size.tabsHeight
                    enabled: root.enabled

                    Accessible.role: Accessible.PageTab
                    Accessible.name: tabText
                    Accessible.selected: selected

                    Keys.onLeftPressed: function(event) {
                        root.move(-1);
                        event.accepted = true;
                    }
                    Keys.onRightPressed: function(event) {
                        root.move(1);
                        event.accepted = true;
                    }

                    onClicked: {
                        root.currentIndex = index;
                        root.activated(index);
                    }

                    background: FocusRing {
                        x: -outset
                        y: -outset
                        width: parent.width + outset * 2
                        height: parent.height + outset * 2
                        keyboardFocusVisible: tab._focusActive
                        cut: LithicTokens.cut.token
                    }

                    contentItem: Label {
                        id: tabLabel
                        text: tab.tabText
                        typeRole: "label"
                        ink: tab.selected ? Lithic.fg : (tab._hoverActive ? Lithic.fg2 : Lithic.fg3)
                        horizontalAlignment: Text.AlignHCenter
                        verticalAlignment: Text.AlignVCenter

                        transform: Translate {
                            y: tab._pressActive ? Lithic.pressSink : 0
                            ChiselNumberBehavior on y { duration: LithicTokens.motion.press }
                        }

                        ChiselColorBehavior on ink {
                            duration: tab._hoverActive ? LithicTokens.motion.hoverIn : LithicTokens.motion.hoverOut
                        }
                    }
                }
            }
        }

        CutRect {
            x: root._selectedItem ? root._selectedItem.x : 0
            y: parent.height - LithicTokens.size.tabsBlade
            width: root._selectedItem ? root._selectedItem.width : 0
            height: LithicTokens.size.tabsBlade
            fill: Lithic.fg
            cutTopLeft: LithicTokens.size.tabsBlade
            cutTopRight: LithicTokens.size.tabsBlade
            visible: root._selectedItem !== null

            ChiselNumberBehavior on x { duration: LithicTokens.motion.select }
            ChiselNumberBehavior on width { duration: LithicTokens.motion.select }
        }
    }
}

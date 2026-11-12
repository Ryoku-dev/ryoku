import QtQuick
import QtQuick.Templates as T

T.Control {
    id: root

    property var model: []
    property int currentIndex: 0
    property string previewState: ""
    property string accessibleName: Lithic.tr("Filters")

    signal activated(int index)

    readonly property Item _selectedItem: currentIndex >= 0 && currentIndex < chipRepeater.count
                                          ? chipRepeater.itemAt(currentIndex)
                                          : null

    focusPolicy: Qt.StrongFocus
    implicitWidth: contentItem.implicitWidth
    implicitHeight: LithicTokens.size.filterHeight
    opacity: enabled ? 1 : Lithic.disabledOpacity

    Accessible.role: Accessible.List
    Accessible.name: accessibleName

    function move(delta) {
        if (!enabled || chipRepeater.count === 0)
            return;
        const next = Math.max(0, Math.min(chipRepeater.count - 1, currentIndex + delta));
        if (next !== currentIndex) {
            currentIndex = next;
            activated(next);
        }
        forceActiveFocus();
    }

    Keys.onLeftPressed: function(event) {
        move(-1);
        event.accepted = true;
    }
    Keys.onRightPressed: function(event) {
        move(1);
        event.accepted = true;
    }
    Keys.onPressed: function(event) {
        if (event.key === Qt.Key_Home && chipRepeater.count > 0) {
            currentIndex = 0;
            activated(0);
            event.accepted = true;
        } else if (event.key === Qt.Key_End && chipRepeater.count > 0) {
            currentIndex = chipRepeater.count - 1;
            activated(currentIndex);
            event.accepted = true;
        }
    }

    contentItem: Item {
        implicitWidth: chipRow.implicitWidth
        implicitHeight: LithicTokens.size.filterHeight

        Repeater {
            model: chipRepeater.count

            delegate: CutRect {
                required property int index
                readonly property Item chip: index < chipRepeater.count ? chipRepeater.itemAt(index) : null

                visible: chip !== null && !chip.selected
                x: chip ? chip.x : 0
                width: chip ? chip.width : 0
                height: LithicTokens.size.filterHeight
                fill: chip && chip._hoverActive ? Lithic.field : Lithic.alpha(Lithic.field, 0)
                cut: LithicTokens.cut.small + (chip && chip._pressActive ? Lithic.pressCutGrow : 0)

                ChiselColorBehavior on fill {
                    duration: chip && chip._hoverActive ? LithicTokens.motion.hoverIn : LithicTokens.motion.hoverOut
                }
                ChiselNumberBehavior on cut { duration: LithicTokens.motion.press }
            }
        }

        CutRect {
            x: root._selectedItem ? root._selectedItem.x : 0
            width: root._selectedItem ? root._selectedItem.width : 0
            height: LithicTokens.size.filterHeight
            fill: Lithic.sel
            cut: LithicTokens.cut.small
            visible: root._selectedItem !== null

            ChiselNumberBehavior on x { duration: LithicTokens.motion.select }
            ChiselNumberBehavior on width { duration: LithicTokens.motion.select }
        }

        Row {
            id: chipRow
            spacing: LithicTokens.size.filterGap

            Repeater {
                id: chipRepeater
                model: root.model

                delegate: T.AbstractButton {
                    id: chip

                    required property int index
                    required property var modelData
                    readonly property string chipText: typeof modelData === "string" ? modelData : String(modelData.label || "")
                    readonly property string countText: typeof modelData === "object" && modelData.count !== undefined
                                                        ? String(modelData.count)
                                                        : ""
                    readonly property bool selected: index === root.currentIndex
                    readonly property bool _hoverActive: enabled && (hovered || (index === 0 && root.previewState === "hover"))
                    readonly property bool _pressActive: enabled && (pressed || (index === 0 && root.previewState === "pressed"))
                    readonly property bool _focusActive: enabled && (visualFocus || (selected && (root.visualFocus || root.previewState === "focus")))

                    hoverEnabled: true
                    focusPolicy: Qt.StrongFocus
                    leftPadding: LithicTokens.size.filterInset
                    rightPadding: LithicTokens.size.filterInset
                    implicitWidth: implicitContentWidth + leftPadding + rightPadding
                    implicitHeight: LithicTokens.size.filterHeight
                    enabled: root.enabled

                    Accessible.role: Accessible.RadioButton
                    Accessible.name: chipText
                    Accessible.checked: selected

                    onClicked: {
                        root.currentIndex = index;
                        root.activated(index);
                    }

                    background: Item {
                        FocusRing {
                            x: -outset
                            y: -outset
                            width: parent.width + outset * 2
                            height: parent.height + outset * 2
                            keyboardFocusVisible: chip._focusActive
                            cut: LithicTokens.cut.small
                        }
                    }

                    contentItem: Item {
                        implicitWidth: chipContent.implicitWidth
                        implicitHeight: chipContent.implicitHeight

                        Row {
                            id: chipContent
                            anchors.centerIn: parent
                            spacing: chip.countText.length > 0 ? LithicTokens.size.filterCountGap : 0
                            transform: Translate {
                                y: chip._pressActive ? Lithic.pressSink : 0
                                ChiselNumberBehavior on y { duration: LithicTokens.motion.press }
                            }

                            Label {
                                text: chip.chipText
                                typeRole: "label"
                                ink: chip.selected ? Lithic.onSel : Lithic.fg2
                            }
                            Label {
                                visible: chip.countText.length > 0
                                text: chip.countText
                                typeRole: "caption"
                                numeric: true
                                ink: chip.selected ? Lithic.onSel2 : Lithic.fg3
                            }
                        }
                    }
                }
            }
        }
    }
}

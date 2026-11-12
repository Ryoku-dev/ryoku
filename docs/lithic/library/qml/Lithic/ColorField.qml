import QtQuick
import QtQuick.Templates as T

T.Control {
    id: root

    property color color: Lithic.gold
    property string label: ""
    property bool allowAlpha: false
    property var swatches: []
    property Item overlayParent: null
    property string accessibleName: label.length > 0 ? label : Lithic.tr("Colour")
    property string previewState: ""
    property bool previewOpen: false
    readonly property bool opened: previewOpen || popover.opened
    readonly property string hex: formatHex(color)
    readonly property string invalidText: Lithic.tr("Use a colour like #ce5e32")

    signal picked(color c)

    readonly property real _labelHeight: label.length > 0 ? Lithic.type.detail.lineHeight + LithicTokens.space.s1 : 0
    readonly property real _disclosureSlot: LithicTokens.size.textFieldSecretAction + LithicTokens.size.textFieldSecretInset * 2

    implicitWidth: LithicTokens.size.textFieldHeight + LithicTokens.space.s2 + LithicTokens.size.pickerWidthMin
    implicitHeight: _labelHeight + hexField.implicitHeight

    Accessible.role: Accessible.Grouping
    Accessible.name: accessibleName
    Accessible.description: hex

    function _channel(value) {
        const text = Math.round(Math.max(0, Math.min(1, value)) * 255).toString(16);
        return text.length < 2 ? "0" + text : text;
    }

    // CSS order, so a pasted #rrggbbaa means what it means on the web
    function formatHex(value) {
        const c = Qt.color(value);
        const base = "#" + _channel(c.r) + _channel(c.g) + _channel(c.b);
        return allowAlpha && c.a < 1 ? base + _channel(c.a) : base;
    }

    function parseHex(text) {
        let value = String(text || "").trim();
        if (value.charAt(0) === "#")
            value = value.slice(1);
        if (!/^[0-9a-fA-F]+$/.test(value))
            return null;
        if (value.length === 3 || (allowAlpha && value.length === 4))
            value = value.split("").map(digit => digit + digit).join("");
        if (value.length !== 6 && !(allowAlpha && value.length === 8))
            return null;
        const channel = index => parseInt(value.substr(index * 2, 2), 16) / 255;
        return Qt.rgba(channel(0), channel(1), channel(2), value.length === 8 ? channel(3) : 1);
    }

    function choose(value) {
        const next = Qt.color(value);
        const changed = !Qt.colorEqual(next, color);
        color = next;
        hexField.text = hex;
        hexField.errorText = "";
        popoverHex.text = hex;
        popoverHex.errorText = "";
        if (changed)
            picked(next);
    }

    function commit(field) {
        if (field.text === hex) {
            field.errorText = "";
            return;
        }
        const value = parseHex(field.text);
        if (value === null) {
            field.errorText = invalidText;
            return;
        }
        choose(value);
    }

    function open() {
        if (enabled)
            popover.open();
    }

    function close() {
        popover.close();
    }

    function toggle() {
        if (popover.opened)
            close();
        else
            open();
    }

    onHexChanged: {
        if (!hexField.activeFocus) {
            hexField.text = hex;
            hexField.errorText = "";
        }
        if (!popoverHex.activeFocus) {
            popoverHex.text = hex;
            popoverHex.errorText = "";
        }
    }
    onColorChanged: alphaSlider.value = Math.round(Qt.color(color).a * 100)
    // the error preview shows a half typed value, as a reader leaves it
    Component.onCompleted: {
        if (previewState === "error") {
            hexField.text = hex.slice(0, hex.length - 1);
            hexField.errorText = invalidText;
        }
    }

    background: null

    // the hex field and its plaque dim themselves; the label and plate follow
    contentItem: Item {
        Label {
            visible: root.label.length > 0
            x: 0
            y: 0
            text: root.label
            typeRole: "detail"
            opacity: root.enabled ? 1 : Lithic.disabledOpacity
        }

        ColorSwatch {
            id: plate
            x: 0
            y: root._labelHeight
            width: LithicTokens.size.textFieldHeight
            height: LithicTokens.size.textFieldHeight
            color: root.color
            opacity: root.enabled ? 1 : Lithic.disabledOpacity
        }

        TextField {
            id: hexField
            x: plate.width + LithicTokens.space.s2
            y: root._labelHeight
            width: parent.width - x
            mono: true
            text: root.hex
            rightPadding: root._disclosureSlot
            previewState: root.previewState === "focus" ? "" : root.previewState
            Accessible.name: Lithic.tr("%1 hex").arg(root.accessibleName)
            onEditingFinished: root.commit(hexField)

            IconPlaque {
                id: disclosure
                anchors.right: parent.right
                anchors.rightMargin: LithicTokens.size.textFieldSecretInset
                y: LithicTokens.size.textFieldSecretInset
                small: true
                kind: "quiet"
                glyph: root.opened ? "expand_less" : "expand_more"
                accessibleName: root.opened ? Lithic.tr("Hide colours") : Lithic.tr("Show colours")
                previewState: root.previewState === "focus" ? "focus" : ""
                onClicked: root.toggle()
            }
        }
    }

    Popover {
        id: popover
        parent: root.overlayParent ? root.overlayParent : root.parent
        anchorItem: hexField
        opener: disclosure
        previewEmbedded: root.previewOpen
        onOpened: {
            const row = swatchRepeater.count > 0 ? swatchRepeater.itemAt(Math.max(0, panel.currentSwatch)) : null;
            if (row)
                row.forceActiveFocus(Qt.TabFocusReason);
            else
                popoverHex.forceActiveFocus(Qt.TabFocusReason);
        }

        CutRect {
            id: panel

            readonly property int currentSwatch: {
                for (let index = 0; index < root.swatches.length; ++index) {
                    if (Qt.colorEqual(root.swatches[index].color, root.color))
                        return index;
                }
                return -1;
            }

            implicitWidth: LithicTokens.size.settingsControlMax
            implicitHeight: panelColumn.implicitHeight
            fill: Lithic.stone4
            strokeColor: Lithic.vein
            strokeWidth: LithicTokens.size.hairline
            cut: LithicTokens.cut.menu

            Column {
                id: panelColumn
                width: parent.width
                topPadding: LithicTokens.size.menuPaddingVertical

                Repeater {
                    id: swatchRepeater
                    model: root.swatches

                    delegate: T.AbstractButton {
                        id: swatchRow

                        required property var modelData
                        required property int index
                        readonly property bool isCurrent: index === panel.currentSwatch

                        width: panelColumn.width
                        height: LithicTokens.size.menuItem
                        hoverEnabled: true
                        focusPolicy: Qt.StrongFocus
                        text: modelData.label || ""
                        Accessible.role: Accessible.RadioButton
                        Accessible.name: text
                        Accessible.checked: isCurrent

                        onClicked: {
                            root.choose(modelData.color);
                            popover.close();
                        }
                        Keys.onReturnPressed: clicked()
                        Keys.onEnterPressed: clicked()
                        Keys.onDownPressed: nextItemInFocusChain(true).forceActiveFocus(Qt.TabFocusReason)
                        Keys.onUpPressed: nextItemInFocusChain(false).forceActiveFocus(Qt.BacktabFocusReason)

                        background: Item {
                            CutRect {
                                anchors.fill: parent
                                fill: swatchRow.down ? Lithic.pressRow(Lithic.stone4)
                                    : swatchRow.hovered ? Lithic.hoverPlane(Lithic.stone4) : "transparent"
                                ChiselColorBehavior on fill {
                                    duration: swatchRow.hovered ? LithicTokens.motion.hoverIn : LithicTokens.motion.hoverOut
                                }
                            }
                            FocusRing {
                                x: -outset
                                y: -outset
                                width: parent.width + outset * 2
                                height: parent.height + outset * 2
                                keyboardFocusVisible: swatchRow.visualFocus
                                cut: 0
                            }
                        }

                        contentItem: Item {
                            ColorSwatch {
                                id: rowSwatch
                                x: LithicTokens.size.menuInset
                                anchors.verticalCenter: parent.verticalCenter
                                width: LithicTokens.size.menuIcon
                                height: LithicTokens.size.menuIcon
                                color: swatchRow.modelData.color
                            }
                            Label {
                                anchors.left: rowSwatch.right
                                anchors.leftMargin: LithicTokens.size.menuIconGap
                                anchors.right: check.left
                                anchors.rightMargin: LithicTokens.size.menuShortcutGap
                                anchors.verticalCenter: parent.verticalCenter
                                text: swatchRow.text
                                typeRole: "label"
                            }
                            Glyph {
                                id: check
                                anchors.right: parent.right
                                anchors.rightMargin: LithicTokens.size.menuInset
                                anchors.verticalCenter: parent.verticalCenter
                                visible: swatchRow.isCurrent
                                name: "check"
                                size: LithicTokens.size.menuIcon
                                ink: Lithic.fg2
                            }
                        }
                    }
                }

                Item {
                    visible: root.swatches.length > 0
                    width: parent.width
                    height: LithicTokens.size.menuPaddingVertical * 2 + LithicTokens.size.hairline

                    Rectangle {
                        x: LithicTokens.size.menuDividerInset
                        anchors.verticalCenter: parent.verticalCenter
                        width: parent.width - LithicTokens.size.menuDividerInset * 2
                        height: LithicTokens.size.hairline
                        color: Lithic.vein
                    }
                }

                Column {
                    width: parent.width
                    leftPadding: LithicTokens.space.s3
                    rightPadding: LithicTokens.space.s3
                    bottomPadding: LithicTokens.space.s3
                    topPadding: root.swatches.length > 0 ? 0 : LithicTokens.space.s3
                    spacing: LithicTokens.space.s3

                    TextField {
                        id: popoverHex
                        width: parent.width - parent.leftPadding - parent.rightPadding
                        label: Lithic.tr("Hex")
                        mono: true
                        text: root.hex
                        onEditingFinished: root.commit(popoverHex)
                    }

                    Column {
                        visible: root.allowAlpha
                        width: parent.width - parent.leftPadding - parent.rightPadding
                        spacing: LithicTokens.space.s1

                        Label {
                            text: Lithic.tr("Opacity")
                            typeRole: "detail"
                        }

                        Item {
                            width: parent.width
                            height: alphaSlider.implicitHeight

                            Slider {
                                id: alphaSlider
                                anchors.left: parent.left
                                anchors.right: alphaValue.left
                                anchors.rightMargin: LithicTokens.space.s3
                                from: 0
                                to: 100
                                stepSize: 1
                                value: Math.round(Qt.color(root.color).a * 100)
                                accessibleName: Lithic.tr("Opacity")
                                onMoved: {
                                    const c = Qt.color(root.color);
                                    root.choose(Qt.rgba(c.r, c.g, c.b, value / 100));
                                }
                            }

                            Label {
                                id: alphaValue
                                anchors.right: parent.right
                                anchors.verticalCenter: parent.verticalCenter
                                text: Math.round(alphaSlider.value) + "%"
                                typeRole: "body"
                                numeric: true
                            }
                        }
                    }
                }
            }
        }
    }
}

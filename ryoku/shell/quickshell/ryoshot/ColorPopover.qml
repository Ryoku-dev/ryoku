pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "look"

Flyout {
    id: pop

    /** Externally driven current colour; seeds the HSV state on change. */
    property color selected: Look.sel

    /** Renders the eyedropper button in its armed (active) state. */
    property bool eyedropArmed: false

    /** Emitted on any swatch, square, hue, or hex commit. */
    signal picked(color c)

    /** The parent closes the popover and arms sampling in the overlay. */
    signal eyedropRequested()

    property real hue: 0
    property real sat: 1
    property real val: 1
    property bool syncing: false

    readonly property color current: Qt.hsva(pop.hue, pop.sat, pop.val, 1)
    readonly property real span: Math.round(200 * pop.u)
    // the colour field's own corner: square in Lithic, the design's small round
    readonly property real fieldRadius: Look.lithic ? 0 : (Look.material ? 12 : Look.adwaita ? 8 : 4) * pop.u

    /**
     * Pulls hue/sat/val from an incoming colour. Greys read back a NaN/-1 hue
     * from Qt, so the prior hue is kept to avoid the marker jumping to red.
     */
    function syncFrom(c) {
        pop.syncing = true;
        if (c.hsvHue >= 0) pop.hue = c.hsvHue;
        pop.sat = c.hsvSaturation;
        pop.val = c.hsvValue;
        pop.syncing = false;
    }

    onSelectedChanged: if (!pop.syncing) pop.syncFrom(pop.selected)
    Component.onCompleted: pop.syncFrom(pop.selected)

    /** Sets sat/val from a point inside the square and emits the new colour. */
    function pickSquare(px, py, w, h) {
        pop.sat = Math.max(0, Math.min(1, px / w));
        pop.val = Math.max(0, Math.min(1, 1 - py / h));
        pop.emitPick();
    }

    /** Sets hue from a point along the strip and emits the new colour. */
    function pickHue(px, w) {
        pop.hue = Math.max(0, Math.min(1, px / w));
        pop.emitPick();
    }

    function emitPick() {
        pop.syncing = true;
        pop.picked(pop.current);
        pop.syncing = false;
    }

    /** Two-digit hex for a 0..1 channel, used to render the #rrggbb readout. */
    function hex2(f) {
        var n = Math.round(f * 255);
        var s = n.toString(16);
        return s.length === 1 ? "0" + s : s;
    }

    readonly property string hexText:
        "#" + hex2(current.r) + hex2(current.g) + hex2(current.b)

    Column {
        width: pop.span
        spacing: Math.round(10 * pop.u)

        Item {
            visible: Look.lithic
            width: parent.width
            height: Math.round(12 * pop.u)
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("Colour").toUpperCase()
                color: Look.fg3
                font.family: Look.mono
                font.pixelSize: Math.round(9 * pop.u)
                font.weight: Font.Bold
                font.letterSpacing: Look.tracking * pop.u
                renderType: Text.CurveRendering
            }
            Text {
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                text: "1–8"
                color: Look.fg3
                font.family: Look.mono
                font.pixelSize: Math.round(9 * pop.u)
                renderType: Text.CurveRendering
            }
        }

        Rectangle {
            id: square
            width: parent.width
            height: Math.round(120 * pop.u)
            radius: pop.fieldRadius
            clip: true
            color: Qt.hsva(pop.hue, 1, 1, 1)

            Rectangle {
                anchors.fill: parent
                radius: parent.radius
                gradient: Gradient {
                    orientation: Gradient.Horizontal
                    GradientStop { position: 0; color: Qt.rgba(1, 1, 1, 1) }
                    GradientStop { position: 1; color: Qt.rgba(1, 1, 1, 0) }
                }
            }
            Rectangle {
                anchors.fill: parent
                radius: parent.radius
                gradient: Gradient {
                    orientation: Gradient.Vertical
                    GradientStop { position: 0; color: Qt.rgba(0, 0, 0, 0) }
                    GradientStop { position: 1; color: Qt.rgba(0, 0, 0, 1) }
                }
            }

            // the picked point: a ring in Lithic's cut and the designs' round
            CutRect {
                visible: Look.lithic
                width: Math.round(12 * pop.u)
                height: width
                x: pop.sat * square.width - width / 2
                y: (1 - pop.val) * square.height - height / 2
                design: "lithic"
                cut: 3 * pop.u
                strokeColor: "white"
                strokeWidth: Math.max(1, Math.round(2 * pop.u))
            }
            Rectangle {
                visible: !Look.lithic
                width: Math.round(14 * pop.u)
                height: width
                radius: width / 2
                x: pop.sat * square.width - width / 2
                y: (1 - pop.val) * square.height - height / 2
                color: pop.current
                border.color: "white"
                border.width: Math.max(1, Math.round(2 * pop.u))
            }

            TapHandler {
                onTapped: (p) => pop.pickSquare(p.position.x, p.position.y, square.width, square.height)
            }
            DragHandler {
                target: null
                onCentroidChanged: if (active) pop.pickSquare(centroid.position.x, centroid.position.y, square.width, square.height)
            }
        }

        Rectangle {
            id: hueStrip
            width: parent.width
            height: Math.round(12 * pop.u)
            radius: Look.lithic ? 0 : height / 2
            gradient: Gradient {
                orientation: Gradient.Horizontal
                GradientStop { position: 0.000; color: Qt.hsva(0.000, 1, 1, 1) }
                GradientStop { position: 0.167; color: Qt.hsva(0.167, 1, 1, 1) }
                GradientStop { position: 0.333; color: Qt.hsva(0.333, 1, 1, 1) }
                GradientStop { position: 0.500; color: Qt.hsva(0.500, 1, 1, 1) }
                GradientStop { position: 0.667; color: Qt.hsva(0.667, 1, 1, 1) }
                GradientStop { position: 0.833; color: Qt.hsva(0.833, 1, 1, 1) }
                GradientStop { position: 1.000; color: Qt.hsva(1.000, 1, 1, 1) }
            }

            Rectangle {
                width: Math.round((Look.lithic ? 4 : 16) * pop.u)
                height: Look.lithic ? parent.height + 6 * pop.u : width
                anchors.verticalCenter: parent.verticalCenter
                radius: Look.lithic ? 0 : width / 2
                x: pop.hue * hueStrip.width - width / 2
                color: Look.lithic ? "transparent" : Qt.hsva(pop.hue, 1, 1, 1)
                border.color: "white"
                border.width: Math.max(1, Math.round(2 * pop.u))
            }

            TapHandler {
                onTapped: (p) => pop.pickHue(p.position.x, hueStrip.width)
            }
            DragHandler {
                target: null
                onCentroidChanged: if (active) pop.pickHue(centroid.position.x, hueStrip.width)
            }
        }

        Row {
            width: parent.width
            spacing: (width - Look.swatches.length * Math.round(20 * pop.u)) / (Look.swatches.length - 1)
            Repeater {
                model: Look.swatches
                Chip {
                    required property var modelData
                    u: pop.u
                    size: 20
                    ink: modelData
                    selected: Qt.colorEqual(pop.selected, modelData)
                    onTapped: pop.picked(modelData)
                }
            }
        }

        Row {
            width: parent.width
            spacing: Math.round(8 * pop.u)

            GlyphButton {
                id: eyedrop
                u: pop.u
                cellWidth: 28
                cellHeight: 28
                iconSize: 16
                icon: "eyedropper"
                checked: pop.eyedropArmed
                tip: I18n.tr("Pick a colour from the shot")
                keyHint: "i"
                onClicked: pop.eyedropRequested()
            }

            Item {
                id: hexBox
                width: parent.width - eyedrop.width - parent.spacing
                height: eyedrop.height

                readonly property bool editing: hexField.activeFocus
                property real flash: 0
                readonly property color rest: hexBox.editing ? Look.sel : Look.lithic ? Look.vein : Lithic.alpha(Look.fg, 0.14)
                readonly property color edge: Lithic.mix(hexBox.rest, Look.alert, hexBox.flash)

                CutRect {
                    visible: Look.lithic
                    anchors.fill: parent
                    design: "lithic"
                    cut: 3 * pop.u
                    fill: Look.well
                    strokeColor: hexBox.edge
                    strokeWidth: Math.max(1, Math.round(pop.u))
                }
                Rectangle {
                    visible: !Look.lithic
                    anchors.fill: parent
                    radius: (Look.material ? 8 : Look.adwaita ? 6 : 4) * pop.u
                    color: Lithic.alpha(Look.fg, Look.fluent ? 0.05 : 0.08)
                    border.width: Look.adwaita ? 0 : Math.max(1, Math.round(pop.u))
                    border.color: Look.material || Look.fluent ? hexBox.edge : "transparent"
                }
                // Fluent's text box marks focus with an accent rule along its foot
                Rectangle {
                    visible: Look.fluent
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.bottom: parent.bottom
                    anchors.leftMargin: 2 * pop.u
                    anchors.rightMargin: 2 * pop.u
                    height: Math.max(1, Math.round((hexBox.editing ? 2 : 1) * pop.u))
                    color: hexBox.editing ? Look.sel : Look.fg3
                }

                SequentialAnimation {
                    id: hexFlash
                    NumberAnimation { target: hexBox; property: "flash"; to: 1; duration: 90 }
                    NumberAnimation { target: hexBox; property: "flash"; to: 0; duration: 240 }
                }

                Rectangle {
                    id: preview
                    anchors.verticalCenter: parent.verticalCenter
                    x: Math.round(6 * pop.u)
                    width: Math.round(14 * pop.u)
                    height: width
                    radius: Look.lithic ? 0 : Look.fluent ? 3 * pop.u : width / 2
                    color: pop.current
                    border.color: Lithic.alpha(Look.fg, 0.25)
                    border.width: Math.max(1, Math.round(pop.u))
                }

                TextInput {
                    id: hexField
                    anchors.left: preview.right
                    anchors.right: parent.right
                    anchors.top: parent.top
                    anchors.bottom: parent.bottom
                    anchors.leftMargin: Math.round(8 * pop.u)
                    anchors.rightMargin: Math.round(8 * pop.u)
                    verticalAlignment: TextInput.AlignVCenter
                    color: Look.fg
                    font.family: Look.mono
                    font.pixelSize: Math.round(11.5 * pop.u)
                    font.capitalization: Look.lithic ? Font.AllUppercase : Font.MixedCase
                    selectByMouse: true
                    selectionColor: Look.sel
                    selectedTextColor: Look.onSel
                    text: pop.hexText
                    maximumLength: 7
                    onActiveFocusChanged: if (activeFocus) selectAll()

                    /**
                     * Accepts a #rrggbb (or rrggbb) string on Enter or focus
                     * loss; a value Qt cannot parse reverts to the current
                     * colour so a half-typed hex never snaps the picker.
                     */
                    function commit() {
                        var t = text.trim();
                        if (t.length > 0 && t[0] !== "#") t = "#" + t;
                        if (/^#[0-9a-fA-F]{3}$/.test(t))
                            t = "#" + t[1] + t[1] + t[2] + t[2] + t[3] + t[3];
                        if (/^#[0-9a-fA-F]{6}$/.test(t)) {
                            pop.picked(t);
                            pop.syncFrom(t);
                        } else {
                            hexFlash.start();
                        }
                        text = Qt.binding(function () { return pop.hexText; });
                    }
                    onEditingFinished: commit()
                    Keys.onReturnPressed: commit()
                    Keys.onEnterPressed: commit()
                }
            }
        }
    }
}

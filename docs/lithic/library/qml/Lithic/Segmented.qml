import QtQuick
import QtQuick.Templates as T

T.Control {
    id: root

    property var model: []
    property int currentIndex: 0
    property string previewState: ""
    property string accessibleName: Lithic.tr("Options")

    signal activated(int index)

    readonly property var _labels: {
        const result = [];
        for (let index = 0; index < model.length; ++index) {
            const entry = model[index];
            result.push(typeof entry === "string" ? entry : String(entry.label || ""));
        }
        return result;
    }
    readonly property real _widest: {
        let widest = 0;
        for (let index = 0; index < _labels.length; ++index)
            widest = Math.max(widest, metrics.advanceWidth(_labels[index]));
        return Math.ceil(widest);
    }
    readonly property real _inset: LithicTokens.size.segmentInset
    readonly property real _segmentWidth: _labels.length > 0 ? Math.max(0, width - _inset * 2) / _labels.length : 0
    readonly property int _previewIndex: currentIndex === 0 ? 1 : 0
    property real _shownIndex: currentIndex

    focusPolicy: Qt.StrongFocus
    implicitWidth: _labels.length * (_widest + LithicTokens.space.s4 * 2) + _inset * 2
    implicitHeight: LithicTokens.size.segmentHeight
    opacity: enabled ? 1 : Lithic.disabledOpacity

    Accessible.role: Accessible.List
    Accessible.name: accessibleName

    ChiselNumberBehavior on _shownIndex {
        duration: LithicTokens.motion.select
    }

    function select(index) {
        if (!enabled || _labels.length === 0)
            return;
        const next = Math.max(0, Math.min(_labels.length - 1, index));
        if (next !== currentIndex) {
            currentIndex = next;
            activated(next);
        }
    }

    Keys.onPressed: function(event) {
        if (event.key === Qt.Key_Left || event.key === Qt.Key_Up)
            select(currentIndex - 1);
        else if (event.key === Qt.Key_Right || event.key === Qt.Key_Down)
            select(currentIndex + 1);
        else if (event.key === Qt.Key_Home)
            select(0);
        else if (event.key === Qt.Key_End)
            select(_labels.length - 1);
        else
            return;
        event.accepted = true;
    }

    FontMetrics {
        id: metrics
        font.family: Lithic.type.label.family
        font.pixelSize: Lithic.type.label.pixelSize
        font.weight: Lithic.type.label.weight
    }

    background: Item {
        FocusRing {
            x: -outset
            y: -outset
            width: parent.width + outset * 2
            height: parent.height + outset * 2
            keyboardFocusVisible: root.enabled && (root.visualFocus || root.previewState === "focus")
            cut: LithicTokens.cut.mid
        }

        CutRect {
            anchors.fill: parent
            fill: Lithic.field
            strokeColor: Lithic.vein
            strokeWidth: LithicTokens.size.hairline
            cut: LithicTokens.cut.mid
        }
    }

    contentItem: Item {
        CutRect {
            visible: root.currentIndex >= 0 && root.currentIndex < root._labels.length
            x: root._inset + Math.max(0, Math.min(root._labels.length - 1, root._shownIndex)) * root._segmentWidth
            y: root._inset
            width: root._segmentWidth
            height: Math.max(0, root.height - root._inset * 2)
            fill: Lithic.sel
            cut: LithicTokens.cut.segment
        }

        Repeater {
            model: root._labels

            delegate: T.AbstractButton {
                id: segment

                required property int index
                required property string modelData
                readonly property bool selected: index === root.currentIndex
                readonly property bool _hoverActive: enabled && !selected
                                                     && (hovered || (index === root._previewIndex && root.previewState === "hover"))
                readonly property bool _pressActive: enabled && !selected
                                                     && (pressed || (index === root._previewIndex && root.previewState === "pressed"))

                x: root._inset + index * root._segmentWidth
                y: root._inset
                width: root._segmentWidth
                height: Math.max(0, root.height - root._inset * 2)
                hoverEnabled: true
                focusPolicy: Qt.NoFocus
                enabled: root.enabled
                text: modelData

                Accessible.role: Accessible.RadioButton
                Accessible.name: modelData
                Accessible.checked: selected

                onClicked: {
                    root.forceActiveFocus();
                    root.select(index);
                }

                background: CutRect {
                    fill: segment._hoverActive ? Lithic.hoverInk(Lithic.fg2) : "transparent"
                    cut: LithicTokens.cut.segment + (segment._pressActive ? Lithic.pressCutGrow : 0)

                    ChiselColorBehavior on fill {
                        duration: segment._hoverActive ? LithicTokens.motion.hoverIn : LithicTokens.motion.hoverOut
                    }
                    ChiselNumberBehavior on cut { duration: LithicTokens.motion.press }
                }

                contentItem: Label {
                    text: segment.modelData
                    typeRole: "label"
                    ink: segment.selected ? Lithic.onSel : Lithic.fg2
                    horizontalAlignment: Text.AlignHCenter
                    verticalAlignment: Text.AlignVCenter

                    transform: Translate {
                        y: segment._pressActive ? Lithic.pressSink : 0
                        ChiselNumberBehavior on y { duration: LithicTokens.motion.press }
                    }

                    ChiselColorBehavior on ink {
                        duration: LithicTokens.motion.select
                    }
                }
            }
        }
    }
}

import QtQuick
import QtQuick.Templates as T

T.AbstractButton {
    id: root

    property alias title: root.text
    property string status: ""
    // "", "light", "standard" or "heavy": what running the look costs
    property string resource: ""
    property url source: ""
    property bool current: false
    property bool selected: false
    property string unavailable: ""
    property bool busy: false
    property string actionText: ""
    property string actionKind: "line"
    property string actionGlyph: ""
    property bool focusVisible: false
    property string previewState: ""

    signal actionTriggered()

    readonly property bool _hoverActive: enabled && !selected && (hovered || previewState === "hover")
    readonly property bool _pressActive: enabled && (down || previewState === "pressed")
    readonly property bool _focusActive: visualFocus || focusVisible || previewState === "focus"
    readonly property bool _onSel: selected || current
    readonly property real _inset: LithicTokens.space.s1
    readonly property real _previewCut: Math.round(LithicTokens.cut.small - _inset * (2 - Math.SQRT2))
    readonly property real _previewWidth: Math.max(0, width - _inset * 2)
    readonly property real _previewHeight: Math.round(_previewWidth / LithicTokens.size.designPreviewRatio)
    readonly property real _textTop: _inset + _previewHeight
    readonly property color _baseFill: selected ? Lithic.sel : Lithic.slab
    readonly property color _ink: _onSel ? Lithic.onSel : Lithic.fg
    readonly property color _quietInk: _onSel ? Lithic.onSel2 : Lithic.fg2
    readonly property bool _showImage: source.toString().length > 0 && image.status === Image.Ready

    hoverEnabled: true
    focusPolicy: Qt.StrongFocus
    opacity: enabled ? 1 : Lithic.disabledOpacity
    implicitWidth: LithicTokens.size.catalogueTileWidthMin
    implicitHeight: _textTop + LithicTokens.space.s2 * 2 + Lithic.type.row.lineHeight
                    + LithicTokens.size.plaqueHeightSmall
    leftPadding: LithicTokens.space.s3
    rightPadding: LithicTokens.space.s3
    topPadding: _textTop + LithicTokens.space.s2
    bottomPadding: LithicTokens.space.s2

    Accessible.role: Accessible.Button
    Accessible.description: busy
                            ? Lithic.tr("%1, busy").arg(unavailable.length > 0 ? unavailable : status)
                            : unavailable.length > 0 ? unavailable : status
    Accessible.selected: selected
    Accessible.checkable: true
    Accessible.checked: current
    Accessible.name: text

    background: Item {
        FocusRing {
            x: -outset
            y: -outset
            width: root.width + outset * 2
            height: root.height + outset * 2
            keyboardFocusVisible: root._focusActive
            cut: LithicTokens.cut.small
        }

        CutRect {
            anchors.fill: parent
            cut: LithicTokens.cut.small
            fill: root._pressActive ? Lithic.pressRow(root._baseFill)
                : root._hoverActive ? Lithic.mix(root._baseFill, Lithic.fg, LithicTokens.alpha.hoverPlane)
                : root._baseFill
            strokeColor: Lithic.vein
            strokeWidth: root.selected ? 0 : LithicTokens.size.hairline

            ChiselColorBehavior on fill {
                duration: root._hoverActive ? LithicTokens.motion.hoverIn : LithicTokens.motion.hoverOut
            }
        }

        // the look in use: bone under the text block only
        CutRect {
            visible: root.current && !root.selected
            y: root._textTop
            width: root.width
            height: root.height - y
            fill: Lithic.sel
            cutTopLeft: 0
            cutTopRight: 0
            cutBottomLeft: LithicTokens.cut.small
            cutBottomRight: LithicTokens.cut.small
        }

        CutClip {
            x: root._inset
            y: root._inset
            width: root._previewWidth
            height: root._previewHeight
            cut: root._previewCut
            squareBottom: true
            opacity: root.unavailable.length > 0 ? Lithic.disabledOpacity : 1

            Rectangle {
                width: root._previewWidth
                height: root._previewHeight
                visible: !root._showImage
                color: Lithic.field

                Hatch {
                    anchors.fill: parent
                }
            }

            Image {
                id: image
                width: root._previewWidth
                height: root._previewHeight
                visible: root._showImage
                source: root.source
                fillMode: Image.PreserveAspectCrop
                asynchronous: true
                cache: true
                sourceSize.width: Math.ceil(root._previewWidth * Screen.devicePixelRatio)
                sourceSize.height: Math.ceil(root._previewHeight * Screen.devicePixelRatio)
            }
        }
    }

    contentItem: Item {
        Label {
            id: titleLabel
            x: 0
            y: 0
            width: parent.width - (markLabel.visible ? markLabel.implicitWidth + LithicTokens.space.s2 : 0)
            text: root.text
            typeRole: "row"
            ink: root._ink
            maximumLineCount: 1
        }

        Label {
            id: markLabel
            visible: text.length > 0
            anchors.right: parent.right
            anchors.baseline: titleLabel.baseline
            text: root.resource === "light" ? Lithic.tr("Light")
                : root.resource === "standard" ? Lithic.tr("Standard")
                : root.resource === "heavy" ? Lithic.tr("Heavy") : ""
            typeRole: "detail"
            ink: root._onSel ? root._quietInk : root.resource === "heavy" ? Lithic.amber : Lithic.fg2
        }

        Item {
            id: statusLine
            x: 0
            y: titleLabel.height
            width: parent.width
            height: LithicTokens.size.plaqueHeightSmall

            Working {
                id: working
                visible: root.busy
                active: root.busy
                size: Lithic.type.caption.lineHeight
                anchors.verticalCenter: parent.verticalCenter
            }

            // two caption lines fill the status line's height, so a narrow tile wraps its status, never cuts it
            Label {
                x: working.visible ? working.width + LithicTokens.space.s2 : 0
                anchors.verticalCenter: parent.verticalCenter
                width: Math.max(0, (action.visible ? action.x - LithicTokens.space.s2 : parent.width) - x)
                text: root.unavailable.length > 0 ? root.unavailable : root.status
                typeRole: "caption"
                ink: root._quietInk
                wrapMode: Text.Wrap
                maximumLineCount: 2
            }

            Plaque {
                id: action
                visible: root.actionText.length > 0
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                small: true
                // gold ink on bone reads poorly; only the gold action keeps its kind there
                kind: root._onSel && root.actionKind !== "gold" ? "on" : root.actionKind
                text: root.actionText
                glyph: root.actionGlyph
                onClicked: root.actionTriggered()
            }
        }
    }
}

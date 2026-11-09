import QtQuick
import QtQuick.Templates as T

T.Control {
    id: root

    property string kind: "gold"
    property bool small: false
    property string glyph: ""
    property string text: ""
    property bool busy: false
    property bool menuOpen: false
    property string previewState: ""
    property string morePreviewState: ""
    property string accessibleName: text
    property string moreAccessibleName: "More actions"

    signal mainClicked()
    signal moreClicked()

    readonly property real controlHeight: small ? LithicTokens.size.plaqueHeightSmall : LithicTokens.size.plaqueHeight
    readonly property real controlCut: small ? LithicTokens.cut.small : LithicTokens.cut.mid
    readonly property bool _transparent: kind === "line" || kind === "quiet" || kind === "dangerLine"
    readonly property color ink: _inkForKind()

    implicitWidth: mainSegment.implicitWidth + LithicTokens.size.splitPlaqueGap + moreSegment.implicitWidth
    implicitHeight: controlHeight
    opacity: enabled ? 1 : Lithic.disabledOpacity

    Accessible.role: Accessible.Grouping
    Accessible.name: accessibleName.length > 0 ? accessibleName : text

    function _baseFill() {
        if (kind === "on")
            return Lithic.sel;
        if (kind === "danger")
            return Lithic.jasper;
        if (_transparent)
            return "transparent";
        return Lithic.gold;
    }

    function _inkForKind() {
        if (kind === "on")
            return Lithic.onSel;
        if (kind === "danger")
            return Lithic.inkOn(Lithic.jasper);
        if (kind === "dangerLine")
            return Lithic.jasper;
        if (kind === "line")
            return Lithic.goldInk;
        if (kind === "quiet")
            return Lithic.fg2;
        return Lithic.onGold;
    }

    function _strokeColor() {
        if (kind === "line")
            return Lithic.goldInk;
        if (kind === "dangerLine")
            return Lithic.jasper;
        return Lithic.vein;
    }

    component Segment: T.AbstractButton {
        id: segment

        property bool moreSegment: false
        property string segmentPreviewState: ""
        property string segmentGlyph: ""
        property string segmentText: ""
        property string segmentAccessibleName: segmentText
        property bool segmentBusy: false

        readonly property bool _hoverActive: enabled && (hovered || segmentPreviewState === "hover")
        readonly property bool _pressActive: enabled && (pressed || segmentPreviewState === "pressed")
        readonly property bool _focusActive: enabled && (visualFocus || segmentPreviewState === "focus")

        hoverEnabled: true
        focusPolicy: Qt.StrongFocus
        leftPadding: moreSegment ? 0 : (root.small ? LithicTokens.size.plaqueInsetSmall : LithicTokens.size.plaqueInset)
        rightPadding: leftPadding
        implicitWidth: moreSegment
                       ? LithicTokens.size.splitPlaqueMoreWidth
                       : implicitContentWidth + leftPadding + rightPadding
        implicitHeight: root.controlHeight

        Accessible.role: Accessible.Button
        Accessible.name: segmentAccessibleName

        background: Item {
            FocusRing {
                x: -outset
                y: -outset
                width: parent.width + outset * 2
                height: parent.height + outset * 2
                keyboardFocusVisible: segment._focusActive
                cut: root.controlCut
            }

            CutRect {
                anchors.fill: parent
                fill: segment._hoverActive
                      ? (root._transparent ? Lithic.hoverInk(root.ink) : Lithic.hoverFill(root._baseFill()))
                      : root._baseFill()
                strokeColor: root._strokeColor()
                strokeWidth: root._transparent ? LithicTokens.size.hairline : 0
                cutTopLeft: segment.moreSegment ? 0 : root.controlCut + (segment._pressActive ? Lithic.pressCutGrow : 0)
                cutBottomLeft: cutTopLeft
                cutTopRight: segment.moreSegment ? root.controlCut + (segment._pressActive ? Lithic.pressCutGrow : 0) : 0
                cutBottomRight: cutTopRight

                ChiselColorBehavior on fill {
                    duration: segment._hoverActive ? LithicTokens.motion.hoverIn : LithicTokens.motion.hoverOut
                }
            }
        }

        contentItem: Item {
            implicitWidth: segmentRow.implicitWidth
            implicitHeight: segmentRow.implicitHeight

            Row {
                id: segmentRow
                anchors.centerIn: parent
                spacing: (working.visible || glyphItem.visible) && labelItem.visible
                         ? (root.small ? LithicTokens.size.plaqueGapSmall : LithicTokens.size.plaqueGap)
                         : 0

                transform: Translate {
                    y: segment._pressActive ? Lithic.pressSink : 0
                    ChiselNumberBehavior on y { duration: LithicTokens.motion.press }
                }

                Working {
                    id: working
                    visible: segment.segmentBusy
                    active: segment.segmentBusy
                    size: root.small ? LithicTokens.size.plaqueIconSmall : LithicTokens.size.plaqueIcon
                }

                Glyph {
                    id: glyphItem
                    visible: !segment.segmentBusy && segment.segmentGlyph.length > 0
                    width: visible ? (root.small ? LithicTokens.size.plaqueIconSmall : LithicTokens.size.plaqueIcon) : 0
                    height: width
                    name: segment.segmentGlyph
                    ink: root.ink
                    size: root.small ? LithicTokens.size.plaqueIconSmall : LithicTokens.size.plaqueIcon
                }
                Label {
                    id: labelItem
                    visible: segment.segmentText.length > 0
                    text: segment.segmentText
                    typeRole: "label"
                    ink: root.ink
                }
            }
        }
    }

    contentItem: Row {
        spacing: LithicTokens.size.splitPlaqueGap

        Segment {
            id: mainSegment
            segmentText: root.text
            segmentGlyph: root.glyph
            segmentBusy: root.busy
            segmentPreviewState: root.previewState
            segmentAccessibleName: root.accessibleName
            enabled: root.enabled && !root.busy
            onClicked: root.mainClicked()
        }

        Segment {
            id: moreSegment
            moreSegment: true
            segmentGlyph: root.menuOpen ? "expand_less" : "expand_more"
            segmentPreviewState: root.morePreviewState
            segmentAccessibleName: root.moreAccessibleName
            enabled: root.enabled && !root.busy
            onClicked: root.moreClicked()
        }
    }
}

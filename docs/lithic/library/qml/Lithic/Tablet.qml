import QtQuick
import QtQuick.Templates as T

T.AbstractButton {
    id: root

    property alias title: root.text
    property string facts: ""
    property string claim: Lithic.tr("Idle")
    property string proof: ""
    property string tone: "idle"
    property real progress: 0
    property bool selected: false
    property bool compact: false
    property color pane: Lithic.stone1
    property Component mark: null
    property string actionText: ""
    property string actionKind: "line"
    property string actionGlyph: ""
    property bool actionBusy: false
    property string previewState: ""

    signal actionTriggered()
    signal primaryAction()

    readonly property bool stateHovered: enabled
        && (hovered || previewState === "hover")
    readonly property bool statePressed: enabled
        && (down || previewState === "pressed")
    readonly property bool stateFocused: visualFocus || previewState === "focus"
    readonly property bool actionVisible: actionText !== ""
        && (stateHovered || stateFocused)
    readonly property color titleInk: selected ? Lithic.onSel : Lithic.fg
    readonly property color quietInk: selected ? Lithic.onSel2 : Lithic.fg3
    readonly property color claimInk: selected
        ? Lithic.onSel
        : tone === "idle" ? Lithic.fg2 : Lithic.stone(tone)
    readonly property color baseFill: selected ? Lithic.sel : Lithic.stone2

    implicitWidth: LithicTokens.size.listMedium
    implicitHeight: compact
        ? LithicTokens.size.tabletHeightCompact
        : LithicTokens.size.tabletHeight
    hoverEnabled: enabled
    focusPolicy: Qt.StrongFocus
    opacity: enabled ? 1 : Lithic.disabledOpacity
    Accessible.name: text
    Accessible.description: facts

    background: CutRect {
        fill: root.statePressed
            ? Lithic.pressRow(root.baseFill)
            : root.stateHovered ? Lithic.hoverPlane(root.baseFill) : root.baseFill

        ChiselColorBehavior on fill {
            duration: root.stateHovered
                ? LithicTokens.motion.hoverIn
                : LithicTokens.motion.hoverOut
        }
    }

    contentItem: Item {
        Item {
            id: markHost
            x: LithicTokens.size.tabletInsetLeft
            anchors.verticalCenter: parent.verticalCenter
            width: LithicTokens.size.tabletMark
            height: LithicTokens.size.tabletMark

            Loader {
                anchors.fill: parent
                sourceComponent: root.mark
            }
        }

        Column {
            id: copy
            x: markHost.x + markHost.width + LithicTokens.size.tabletMarkGap
            anchors.verticalCenter: parent.verticalCenter
            width: Math.max(0, trailing.x - x - LithicTokens.space.s3)

            Label {
                width: parent.width
                text: root.text
                typeRole: "row"
                ink: root.titleInk
                maximumLineCount: 1
            }
            Label {
                width: parent.width
                text: root.facts
                typeRole: "caption"
                ink: root.quietInk
                maximumLineCount: 1
            }
        }

        Item {
            id: trailing
            anchors.right: parent.right
            anchors.rightMargin: LithicTokens.size.tabletInsetRight
            anchors.verticalCenter: parent.verticalCenter
            width: Math.max(LithicTokens.size.tabletTrailingMin,
                            Math.max(claimBlock.implicitWidth, action.implicitWidth))
            height: parent.height

            Column {
                id: claimBlock
                visible: !root.actionVisible
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter

                Label {
                    anchors.right: parent.right
                    text: root.claim
                    typeRole: "label"
                    ink: root.claimInk
                    horizontalAlignment: Text.AlignRight
                }
                Label {
                    anchors.right: parent.right
                    text: root.proof
                    typeRole: "caption"
                    numeric: true
                    ink: root.quietInk
                    horizontalAlignment: Text.AlignRight
                }
            }

            Plaque {
                id: action
                visible: root.actionVisible
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                text: root.actionText
                kind: root.actionKind
                glyph: root.actionGlyph
                small: true
                busy: root.actionBusy
                previewState: root.previewState
                onClicked: root.actionTriggered()
            }
        }
    }

    Notch {
        anchors.left: parent.left
        anchors.top: parent.top
        size: LithicTokens.cut.notch
        fill: root.tone === "idle" ? root.pane : Lithic.stone(root.tone)
    }

    Blade {
        visible: root.tone === "becoming"
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        height: LithicTokens.size.tabletProgress
        fraction: root.progress
        track: "transparent"
        fill: Lithic.lapis
    }

    FocusRing {
        x: -outset
        y: -outset
        width: root.width + outset * 2
        height: root.height + outset * 2
        keyboardFocusVisible: root.stateFocused
        cut: 0
    }

    TapHandler {
        acceptedButtons: Qt.LeftButton
        onDoubleTapped: root.primaryAction()
    }
    // Space selects; Return selects and opens, like a double tap and the web list's Enter
    Keys.onReturnPressed: event => root._open(event)
    Keys.onEnterPressed: event => root._open(event)
    function _open(event) {
        if (!visible) {
            event.accepted = false
            return
        }
        click()
        primaryAction()
    }
}

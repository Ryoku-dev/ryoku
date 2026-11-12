import QtQuick
import QtQuick.Templates as T

T.AbstractButton {
    id: root

    property alias title: root.text
    property string detail: ""
    property bool changed: false
    property string unavailable: ""
    property bool highlighted: false
    property bool activatable: false
    property bool first: false
    property color plane: Lithic.slab
    property string previewState: ""
    default property alias control: controlHost.data

    signal resetRequested()
    signal activated()

    readonly property bool _available: unavailable.length === 0
    readonly property real _gap: first ? 0 : LithicTokens.size.settingsRowGap
    readonly property bool _hoverActive: activatable && _available && (hovered || previewState === "hover")
    readonly property bool _pressActive: activatable && _available && (down || previewState === "pressed")
    // the ring follows focus on any row that takes it: activatable rows, and rows a page makes focusable
    readonly property bool _focusActive: focusPolicy !== Qt.NoFocus && (visualFocus || previewState === "focus")
    readonly property bool _focusWithin: {
        let item = Window.activeFocusItem;
        while (item) {
            if (item === root)
                return true;
            item = item.parent;
        }
        return false;
    }
    readonly property bool _resetShown: hovered || _focusWithin
                                        || previewState === "hover" || previewState === "focus"
    readonly property real _resetSlot: changed ? LithicTokens.size.iconPlaqueSmall + LithicTokens.space.s2 : 0
    readonly property real _textWidth: Math.max(0, width - controlHost.width - _resetSlot
                                                - (controlHost.width > 0 ? LithicTokens.space.s4 : 0))
    readonly property real _boxHeight: Math.max(LithicTokens.size.factsRowHeight,
                                                copy.implicitHeight, controlHost.height,
                                                changed ? LithicTokens.size.iconPlaqueSmall : 0)
    readonly property color _hoverFill: Lithic.mix(plane, Lithic.fg, LithicTokens.alpha.hoverPlane)
    property real _spot: 0

    hoverEnabled: true
    focusPolicy: activatable ? Qt.StrongFocus : Qt.NoFocus
    implicitWidth: LithicTokens.size.settingsMediumMax
    implicitHeight: _gap + _boxHeight
    leftPadding: 0
    rightPadding: 0
    topPadding: _gap
    bottomPadding: 0

    Accessible.role: activatable ? Accessible.Button : Accessible.Grouping
    Accessible.name: text
    Accessible.description: _available ? detail : unavailable

    onClicked: {
        if (activatable && _available)
            activated();
    }
    // only a row that acts takes Return; a key from a field inside the row goes on to the host
    Keys.onReturnPressed: event => root._returnPressed(event)
    Keys.onEnterPressed: event => root._returnPressed(event)
    function _returnPressed(event) {
        if (activeFocus && activatable && _available)
            activated();
        else
            event.accepted = false;
    }

    // the search spotlight shows once and fades; reduced motion holds it until the host clears it
    onHighlightedChanged: {
        spotFade.stop();
        if (!highlighted) {
            if (Lithic.reduceMotion)
                _spot = 0;
            return;
        }
        _spot = 1;
        if (!Lithic.reduceMotion)
            spotFade.start();
    }
    Component.onCompleted: if (highlighted) highlightedChanged()

    // a reset hides its own button and a hidden item keeps the keys, so they move on to the row's control
    onChangedChanged: if (!changed && resetPlaque.activeFocus) _leaveReset()
    function _leaveReset() {
        const reason = resetPlaque.visualFocus ? Qt.TabFocusReason : Qt.OtherFocusReason;
        const next = resetPlaque.nextItemInFocusChain(true);
        const target = next && _within(next, controlHost) ? next
            : focusPolicy !== Qt.NoFocus ? root : next;
        if (!target || target === resetPlaque)
            return;
        target.forceActiveFocus(reason);
        if (target.focusReason !== undefined)
            target.focusReason = reason;
    }
    function _within(item, ancestor) {
        for (let it = item; it; it = it.parent) {
            if (it === ancestor)
                return true;
        }
        return false;
    }

    NumberAnimation {
        id: spotFade
        target: root
        property: "_spot"
        from: 1
        to: 0
        duration: LithicTokens.motion.highlight
        easing.type: Easing.BezierSpline
        easing.bezierCurve: Lithic.curve
    }

    background: Item {
        Rectangle {
            visible: !root.first
            x: 0
            y: Math.floor((root._gap - LithicTokens.size.hairline) / 2)
            width: root.width
            height: LithicTokens.size.hairline
            color: Lithic.vein
        }

        CutRect {
            id: rowPlane
            x: -LithicTokens.space.s2
            y: root._gap - LithicTokens.space.s1
            width: root.width + LithicTokens.space.s2 * 2
            height: root._boxHeight + LithicTokens.space.s1 * 2
            cut: LithicTokens.cut.small
            fill: root._pressActive ? Lithic.pressRow(root._hoverFill)
                : root._hoverActive ? root._hoverFill
                : Lithic.alpha(root._hoverFill, root.previewState === "highlight" ? 1 : root._spot)

            ChiselColorBehavior on fill {
                enabled: !spotFade.running
                duration: root._hoverActive ? LithicTokens.motion.hoverIn : LithicTokens.motion.hoverOut
            }

            FocusRing {
                x: -outset
                y: -outset
                width: rowPlane.width + outset * 2
                height: rowPlane.height + outset * 2
                keyboardFocusVisible: root._focusActive
                cut: rowPlane.cut
            }
        }
    }

    contentItem: Item {
        implicitHeight: root._boxHeight

        Column {
            id: copy
            x: 0
            anchors.verticalCenter: parent.verticalCenter
            width: root._textWidth
            spacing: LithicTokens.size.settingsDetailGap

            // the changed tag sits beside a title that fits on one line with it, else on its own line below
            Item {
                readonly property bool _tagInline: titleLabel.implicitWidth + LithicTokens.space.s3
                                                   + changedMark.implicitWidth <= width
                width: parent.width
                height: titleLabel.height + (root.changed && !_tagInline ? changedMark.height : 0)

                Label {
                    id: titleLabel
                    width: Math.min(implicitWidth, parent.width)
                    text: root.text
                    typeRole: "row"
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }

                Label {
                    id: changedMark
                    visible: root.changed
                    x: parent._tagInline ? titleLabel.contentWidth + LithicTokens.space.s3 : 0
                    y: parent._tagInline ? titleLabel.baselineOffset - baselineOffset : titleLabel.height
                    text: Lithic.tr("Changed")
                    typeRole: "detail"
                    inkRole: "fg3"
                    elide: Text.ElideNone
                }
            }

            Label {
                width: parent.width
                visible: text.length > 0
                text: root._available ? root.detail : root.unavailable
                typeRole: "detail"
                inkRole: root._available ? "fg2" : "fg3"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
        }

        IconPlaque {
            id: resetPlaque
            visible: root.changed
            anchors.right: controlHost.left
            anchors.rightMargin: controlHost.width > 0 ? LithicTokens.space.s2 : 0
            anchors.verticalCenter: parent.verticalCenter
            small: true
            kind: "quiet"
            glyph: "restart_alt"
            accessibleName: Lithic.tr("Reset %1").arg(root.text)
            enabled: root._available
            opacity: (root._resetShown ? 1 : 0) * (enabled ? 1 : Lithic.disabledOpacity)
            onClicked: root.resetRequested()

            ChiselNumberBehavior on opacity {
                duration: root._resetShown ? LithicTokens.motion.hoverIn : LithicTokens.motion.hoverOut
            }
        }

        Row {
            id: controlHost
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            spacing: LithicTokens.space.s2
            enabled: root._available
        }
    }
}

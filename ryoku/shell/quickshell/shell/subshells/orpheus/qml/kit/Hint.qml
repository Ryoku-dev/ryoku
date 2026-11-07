import QtQuick
import Quickshell

Item {
    id: root

    property Item target: null
    property string text: ""
    property bool shown: false
    property bool _revealed: false
    // A card can hide the hint after its delay has fired; closing the card must
    // not reveal that stale hover until the pointer leaves the module.
    property bool _blockedUntilLeave: false
    property real _restingY: 0
    property Item _host: null
    property Item _geometryRoot: null
    property Item _suppressor: null
    readonly property bool _suppressed: _suppressor !== null && _suppressor.hintsSuppressed === true
    readonly property bool _showing: _revealed && text.length > 0
        && !_suppressed && !_blockedUntilLeave
    readonly property var _hostWindow: target ? target.QsWindow.window : null

    function adoptHost() {
        if (!target)
            return;
        let owner = target;
        while (owner && owner.hintsSuppressed === undefined)
            owner = owner.parent;
        _suppressor = owner || null;
        let top = target;
        while (top.parent && top.parent.parent)
            top = top.parent;
        if (_host !== top) {
            _host = top;
            parent = top;
        }
        let item = target;
        while (item && item.parent && item.parent !== _host)
            item = item.parent;
        _geometryRoot = item;
    }

    function reposition() {
        if (!target || !parent)
            return;
        const point = target.mapToItem(parent, target.width / 2, target.height);
        const edge = Style.s1;
        const desiredX = point.x - width / 2;
        const desiredY = Math.max(Style.barH + Style.s1, point.y + Style.s2);
        x = Math.max(edge, Math.min(parent.width - width - edge, desiredX));
        _restingY = Math.max(edge, Math.min(parent.height - height - edge, desiredY));
    }

    z: 10000
    implicitWidth: hintLabel.implicitWidth + Style.s4
    implicitHeight: hintLabel.implicitHeight + Style.s3
    width: implicitWidth
    height: implicitHeight
    y: _restingY + (_showing ? 0 : Style.s1)
    opacity: _showing ? 1 : 0
    visible: _showing || opacity > 0
    enabled: false

    onWidthChanged: {
        if (shown)
            reposition();
    }
    onShownChanged: {
        if (!shown) {
            delay.stop();
            _revealed = false;
            _blockedUntilLeave = false;
        } else if (text.length > 0 && !_suppressed && !_blockedUntilLeave) {
            adoptHost();
            reposition();
            delay.restart();
        }
    }
    on_SuppressedChanged: {
        if (_suppressed && shown) {
            delay.stop();
            _revealed = false;
            _blockedUntilLeave = true;
        }
    }
    onTextChanged: {
        if (shown && !_suppressed && !_blockedUntilLeave) {
            _revealed = false;
            delay.restart();
        }
    }
    Component.onCompleted: {
        if (shown && text.length > 0 && !_suppressed && !_blockedUntilLeave) {
            adoptHost();
            reposition();
            delay.start();
        }
    }

    Timer {
        id: delay
        interval: 500
        repeat: false
        onTriggered: {
            if (root.shown && root.text.length > 0
                    && !root._suppressed && !root._blockedUntilLeave) {
                root.reposition();
                root._revealed = true;
            }
        }
    }

    Connections {
        target: root.target
        enabled: root.shown && root.target !== null
        function onXChanged() { root.reposition(); }
        function onYChanged() { root.reposition(); }
        function onWidthChanged() { root.reposition(); }
        function onHeightChanged() { root.reposition(); }
    }

    Connections {
        target: root._geometryRoot
        enabled: root.shown && root._geometryRoot !== null
        function onXChanged() { root.reposition(); }
        function onYChanged() { root.reposition(); }
        function onWidthChanged() { root.reposition(); }
        function onHeightChanged() { root.reposition(); }
    }

    Connections {
        target: root._hostWindow
        enabled: root.shown && root._hostWindow !== null
        function onWidthChanged() { root.reposition(); }
        function onHeightChanged() { root.reposition(); }
    }

    Behavior on opacity {
        enabled: !Style.still
        NumberAnimation { duration: Style.effects; easing.type: Easing.OutCubic }
    }
    Behavior on y {
        enabled: !Style.still
        NumberAnimation { duration: Style.snap; easing.type: Easing.OutCubic }
    }

    Rectangle {
        anchors.fill: parent
        radius: Style.radius
        color: Style.base
        border.width: Style.border
        border.color: Style.lineStrong
    }

    Label {
        id: hintLabel
        anchors.centerIn: parent
        text: root.text
        role: "label"
        tone: "ink"
    }
}

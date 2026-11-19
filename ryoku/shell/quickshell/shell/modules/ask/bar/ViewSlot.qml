pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import "../look"

// One place in the bar's view stack. It builds its view when the view enters
// the stack, feeds it the contract's inputs, and moves it in and out the way
// the design does: Lithic wipes a blade across, Material slides on a shared
// axis, Adwaita slides a little and fades, Fluent drills in from 96%.
Item {
    id: root

    required property real s
    property string name: ""
    // in the stack (alive), and on top of it (shown)
    property bool live: false
    property bool current: false
    // +1 when the last move was a push, -1 a pop
    property int direction: 1
    property bool barActive: false
    property bool animate: true
    property string fieldText: ""
    property string heldText: ""
    property var args: ({})
    property real maximumHeight: 480 * s
    // views named by file drop in later; they take their inputs by name
    property url source: ""
    property Component sourceComponent: null
    property var gifts: ({})

    readonly property Item view: loader.item as Item
    readonly property bool failed: loader.status === Loader.Error
    readonly property real side: root.current ? root.direction : -root.direction

    property real t: root.current ? 1 : 0
    Behavior on t {
        enabled: root.animate
        NumberAnimation {
            duration: Look.dur(root.current || Look.viewWipe ? Look.viewMs : Math.round(Look.viewMs * 0.55))
            easing.type: Easing.BezierSpline
            easing.bezierCurve: root.current ? Look.arrive : Look.leave
        }
    }

    // a push wipes in from the left edge, the way back from the right
    readonly property bool wiping: Look.viewWipe && root.current && root.t < 0.999
    readonly property bool fromRight: root.direction < 0

    visible: root.t > 0.001 || root.current
    z: root.current ? 2 : 1
    clip: root.wiping
    x: root.wiping && root.fromRight && root.parent ? root.parent.width - root.width : 0
    width: !root.parent ? 0 : Look.viewWipe && root.current ? Math.ceil(root.parent.width * Math.min(1, root.t)) : root.parent.width
    height: root.parent ? root.parent.height : 0
    opacity: Look.viewWipe ? 1 : Math.max(0, Math.min(1, root.current ? root.t * 1.4 : root.t * 2 - 1))

    function give() {
        const item = loader.item;
        if (!item)
            return;
        for (var key in root.gifts) {
            if (key in item)
                item[key] = root.gifts[key];
        }
    }
    onGiftsChanged: root.give()

    onLiveChanged: {
        if (root.source.toString().length === 0)
            return;
        if (root.live && loader.status === Loader.Null)
            loader.setSource(root.source, { s: root.s });
    }

    // the wipe covers the view it replaces
    Rectangle {
        x: -root.x
        width: root.parent ? root.parent.width : 0
        height: parent.height
        color: Look.frame
        visible: Look.viewWipe
    }

    Loader {
        id: loader
        width: root.parent ? root.parent.width : 0
        x: Look.viewTravel * root.s * (1 - Math.min(1, root.t)) * root.side - root.x
        scale: Look.viewScale + (1 - Look.viewScale) * Math.min(1, root.t)
        transformOrigin: Item.Top
        active: root.live || root.t > 0.001
        sourceComponent: root.source.toString().length === 0 ? root.sourceComponent : undefined
        onLoaded: root.give()
    }

    // Lithic's blade: the wipe's leading edge
    Rectangle {
        x: root.fromRight ? 0 : root.width - width
        width: Math.max(1, Math.round(root.s))
        height: parent.height
        color: Lithic.sel
        visible: root.clip
    }

    Binding { target: root.view; property: "active"; value: root.current && root.barActive; when: root.view !== null }
    Binding { target: root.view; property: "query"; value: root.current ? root.fieldText : root.heldText; when: root.view !== null }
    Binding { target: root.view; property: "args"; value: root.args; when: root.view !== null }
    Binding { target: root.view; property: "maximumHeight"; value: root.maximumHeight; when: root.view !== null && "maximumHeight" in root.view }
}

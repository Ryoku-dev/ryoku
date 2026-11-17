pragma ComponentBehavior: Bound

import QtQuick
import Lithic

// What every notification look shares, so each look file only draws: the inputs
// the column hands a card, the actions it offers, compact versus expanded, the
// height the column lays out, the lifespan and the arrival and exit hooks.
//
// A look sets `contentHeight` to its drawn height and binds `bodyLines` to an
// unclamped copy of its body, so the expand chevron shows only when expanding
// reveals something. It sets `enterMotion` and `exitMotion` to its own
// animations over `arrive` (0 hidden, 1 at rest; a spring may pass 1) and `gone`
// (0 at rest, 1 gone). Under reduced motion the card stands at rest at once and
// leaves at once, and its lifespan chrome holds still.
Item {
    id: root

    // a Notifs entry, or any object with the same fields in a preview
    required property var notif
    property real us: 1
    property bool compact: true
    property int lifespanMs: 0
    // the column's notification_position, so a card arrives from its own edge
    property string position: "Right"
    property string timeText: ""
    property bool showImages: false
    property string imageSource: ""
    property string appIconSource: ""
    property bool shadows: true

    signal closeRequested()
    signal actionInvoked()
    // the exit has finished; the column may let the card go
    signal departed()

    // The entry dies while its card is still leaving, and a dead object answers
    // nothing, so every read goes through n(): the entry while it lives, else
    // what it said when it arrived (its actions inert).
    property var held: ({})
    function n() {
        return root.notif || root.held;
    }
    function hold() {
        if (!root.notif)
            return;
        const all = root.notif.actions || [];
        const actions = [];
        for (let i = 0; i < all.length; i++)
            if (all[i])
                actions.push({ identifier: all[i].identifier, text: all[i].text, invoke: () => {} });
        root.held = {
            appName: root.notif.appName, summary: root.notif.summary, body: root.notif.body,
            urgency: root.notif.urgency, actions: actions
        };
    }
    onNotifChanged: root.hold()
    readonly property string appName: root.n().appName || ""
    readonly property string summary: root.n().summary || ""
    readonly property string body: root.n().body || ""
    readonly property bool critical: root.n().urgency === 2

    // the freedesktop default action is the open button, never a row button
    readonly property var visibleActions: {
        const all = root.n().actions || [];
        const out = [];
        for (let i = 0; i < all.length; i++)
            if (all[i] && all[i].identifier !== "default" && (all[i].text || "").length > 0)
                out.push(all[i]);
        return out;
    }
    readonly property var defaultAction: {
        const all = root.n().actions || [];
        for (let i = 0; i < all.length; i++)
            if (all[i] && all[i].identifier === "default")
                return all[i];
        return null;
    }
    function openDefault() {
        if (root.defaultAction)
            root.defaultAction.invoke();
        root.actionInvoked();
    }
    function invoke(action) {
        action.invoke();
        root.actionInvoked();
    }

    readonly property int compactLines: 2
    property bool expanded: false
    readonly property bool showFull: !root.compact || root.expanded
    property int bodyLines: 0
    readonly property bool expandable: root.compact && (root.visibleActions.length > 0 || root.bodyLines > root.compactLines)

    // Only an explicit expand or collapse eases the height; anything else (an
    // image arriving, a font settling) snaps, so the column never trails the card.
    property real contentHeight: 0
    readonly property real targetImplicitHeight: root.contentHeight
    property real animatedImplicitHeight: 0
    property bool userHeightChange: false
    property int expandMs: 260
    property var expandCurve: Lithic.curve
    implicitHeight: root.animatedImplicitHeight
    Component.onCompleted: {
        root.hold();
        root.animatedImplicitHeight = root.targetImplicitHeight;
        root.runDrain();
    }
    function toggleExpanded() {
        root.userHeightChange = true;
        root.expanded = !root.expanded;
        heightStart.restart();
    }
    onTargetImplicitHeightChanged: {
        if (root.userHeightChange) {
            heightStart.restart();
            return;
        }
        heightMotion.stop();
        root.animatedImplicitHeight = root.targetImplicitHeight;
    }
    // a frame for the new layout to land before the ease reads its height
    Timer {
        id: heightStart
        interval: 16
        onTriggered: {
            heightMotion.stop();
            heightMotion.from = root.animatedImplicitHeight;
            heightMotion.to = root.targetImplicitHeight;
            heightMotion.start();
            root.userHeightChange = false;
        }
    }
    NumberAnimation {
        id: heightMotion
        target: root
        property: "animatedImplicitHeight"
        duration: Lithic.duration(root.expandMs)
        easing.type: Easing.BezierSpline
        easing.bezierCurve: root.expandCurve
    }

    // which way is out: the edge the column hangs from
    readonly property real edgeX: root.position === "Left" ? -1 : root.position === "Center" ? 0 : 1
    readonly property real edgeY: root.position === "Center" ? -1 : 0
    readonly property int anchorOrigin: root.position === "Left" ? Item.TopLeft
        : root.position === "Center" ? Item.Top
        : root.position === "BottomRight" ? Item.BottomRight : Item.TopRight

    // A look with countdown chrome sets `drains` and how many steps it shows the
    // lifespan in (`lifeSteps`: a fuse's ticks, a ring's segments), then reads
    // `lifeStep` or `remaining` (1 to 0). The drain steps on a timer rather than
    // animating, so the card redraws once a step, not every frame of the
    // lifespan. A persistent card (lifespan 0) keeps it full and draws no
    // countdown; under reduced motion it holds still.
    readonly property bool counting: root.lifespanMs > 0
    property bool drains: false
    property int lifeSteps: 100
    property int lifeStep: 0
    readonly property real remaining: root.counting ? 1 - root.lifeStep / Math.max(1, root.lifeSteps) : 1
    readonly property bool draining: root.drains && root.counting && !Lithic.reduceMotion && !root.leaving
    onDrainingChanged: root.runDrain()
    // the interval is read when the drain starts: bound, a lifespan set after
    // the card was built would start it on the wrong beat
    function runDrain() {
        if (!root.draining) {
            drain.stop();
            return;
        }
        root.lifeStep = 0;
        drain.interval = Math.max(16, root.lifespanMs / Math.max(1, root.lifeSteps));
        drain.restart();
    }
    Timer {
        id: drain
        repeat: true
        onTriggered: {
            root.lifeStep++;
            if (root.lifeStep >= root.lifeSteps)
                drain.stop();
        }
    }

    property real arrive: 0
    property real gone: 0
    property Animation enterMotion: null
    property Animation exitMotion: null
    property bool leaving: false
    // the instant the card first stands at rest, for a flourish after landing
    signal landed()
    // A card arriving over others waits while the stack makes room, so it never
    // fades in over the card it is pushing down.
    property int enterDelay: 0
    Timer {
        id: enterWait
        interval: root.enterDelay
        onTriggered: root.enterMotion.restart()
    }

    function enter() {
        if (Lithic.reduceMotion || !root.enterMotion) {
            root.settle();
            return;
        }
        if (root.enterDelay > 0)
            enterWait.restart();
        else
            root.enterMotion.restart();
    }
    function settle() {
        enterWait.stop();
        if (root.enterMotion)
            root.enterMotion.stop();
        root.arrive = 1;
        root.gone = 0;
    }
    function leave() {
        if (root.leaving)
            return;
        root.leaving = true;
        enterWait.stop();
        if (root.enterMotion)
            root.enterMotion.stop();
        if (Lithic.reduceMotion || !root.exitMotion) {
            root.gone = 1;
            root.departed();
            return;
        }
        root.exitMotion.restart();
    }
    Connections {
        target: root.enterMotion
        function onFinished() {
            root.landed();
        }
    }
    Connections {
        target: root.exitMotion
        function onFinished() {
            root.departed();
        }
    }
}

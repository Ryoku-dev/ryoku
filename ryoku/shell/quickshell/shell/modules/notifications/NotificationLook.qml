pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import "designs"

// Ryoku's own notification card in the desktop's design language (theme.design),
// switched live: each look in designs/ owns its shape, its type and its motion.
// The column hands it the entry and what the shell knows about it (the time, the
// image sources) and hears back when to dismiss it and when its exit is done.
// It knows nothing of the shell's services, so Settings' recordings film it as is.
Loader {
    id: root

    required property var notif
    property real us: 1
    property bool compact: true
    property int lifespanMs: 0
    property string position: "Right"
    property string timeText: ""
    property bool showImages: false
    property string imageSource: ""
    property string appIconSource: ""
    property bool shadows: true
    // how long a card arriving over others waits for the stack to make room
    property int enterDelay: 0

    signal closeRequested()
    signal actionInvoked()
    signal departed()

    readonly property real targetImplicitHeight: root.item ? root.item.targetImplicitHeight : 0

    // A look swapped in while the card is up stands at rest at once, or picks up
    // the exit where the last one was, so the column still hears it leave.
    property bool entered: false
    property bool leaving: false
    function leave() {
        root.leaving = true;
        if (root.item)
            root.item.leave();
        else
            root.departed();
    }
    onLoaded: {
        if (root.leaving)
            root.item.leave();
        else if (root.entered)
            root.item.settle();
        else
            root.item.enter();
        root.entered = true;
    }

    sourceComponent: Lithic.design === "material" ? material
        : Lithic.design === "adwaita" ? adwaita
        : Lithic.design === "fluent" ? fluent
        : lithic

    Connections {
        target: root.item
        function onCloseRequested() {
            root.closeRequested();
        }
        function onActionInvoked() {
            root.actionInvoked();
        }
        function onDeparted() {
            root.departed();
        }
    }

    // Each look is built with its real width and inputs, so it measures its true
    // height the moment it is inserted: one that grew a frame later left the
    // column's displaced cards short of it, and the stack overlapped at rest.
    Component {
        id: lithic
        LithicNotification {
            width: root.width; notif: root.notif; us: root.us; compact: root.compact
            lifespanMs: root.lifespanMs; position: root.position; timeText: root.timeText
            showImages: root.showImages; imageSource: root.imageSource
            appIconSource: root.appIconSource; shadows: root.shadows; enterDelay: root.enterDelay
        }
    }
    Component {
        id: material
        MaterialNotification {
            width: root.width; notif: root.notif; us: root.us; compact: root.compact
            lifespanMs: root.lifespanMs; position: root.position; timeText: root.timeText
            showImages: root.showImages; imageSource: root.imageSource
            appIconSource: root.appIconSource; shadows: root.shadows; enterDelay: root.enterDelay
        }
    }
    Component {
        id: adwaita
        AdwaitaNotification {
            width: root.width; notif: root.notif; us: root.us; compact: root.compact
            lifespanMs: root.lifespanMs; position: root.position; timeText: root.timeText
            showImages: root.showImages; imageSource: root.imageSource
            appIconSource: root.appIconSource; shadows: root.shadows; enterDelay: root.enterDelay
        }
    }
    Component {
        id: fluent
        FluentNotification {
            width: root.width; notif: root.notif; us: root.us; compact: root.compact
            lifespanMs: root.lifespanMs; position: root.position; timeText: root.timeText
            showImages: root.showImages; imageSource: root.imageSource
            appIconSource: root.appIconSource; shadows: root.shadows; enterDelay: root.enterDelay
        }
    }
}

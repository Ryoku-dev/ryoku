pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import Ryoku.Ui.Singletons

// The quick-settings face drawn inside the island's clock pill. The island owns
// the growth; this owns the pages and the push between them: two scrollers, only
// one visible at a time, the incoming one sliding in while the outgoing slides
// away.
Item {
    id: pane

    property var host
    property string page: "home"
    property int dir: 1
    property bool frontIsA: true
    // Probe ownership belongs to the pane, not its briefly overlapping pages.
    // The caches survive route changes and repeated opens.
    property bool devicesHeld: false
    property var ddcBrightness: ({})
    property var ddcBrightnessAt: ({})
    property var monitorSnapshot: []
    property bool monitorSnapshotValid: false

    function holdDevices(held) {
        if (pane.devicesHeld === held)
            return;
        pane.devicesHeld = held;
        if (held)
            Devices.startProbes(pane);
        else
            Devices.stopProbes(pane);
    }

    function urlFor(id) {
        if (id === "wifi") return "pages/WifiPage.qml";
        if (id === "bluetooth") return "pages/BluetoothPage.qml";
        if (id === "display") return "pages/DisplayPage.qml";
        if (id === "sound") return "pages/SoundPage.qml";
        return "pages/HomePage.qml";
    }
    function go(id) {
        if (pane.page === id) return;
        pane.dir = (id === "home") ? -1 : 1;
        pane.page = id;
    }
    function back() { pane.go("home"); }
    function close() { if (pane.host) pane.host.quickCloseRequested(); }

    function reset() {
        pane.page = "home";
        pane.dir = 1;
        pane.frontIsA = true;
        loaderA.source = pane.urlFor("home");
        loaderB.source = "";
        scrollerA.visible = true;
        scrollerA.z = 1;
        scrollerA.slide = 0;
        scrollerA.opacity = 1;
        scrollerB.visible = false;
        scrollerB.z = 0;
        scrollerB.slide = 0;
        scrollerB.opacity = 0;
    }

    onVisibleChanged: {
        if (pane.visible) {
            releaseProbes.stop();
            pane.holdDevices(true);
            return;
        }
        pane.reset();
        releaseProbes.restart();
    }
    Component.onCompleted: {
        if (pane.visible)
            pane.holdDevices(true);
    }
    Component.onDestruction: Devices.stopProbes(pane)

    Timer {
        id: releaseProbes
        interval: 600
        onTriggered: pane.holdDevices(false)
    }
    Connections {
        target: Wm
        function onOutputsChanged() {
            pane.monitorSnapshot = [];
            pane.monitorSnapshotValid = false;
            pane.ddcBrightness = ({});
            pane.ddcBrightnessAt = ({});
        }
    }
    Connections {
        target: Devices
        function onDdcMonitorsChanged() {
            pane.ddcBrightness = ({});
            pane.ddcBrightnessAt = ({});
        }
    }
    onPageChanged: {
        if (!pane.visible) return;
        var outScroll = pane.frontIsA ? scrollerA : scrollerB;
        var innScroll = pane.frontIsA ? scrollerB : scrollerA;
        var innLoader = pane.frontIsA ? loaderB : loaderA;
        innLoader.source = pane.urlFor(pane.page);
        innScroll.slide = pane.dir;
        innScroll.opacity = 0;
        innScroll.visible = true;
        innScroll.z = 2;
        outScroll.z = 1;
        Qt.callLater(function () {
            innScroll.slide = 0;
            innScroll.opacity = 1;
            outScroll.slide = -pane.dir * 0.28;
            outScroll.opacity = 0;
        });
        pane.frontIsA = !pane.frontIsA;
        clearTimer.restart();
    }
    Timer {
        id: clearTimer
        interval: Math.max(1, Motion.morph) + 60
        onTriggered: {
            var behindScroll = pane.frontIsA ? scrollerB : scrollerA;
            var behindLoader = pane.frontIsA ? loaderB : loaderA;
            behindScroll.visible = false;
            behindLoader.source = "";
        }
    }

    Flickable {
        id: scrollerA
        width: pane.width
        height: pane.height
        x: scrollerA.slide * pane.width
        contentWidth: width
        contentHeight: loaderA.implicitHeight
        boundsBehavior: Flickable.StopAtBounds
        clip: true
        property real slide: 0
        opacity: 1
        Behavior on opacity {
            enabled: !Motion.reduce
            NumberAnimation { duration: Motion.morph; easing.type: Motion.easeStandard }
        }
        Behavior on slide {
            enabled: !Motion.reduce
            NumberAnimation { duration: Motion.morph; easing.type: Motion.easeStandard }
        }
        Loader {
            id: loaderA
            width: scrollerA.width
            active: pane.visible
            source: pane.urlFor("home")
            onLoaded: if (item) item.host = pane
        }
    }

    Flickable {
        id: scrollerB
        width: pane.width
        height: pane.height
        x: scrollerB.slide * pane.width
        contentWidth: width
        contentHeight: loaderB.implicitHeight
        boundsBehavior: Flickable.StopAtBounds
        clip: true
        property real slide: 0
        opacity: 0
        visible: false
        Behavior on opacity {
            enabled: !Motion.reduce
            NumberAnimation { duration: Motion.morph; easing.type: Motion.easeStandard }
        }
        Behavior on slide {
            enabled: !Motion.reduce
            NumberAnimation { duration: Motion.morph; easing.type: Motion.easeStandard }
        }
        Loader {
            id: loaderB
            width: scrollerB.width
            active: pane.visible
            onLoaded: if (item) item.host = pane
        }
    }
}

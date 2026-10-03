import QtQuick
import Quickshell
import "../../../reusables"
import "../../../"

Item {
    id: root
    anchors.fill: parent

    property real minWidth: 100
    property real minHeight: 70
    property real maxWidth: 600
    property real maxHeight: 400
    property real minAspect: 0.6
    property real maxAspect: 3.0
    property bool isRound: false
    // Ryoku host hook: while the desktop slot draws the plate (Ryoku style),
    // this face's own plate steps aside; the content keeps painting.
    property bool ryokuBare: false

    property bool isSubscribed: false
    property bool compactMode: root.height < Scaler.s(100) || root.width < Scaler.s(140)

    function updateSubscription() {
        if (root.visible && !root.isSubscribed) {
            SysData.subscribe();
            root.isSubscribed = true;
        } else if (!root.visible && root.isSubscribed) {
            SysData.unsubscribe();
            root.isSubscribed = false;
        }
    }

    onVisibleChanged: updateSubscription()
    Component.onCompleted: updateSubscription()
    Component.onDestruction: {
        if (root.isSubscribed) {
            SysData.unsubscribe();
            root.isSubscribed = false;
        }
    }

    property real wavePhase: waveClock.phase
    WaveClock {
        id: waveClock
        duration: 1800
        running: root.visible
    }

    property real rawTemp: isNaN(SysData.temp) ? 0.0 : SysData.temp
    property real tempC: rawTemp
    Behavior on tempC { enabled: root.visible; NumberAnimation { duration: 800; easing.type: Easing.OutQuint } }

    SystemUsageCard {
        anchors.fill: parent
        value: Math.max(0.0, Math.min(1.0, root.tempC / 100.0))
        colorBase: root.ryokuBare ? "transparent" : ThemeBackend.surface0
        colorFill: ThemeBackend.mauve
        icon: "\uF2C9"
        title: I18n.t("quickactions.systemusage.temp")
        valueText: Math.round(root.tempC) + "°"
        wavePhase: root.wavePhase
        isLive: root.visible
        compact: root.compactMode
        scaleFunc: Scaler.s
    }
}

pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import "../kit" as K
import "../settings" as S

Item {
    id: root

    required property var scene
    required property string lane

    readonly property var selected: S.Prefs.sysmonItems || []
    readonly property bool wantsStats: selected.indexOf("gpu") >= 0
        || selected.indexOf("net") >= 0
    readonly property var visibleItems: {
        const result = [];
        for (let i = 0; i < selected.length; ++i) {
            const id = selected[i];
            if (id === "temp" && !Sysinfo.hasTemp)
                continue;
            if (id === "gpu" && !StatsFeed.gpuAvailable)
                continue;
            result.push(id);
        }
        return result;
    }

    property bool present: visibleItems.length > 0

    implicitWidth: present ? content.implicitWidth + 2 * K.Style.hitPad : 0
    implicitHeight: K.Style.barH
    height: K.Style.barH

    function syncClaims() {
        Sysinfo.setActive(root, root.visible && root.present);
        StatsFeed.setActive(root, root.visible && root.present && root.wantsStats);
    }

    function iconFor(id) {
        if (id === "cpu") return "developer_board";
        if (id === "mem") return "memory";
        if (id === "temp") return "thermostat";
        if (id === "gpu") return "monitor";
        return "download";
    }

    function valueFor(id) {
        if (id === "cpu") return Math.round(Sysinfo.cpu * 100) + "%";
        if (id === "mem") return Math.round(Sysinfo.mem * 100) + "%";
        if (id === "temp") return Math.round(Sysinfo.tempC) + "°";
        if (id === "gpu") return Math.round(StatsFeed.gpuPct) + "%";
        return compactRate(StatsFeed.downBps);
    }

    function widestFor(id) {
        if (id === "temp") return "88°";
        if (id === "net") return "88.8K/s";
        return "88%";
    }

    function compactRate(value) {
        const rate = Math.max(0, Number(value) || 0);
        if (rate >= 1048576)
            return (rate / 1048576).toFixed(rate >= 10485760 ? 0 : 1) + "M/s";
        return (rate / 1024).toFixed(rate >= 102400 ? 0 : 1) + "K/s";
    }

    function fullRate(value) {
        const rate = Math.max(0, Number(value) || 0);
        if (rate >= 1048576)
            return (rate / 1048576).toFixed(1) + " MB/s";
        return (rate / 1024).toFixed(1) + " KB/s";
    }

    function readings() {
        const parts = [];
        for (let i = 0; i < visibleItems.length; ++i) {
            const id = visibleItems[i];
            if (id === "cpu")
                parts.push("Processor " + Math.round(Sysinfo.cpu * 100) + "%");
            else if (id === "mem")
                parts.push("Memory " + Sysinfo.memUsedGiB.toFixed(1) + " of "
                    + Sysinfo.memTotalGiB.toFixed(1) + " GiB");
            else if (id === "temp")
                parts.push("Temperature " + Math.round(Sysinfo.tempC) + "°C");
            else if (id === "gpu")
                parts.push("Graphics " + Math.round(StatsFeed.gpuPct) + "%");
            else
                parts.push("Network down " + fullRate(StatsFeed.downBps)
                    + ", up " + fullRate(StatsFeed.upBps));
        }
        return parts.join(", ");
    }

    onVisibleChanged: syncClaims()
    onPresentChanged: syncClaims()
    onWantsStatsChanged: syncClaims()
    Component.onCompleted: syncClaims()
    Component.onDestruction: {
        Sysinfo.setActive(root, false);
        StatsFeed.setActive(root, false);
    }

    Row {
        id: content
        anchors.centerIn: parent
        spacing: K.Style.s2
        scale: press.pressScale

        Repeater {
            model: root.visibleItems

            delegate: Row {
                id: reading

                required property string modelData
                required property int index

                spacing: 3

                K.Glyph {
                    anchors.verticalCenter: parent.verticalCenter
                    name: root.iconFor(reading.modelData)
                    size: Math.max(12, Math.round(14 * K.Style.scale))
                    tone: "dim"
                }

                // A reading holds the width of its widest usual value, so a
                // number gaining a digit never resizes the drop: every resize
                // springs the whole bar body for a second.
                K.Label {
                    anchors.verticalCenter: parent.verticalCenter
                    width: Math.max(implicitWidth, widest.implicitWidth)
                    horizontalAlignment: Text.AlignRight
                    text: root.valueFor(reading.modelData)
                    role: "barSub"
                    tone: "dim"
                    numeric: true
                }

                K.Label {
                    id: widest
                    visible: false
                    text: root.widestFor(reading.modelData)
                    role: "barSub"
                    numeric: true
                }
            }
        }
    }

    K.Press {
        id: press
        radius: K.Style.radius
        onClicked: root.scene.toggleCard("system", root)
        onRightClicked: root.scene.toggleCard("system", root)
        onHoveredChanged: root.scene.hoverCard("system", root, hovered)
    }

    K.Hint {
        target: root
        text: root.readings()
        shown: root.present && press.hovered
    }
}

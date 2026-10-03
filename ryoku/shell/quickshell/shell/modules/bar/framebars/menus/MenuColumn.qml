pragma ComponentBehavior: Bound

import QtQuick

// The shared menu panel body (contract 05 sec 2a): a vertical stack of the
// menu's widgets inside a 20px margin, exactly minimumWidth wide (the width is
// set by the host and never grows to content). Content taller than the band
// scrolls vertically with no visible scrollbar chrome; there is no horizontal
// scrolling.
Item {
    id: root

    property var widgets: []
    property bool open: false
    property bool mounted: open
    property bool retain: false
    property bool incubate: false
    property real scale: 1
    signal requestClose()

    // Repeater.itemAt() does not make the returned delegate's properties part
    // of this binding's dependency graph. Bump a scalar when a nested loader
    // settles so an early false result is reevaluated instead of sticking.
    property int readinessRevision: 0
    readonly property bool ready: {
        root.readinessRevision;
        if (hosts.count !== root.widgets.length)
            return false;
        for (let index = 0; index < hosts.count; ++index) {
            const host = hosts.itemAt(index);
            if (!host || !host.contentReady)
                return false;
        }
        return true;
    }

    readonly property real pad: 20 * scale
    implicitWidth: column.implicitWidth + pad * 2
    implicitHeight: column.implicitHeight + pad * 2

    Flickable {
        anchors.fill: parent
        contentWidth: width
        contentHeight: column.implicitHeight + root.pad * 2
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        // Only steal drags when there is overflow to scroll.
        interactive: root.open && contentHeight > height

        Column {
            id: column
            x: root.pad
            y: root.pad
            width: root.width - root.pad * 2
            spacing: 0

            Repeater {
                id: hosts
                model: root.mounted || root.retain ? root.widgets : []
                delegate: MenuHostLoader {
                    required property var modelData
                    width: column.width
                    widget: modelData
                    scale: root.scale
                    open: root.open
                    incubate: root.incubate
                    onContentReadyChanged: ++root.readinessRevision
                    // A sole widget owns the whole band; stacked widgets keep
                    // natural heights.
                    onRequestClose: root.requestClose()
                }
            }
        }
    }
}

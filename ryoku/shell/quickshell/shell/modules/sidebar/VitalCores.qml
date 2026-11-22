pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons

Item {
    id: root

    required property var monitor
    required property real s

    readonly property int count: Math.max(0, root.monitor.coreCount)
    readonly property bool available: root.monitor.cpuAvailable && root.count > 0
    readonly property real cell: root.count > 0 ? root.width / root.count : 0
    readonly property real bar: QsLook.lithic
        ? Math.max(Math.round(2 * root.s), Math.min(root.cell * 0.45, Math.round(5 * root.s)))
        : Math.max(Math.round(3 * root.s), Math.min(root.cell * 0.6, Math.round(8 * root.s)))

    function ink(load) {
        return load > 85 ? Lithic.amber : load >= 40 ? Lithic.fg2 : Lithic.fg3;
    }

    implicitHeight: Math.round(12 * root.s)
    Accessible.role: Accessible.Graphic
    Accessible.name: I18n.tr("Load per core")

    Rectangle {
        visible: QsLook.lithic || !root.available
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        height: Math.max(1, Math.round(root.s))
        color: Lithic.vein
    }

    Repeater {
        model: root.available ? root.count : 0

        delegate: Item {
            id: core

            required property int index
            readonly property real load: {
                const loads = root.monitor.coreLoads;
                if (!loads || core.index >= loads.length || !isFinite(loads[core.index]))
                    return 0;
                return Math.max(0, Math.min(100, loads[core.index]));
            }

            x: core.index * root.cell
            width: root.cell
            height: root.height

            Rectangle {
                visible: !QsLook.lithic
                anchors.horizontalCenter: parent.horizontalCenter
                anchors.bottom: parent.bottom
                width: root.bar
                height: parent.height
                radius: width / 2
                color: Lithic.alpha(Lithic.fg, QsLook.dark ? 0.08 : 0.07)
            }

            CutRect {
                visible: QsLook.lithic
                anchors.horizontalCenter: parent.horizontalCenter
                anchors.bottom: parent.bottom
                width: root.bar
                height: Math.max(Math.round(2 * root.s), parent.height * core.load / 100)
                cut: 0
                cutTopRight: root.bar
                fill: root.ink(core.load)

                Behavior on height {
                    NumberAnimation { duration: QsLook.dur(root.monitor.samplePeriodMs); easing.type: Easing.Linear }
                }
                Behavior on fill {
                    ColorAnimation { duration: QsLook.dur(QsLook.stateMs * 2) }
                }
            }

            Rectangle {
                visible: !QsLook.lithic
                anchors.horizontalCenter: parent.horizontalCenter
                anchors.bottom: parent.bottom
                width: root.bar
                height: Math.max(root.bar, parent.height * core.load / 100)
                radius: width / 2
                color: root.ink(core.load)

                Behavior on height {
                    NumberAnimation { duration: QsLook.dur(root.monitor.samplePeriodMs); easing.type: Easing.Linear }
                }
                Behavior on color {
                    ColorAnimation { duration: QsLook.dur(QsLook.stateMs * 2) }
                }
            }
        }
    }
}

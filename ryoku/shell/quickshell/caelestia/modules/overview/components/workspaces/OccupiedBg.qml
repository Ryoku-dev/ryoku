pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Caelestia.Config
import caelestia.components
import caelestia.services

Item {
    id: root

    required property Repeater workspaces
    required property var occupied
    required property int groupOffset
    property list<var> pills: []

    onOccupiedChanged: {
        if (!occupied)
            return;
        let count = 0;
        const start = groupOffset;
        const wsCount = root.workspaces.count > 0 ? root.workspaces.count : Config.bar.workspaces.shown;
        const end = start + wsCount;
        for (const [ws, occ] of Object.entries(occupied)) {
            if (ws > start && ws <= end && occ) {
                const isFirstInGroup = Number(ws) === start + 1;
                const isLastInGroup = Number(ws) === end;
                if (isFirstInGroup || !occupied[ws - 1]) {
                    if (pills[count])
                        pills[count].start = ws;
                    else
                        pills.push(pillComp.createObject(root, {
                            start: ws
                        }));
                    count++;
                }
                if ((isLastInGroup || !occupied[ws + 1]) && pills[count - 1])
                    pills[count - 1].end = ws;
            }
        }
        if (pills.length > count)
            pills.splice(count, pills.length - count).forEach(p => p.destroy());
    }

    Repeater {
        model: ScriptModel {
            values: root.pills.filter(p => p)
        }

        StyledRect {
            id: rect

            required property var modelData
            readonly property Workspace start: root.workspaces.count > 0 ? root.workspaces.itemAt(getWsIdx(modelData.start)) ?? null : null // qmllint disable incompatible-type
            readonly property Workspace end: root.workspaces.count > 0 ? root.workspaces.itemAt(getWsIdx(modelData.end)) ?? null : null // qmllint disable incompatible-type
            readonly property int indicatorSize: start ? start.indicatorSize : 40

            function getWsIdx(ws: int): int {
                let i = ws - 1;
                const count = root.workspaces.count > 0 ? root.workspaces.count : Config.bar.workspaces.shown;
                while (i < 0)
                    i += count;
                return i % count;
            }

            anchors.verticalCenter: root.verticalCenter
            x: (start?.x ?? 0) - 1
            y: 0
            implicitWidth: (start && end ? end.x + end.size - start.x : 0)
            implicitHeight: indicatorSize
            color: Colours.layer(Colours.palette.m3surfaceContainerHigh, 2)
            radius: Tokens.rounding.large
            scale: 0
            Component.onCompleted: scale = 1

            Behavior on scale {
                Anim {
                    easing: Tokens.anim.standardDecel
                }
            }
            Behavior on x {
                Anim {}
            }
            Behavior on implicitWidth {
                Anim {}
            }
        }
    }
    Component {
        id: pillComp

        Pill {}
    }

    component Pill: QtObject {
        property int start
        property int end
    }
}

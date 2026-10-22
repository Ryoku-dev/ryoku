pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Caelestia.Components
import Caelestia.Config
import caelestia.components
import caelestia.services

Item {
    id: root

    required property var workspaces
    required property int wsSpacing
    required property bool isHorizontal

    readonly property color colour: Colours.layer(Colours.palette.m3surfaceContainerHigh, 2)
    property color colourAnimated: colour

    Behavior on colourAnimated {
        CAnim {}
    }

    // Wrappers so the rects can extend 1px past the strip and still be faded as
    // one layer.
    Item {
        anchors.fill: parent
        anchors.margins: -1

        opacity: root.colourAnimated.a
        layer.enabled: opacity < 1

        Item {
            anchors.fill: parent
            anchors.margins: 1

            AnimatedRepeater {
                model: ScriptModel {
                    values: root.workspaces
                }

                removeDuration: Tokens.anim.durations.expressiveDefaultEffects

                OccupiedRect {}
            }
        }
    }

    component OccupiedRect: StyledRect {
        id: rect

        required property int index
        required property Workspace modelData

        readonly property real pillRadius: modelData ? (root.isHorizontal ? modelData.height : modelData.width) / 2 : 0
        property real leadRadius: ifAdjacent(0, -1, 0, pillRadius)
        property real trailRadius: ifAdjacent(root.workspaces.length - 1, 1, 0, pillRadius)
        property real leadPadding: ifAdjacent(0, -1, root.wsSpacing, 0)
        property real trailPadding: ifAdjacent(root.workspaces.length - 1, 1, root.wsSpacing, 0)

        function ifAdjacent(exclIdx: int, adj: int, yes: real, no: real): real {
            if (AnimatedRepeater.adding || AnimatedRepeater.removing || !modelData?.isOccupied || index === exclIdx)
                return no;
            return root.workspaces[index + adj]?.isOccupied ? yes : no;
        }

        x: modelData ? modelData.x - (root.isHorizontal ? leadPadding + 1 : 1) : 0
        y: modelData ? modelData.y - (root.isHorizontal ? 1 : leadPadding + 1) : 0
        implicitWidth: root.isHorizontal ? (modelData ? modelData.size + leadPadding + trailPadding + 2 : 0) : (modelData ? modelData.width + 2 : 0)
        implicitHeight: root.isHorizontal ? (modelData ? modelData.height + 2 : 0) : (modelData ? modelData.size + leadPadding + trailPadding + 2 : 0)

        topLeftRadius: leadRadius
        bottomLeftRadius: root.isHorizontal ? leadRadius : trailRadius
        topRightRadius: root.isHorizontal ? trailRadius : leadRadius
        bottomRightRadius: trailRadius

        color: Qt.alpha(root.colour, 1)
        opacity: modelData?.isOccupied ? 1 : 0

        Behavior on leadRadius {
            Anim {
                type: Anim.DefaultEffects
            }
        }

        Behavior on trailRadius {
            Anim {
                type: Anim.DefaultEffects
            }
        }

        Behavior on leadPadding {
            Anim {
                type: Anim.DefaultEffects
            }
        }

        Behavior on trailPadding {
            Anim {
                type: Anim.DefaultEffects
            }
        }

        Behavior on opacity {
            Anim {
                type: Anim.DefaultEffects
            }
        }
    }
}

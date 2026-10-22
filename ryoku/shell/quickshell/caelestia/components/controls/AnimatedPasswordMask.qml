pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import M3Shapes
import Caelestia.Config
import caelestia.components
import caelestia.services

ListView {
    id: root

    required property string buffer
    property var shapeQueue: []
    property bool showPassword: false

    readonly property int fullWidth: {
        let w = (count - 1) * spacing;
        for (let i = 0; i < count; i++)
            w += ((itemAtIndex(i) as CharItem)?.nonAnimWidthScale ?? 1) * implicitHeight;
        return w + implicitHeight;
    }

    function bindImWidth(): void {
        imWidthBehavior.enabled = false;
        implicitWidth = Qt.binding(() => fullWidth);
        imWidthBehavior.enabled = true;
    }

    implicitWidth: fullWidth
    implicitHeight: Tokens.font.body.medium.pointSize

    orientation: Qt.Horizontal
    spacing: Tokens.spacing.extraSmall
    interactive: false

    model: ScriptModel {
        values: root.buffer.split("")
    }

    delegate: CharItem {}

    Behavior on implicitWidth {
        id: imWidthBehavior

        Anim {}
    }

    component CharItem: Item {
        id: ch

        required property int index
        property real nonAnimWidthScale: 1

        implicitHeight: root.implicitHeight

        ListView.onRemove: {
            initAnim.stop();
            removeAnim.start();
        }

        MaterialShape {
            id: charShape

            anchors.centerIn: parent
            implicitSize: root.implicitHeight * 1.5
            shape: root.shapeQueue[ch.index % root.shapeQueue.length] ?? MaterialShape.Circle
            color: Colours.palette.m3onSurface

            opacity: root.showPassword ? 0 : 1

            Behavior on opacity {
                Anim {
                    type: Anim.DefaultEffects
                }
            }

            Behavior on color {
                CAnim {}
            }

            SequentialAnimation {
                id: initAnim

                running: true

                ParallelAnimation {
                    Anim {
                        target: ch
                        property: "opacity"
                        from: 0
                        to: 1
                        type: Anim.DefaultEffects
                    }
                    Anim {
                        target: ch
                        property: "scale"
                        from: 0
                        to: 1
                        type: Anim.FastSpatial
                    }
                    Anim {
                        target: ch
                        property: "implicitWidth"
                        from: root.implicitHeight
                        to: root.implicitHeight * 1.3
                        type: Anim.DefaultEffects
                    }
                    PropertyAction {
                        target: ch
                        property: "nonAnimWidthScale"
                        value: 1.5
                    }
                }
                PauseAnimation {
                    duration: 180 * Tokens.anim.durations.scale
                }
                PropertyAction {
                    target: charShape
                    property: "shape"
                    value: MaterialShape.Circle
                }
                ParallelAnimation {
                    Anim {
                        target: charShape
                        property: "scale"
                        to: 2 / 3
                        type: Anim.FastSpatial
                    }
                    Anim {
                        target: ch
                        property: "implicitWidth"
                        to: root.implicitHeight
                        type: Anim.DefaultEffects
                    }
                    PropertyAction {
                        target: ch
                        property: "nonAnimWidthScale"
                        value: 1
                    }
                }
            }

            SequentialAnimation {
                id: removeAnim

                PropertyAction {
                    target: ch
                    property: "ListView.delayRemove"
                    value: true
                }
                ParallelAnimation {
                    Anim {
                        type: Anim.DefaultEffects
                        target: charShape
                        property: "opacity"
                        to: 0
                    }
                    Anim {
                        target: charShape
                        property: "scale"
                        to: 0.5
                    }
                }
                PropertyAction {
                    target: ch
                    property: "ListView.delayRemove"
                    value: false
                }
            }
        }

        Loader {
            id: textLoader

            anchors.centerIn: parent

            opacity: root.showPassword ? 1 : 0
            active: opacity > 0
            asynchronous: true

            sourceComponent: StyledText {
                text: root.buffer[ch.index]
            }

            Behavior on opacity {
                Anim {
                    type: Anim.DefaultEffects
                }
            }
        }
    }
}

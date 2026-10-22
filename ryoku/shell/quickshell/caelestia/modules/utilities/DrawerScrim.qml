pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Shapes
import Caelestia.Config
import caelestia.components
import caelestia.services

/// The dimmed backdrop a utilities-drawer modal sits on. It sizes itself to the
/// drawer's surface — over the padding and border the panel clips to, and softened
/// on the two edges the drawer animation drags — so a modal only has to declare it
/// and put its own card on top. Seeded from the block RecordingDeleteModal grew.
Item {
    id: root

    required property matrix4x4 deformMatrix

    anchors.fill: parent
    anchors.margins: -Tokens.padding.large
    anchors.rightMargin: -Tokens.padding.large - Config.border.thickness
    anchors.bottomMargin: -Tokens.padding.large - Config.border.thickness
    opacity: 0.5

    StyledRect {
        anchors.fill: parent
        anchors.rightMargin: -parent.width * (1 - root.deformMatrix.m11) / 2
        anchors.bottomMargin: -parent.height * 0.1
        topLeftRadius: Tokens.rounding.extraLarge
        color: Colours.palette.m3scrim
    }

    Shape {
        id: shape

        anchors.fill: parent
        preferredRendererType: Shape.CurveRenderer
        asynchronous: true

        ShapePath {
            startX: -root.Config.border.smoothing * 2
            startY: shape.height - root.Config.border.thickness
            strokeWidth: 0
            fillGradient: LinearGradient {
                orientation: LinearGradient.Horizontal
                x1: -root.Config.border.smoothing * 2

                GradientStop {
                    position: 0
                    color: Qt.alpha(Colours.palette.m3scrim, 0)
                }
                GradientStop {
                    position: 1
                    color: Colours.palette.m3scrim
                }
            }

            PathLine {
                relativeX: root.Config.border.smoothing
                relativeY: 0
            }
            PathCubic {
                relativeX: root.Config.border.smoothing
                relativeY: -root.Config.border.smoothing
                relativeControl1X: root.Config.border.smoothing * 0.93
                relativeControl1Y: -root.Config.border.smoothing * 0.07
                relativeControl2X: root.Config.border.smoothing * 0.93
                relativeControl2Y: -root.Config.border.smoothing * 0.07
            }
            PathLine {
                relativeX: 0
                relativeY: root.Config.border.smoothing + root.Config.border.thickness
            }
            PathLine {
                relativeX: -root.Config.border.smoothing * 2
                relativeY: 0
            }
        }

        ShapePath {
            startX: shape.width - root.Config.border.smoothing - root.Config.border.thickness + (1 - root.deformMatrix.m11) * shape.width / 2
            strokeWidth: 0
            fillGradient: LinearGradient {
                orientation: LinearGradient.Vertical
                y1: -root.Config.border.smoothing * 2

                GradientStop {
                    position: 0
                    color: Qt.alpha(Colours.palette.m3scrim, 0)
                }
                GradientStop {
                    position: 1
                    color: Colours.palette.m3scrim
                }
            }

            PathCubic {
                relativeX: root.Config.border.smoothing
                relativeY: -root.Config.border.smoothing
                relativeControl1X: root.Config.border.smoothing * 0.93
                relativeControl1Y: -root.Config.border.smoothing * 0.07
                relativeControl2X: root.Config.border.smoothing * 0.93
                relativeControl2Y: -root.Config.border.smoothing * 0.07
            }
            PathLine {
                relativeX: 0
                relativeY: -root.Config.border.smoothing
            }
            PathLine {
                relativeX: root.Config.border.thickness
                relativeY: 0
            }
            PathLine {
                relativeX: 0
            }
        }
    }
}

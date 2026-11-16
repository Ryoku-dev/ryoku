import QtQuick
import Lithic

Item {
    id: root
    property var effects: []
    property string glow: ""
    property color accent: Lithic.sel
    property color ink: Lithic.fg
    property real time: 0
    property vector2d tilt: Qt.vector2d(0, 0)
    property bool borderOnly: false
    Image {
        id: mask
        width: root.width; height: root.height; opacity: 0
        source: root.glow ? "file://" + root.glow : ""
        sourceSize.width: Math.max(1, Math.ceil(root.width * 2))
        sourceSize.height: Math.max(1, Math.ceil(root.height * 2))
    }
    // a design without a glow mask emits nothing: the shader still needs a texture to read
    Rectangle {
        id: blankMask
        width: 2; height: 2; opacity: 0
        color: "black"
        layer.enabled: true
    }
    Repeater {
        model: root.effects.filter(e => (e.id === "diamond") === root.borderOnly && e.id !== "glitch")
        delegate: Loader {
            id: effect
            required property var modelData
            readonly property color tint: effect.modelData.color || root.accent
            readonly property real phase: root.time * effect.modelData.speed
            anchors.fill: parent
            opacity: effect.modelData.strength
            sourceComponent: effect.modelData.id === "foil" ? foil : effect.modelData.id === "sweep" ? sweep : effect.modelData.id === "diamond" ? diamond : atmosphere
            Component {
                id: foil
                ShaderEffect {
                    property color c1: effect.tint
                    property color c2: root.ink
                    property color c3: Lithic.mix(effect.tint, Lithic.lapis, 0.5)
                    property color c4: Lithic.mix(effect.tint, Lithic.jade, 0.5)
                    property vector2d size: Qt.vector2d(width, height)
                    property vector2d shift: root.tilt
                    property real strength: 0.5
                    property real cut: 6
                    property real etch: 0.2
                    fragmentShader: Qt.resolvedUrl("../../../shaders/foil.frag.qsb")
                }
            }
            Component {
                id: sweep
                ShaderEffect {
                    property color tint: effect.tint
                    property vector2d size: Qt.vector2d(width, height)
                    property real pos: effect.phase % 6 < 1.5 ? -0.5 + (effect.phase % 6) * 1.25 : -1
                    property real cut: 6
                    fragmentShader: Qt.resolvedUrl("../../../shaders/sweep.frag.qsb")
                }
            }
            Component {
                id: diamond
                ShaderEffect {
                    property color c1: effect.tint
                    property color c2: Lithic.mix(effect.tint, root.ink, 0.25)
                    property color c3: effect.tint
                    property color c4: Lithic.mix(effect.tint, Lithic.ground, 0.45)
                    property color c5: root.ink
                    property vector2d size: Qt.vector2d(width, height)
                    property vector2d shift: root.tilt
                    property real time: effect.phase
                    property real cut: 22
                    property real facet: 12
                    fragmentShader: Qt.resolvedUrl("../../../shaders/diamond.frag.qsb")
                }
            }
            Component {
                id: atmosphere
                ShaderEffect {
                    property var emission: root.glow ? mask : blankMask
                    property color tint: effect.tint
                    property vector2d size: Qt.vector2d(width, height)
                    property vector2d shift: root.tilt
                    property real time: effect.phase
                    property real mode: ["hexgrid", "circuit", "scan", "embers", "aurora"].indexOf(effect.modelData.id)
                    fragmentShader: Qt.resolvedUrl("../../../shaders/design-atmosphere.frag.qsb")
                }
            }
        }
    }
}

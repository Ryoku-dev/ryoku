import QtQuick

// a manga page's screentone over a rect: dots in one ink that grow along a ramp
ShaderEffect {
    property color ink: "black"
    property point size: Qt.point(width, height)
    // linear: from x,y to z,w; radial: centre x,y and radius z
    property vector4d ramp: Qt.vector4d(0, 0, width, 0)
    // coverage at the ramp's start and end; below zero draws no dot
    property point cover: Qt.point(0, 0.5)
    property real pitch: 3.6
    property real angle: 0.785
    property real radial: 0
    property real res: 2
    fragmentShader: Qt.resolvedUrl("../../../shaders/stand-tone.frag.qsb")
}

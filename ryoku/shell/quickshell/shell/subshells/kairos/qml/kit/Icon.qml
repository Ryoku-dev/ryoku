import QtQuick
import shell.barkit as Pill

Pill.MaterialIcon {
    property string name: ""
    property int size: 20
    property bool filled: false

    text: name
    fill: filled ? 1 : 0
    color: Md.ink
    font.pixelSize: size
    width: size
    height: size
    // filled glyphs overlap their own contours, the distance-field cache draws those hollow and
    // native rendering snaps to whole pixels while things scale and move
    renderType: Text.CurveRendering

    Behavior on fill {
        enabled: !Md.still
        NumberAnimation { duration: Md.fast }
    }
}

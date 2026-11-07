import QtQuick
import shell.barkit as Pill

Pill.MaterialIcon {
    id: root

    property string name: ""
    property int size: Style.icon
    property string tone: "ink"
    property bool filled: false

    function toneColor(name) {
        switch (name) {
        case "variant":
        case "dim": return Style.inkDim;
        case "muted": return Style.inkMuted;
        case "faint": return Style.inkFaint;
        case "onBone": return Style.inkOnBone;
        case "alert": return Style.alert;
        case "accent": return Style.accent;
        default: return Style.ink;
        }
    }

    text: name
    fill: filled ? 1 : 0
    font.pixelSize: size
    color: toneColor(tone)
    width: size
    height: size
    horizontalAlignment: Text.AlignHCenter
    verticalAlignment: Text.AlignVCenter
}

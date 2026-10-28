import QtQuick
import Ryoku.Ui.Singletons

// The app's mark, in ink. The generated coiled-dragon plate (3.7) is a dev-time
// art deliverable graded through DESIGN section 10; the mark that ships in the
// instrument itself is the 力 seal, ink-only and zero-chroma, anchoring the
// empty library, the KVM gate, and the ISO lane's left column.
Text {
    property real size: 96
    property color tint: Tokens.inkFaint
    text: UiPreferences.brandMark("\u529b")
    color: tint
    font.family: Tokens.jp
    font.pixelSize: size
    font.weight: Font.Bold
}

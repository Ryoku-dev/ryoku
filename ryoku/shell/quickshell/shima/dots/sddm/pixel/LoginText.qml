// Text as the shell draws it: hinted glyphs from the font engine. Qt's default (distance fields, no hinting) reads
// soft and grainy at 1x, which is how most login screens are seen.
import QtQuick 2.15

Text {
    renderType: Text.NativeRendering
}

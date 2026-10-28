import QtQuick
import QtQuick.Effects
import "Singletons"

// A faint, soft background typographic watermark: the section's kanji set huge
// and bled off an edge, sitting far behind the content as ambient texture, never
// text to read. Low opacity plus a light blur keep it from ever competing with a
// control -- this is the one blur permitted on an app surface, because it is
// background art, not panel depth (DESIGN.md §4, §12). Drop it in a page behind
// the head and the scroll; it dresses the dead lower margin with the brand
// language and answers "the page has a face" without a photo.
Item {
    id: wm

    property string text: ""
    property color color: Tokens.ink
    property real strength: 0.05        // glyph opacity: a whisper, never legible
    property real glyphScale: 0.62      // glyph height as a fraction of the item

    clip: true

    Text {
        id: glyph
        text: UiPreferences.showJapaneseGloss ? wm.text : ""
        anchors { right: parent.right; bottom: parent.bottom }
        anchors.rightMargin: -Math.round(wm.width * 0.05)
        anchors.bottomMargin: -Math.round(wm.height * 0.16)
        color: wm.color
        opacity: wm.strength
        font.family: Tokens.jp
        font.weight: Font.Light
        font.pixelSize: Math.max(160, Math.round(wm.height * wm.glyphScale))
        // soft edges, so the giant glyph reads as haze, not a hard shape. It is
        // static (never animates), so the layer captures once -- cheap.
        layer.enabled: true
        layer.effect: MultiEffect {
            blurEnabled: true
            blur: 0.5
            blurMax: 40
            autoPaddingEnabled: true
        }
    }
}

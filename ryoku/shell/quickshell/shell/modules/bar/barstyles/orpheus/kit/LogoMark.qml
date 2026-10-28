import QtQuick
import shell.barkit as Pill
import shell.services
import "LogoGlyphs.js" as LogoGlyphs

// The logo's mark, drawn once for the bar and for the settings picker so both
// agree. "system" is the desktop brand mark (力, or the glyph or image chosen in
// Ryoku Settings -> Shell -> Global); any LogoGlyphs id is that Nerd Font mark.
Item {
    id: root

    property string mark: "system"
    property real size: 18
    property color color: Theme.primary
    property int weight: Font.Black

    readonly property bool custom: mark !== "system" && LogoGlyphs.valid(mark)
    readonly property real glyphSize: custom ? Math.round(size * LogoGlyphs.scale(mark)) : size
    // Centres the ink, not the line box: CJK and Nerd Font glyphs sit off the
    // font's vertical middle by different amounts.
    readonly property real opticalOffset: metrics.imageMode ? 0
        : metrics.boundingRect.y + metrics.boundingRect.height / 2
            - metrics.tightBoundingRect.y - metrics.tightBoundingRect.height / 2

    implicitWidth: size
    implicitHeight: size

    TextMetrics {
        id: metrics
        readonly property bool imageMode: !root.custom && Theme.markSource.length > 0
        text: root.custom ? LogoGlyphs.glyph(root.mark) : Theme.mark
        font.family: root.custom ? LogoGlyphs.family(root.mark) : Theme.fontJp
        font.pixelSize: root.glyphSize
        font.weight: root.weight
    }

    Pill.BrandMark {
        visible: !root.custom
        anchors.centerIn: parent
        anchors.verticalCenterOffset: root.opticalOffset
        width: root.size
        height: root.size
        size: root.size
        color: root.color
        weight: root.weight
    }

    Text {
        visible: root.custom
        anchors.centerIn: parent
        anchors.verticalCenterOffset: root.opticalOffset
        anchors.horizontalCenterOffset: root.custom
            ? LogoGlyphs.xOffset(root.mark) * root.size / 18 : 0
        text: root.custom ? LogoGlyphs.glyph(root.mark) : ""
        color: root.color
        font.family: root.custom ? LogoGlyphs.family(root.mark) : Theme.fontJp
        font.pixelSize: root.glyphSize
        font.weight: root.weight
    }
}

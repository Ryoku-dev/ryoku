import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import "../../registry/registry.js" as Registry
import "../../registry/numbering.js" as Numbering

// the sheet's masthead over the page, in the smallest mono: the page's number and where it
// lives, then the page's place in Settings, the window manager and bar, and the clock. A tick
// ruler runs under it whose marker stands where the page is scrolled to
Item {
    id: root

    required property var hub
    // how far the page is read, 0..1, or -1 for a page that does not scroll
    property real progress: -1
    // room kept at the right end for the window's close button
    property real endRoom: 48

    readonly property var page: Registry.page(root.hub.pageId)
    readonly property string number: Numbering.pageNumber(root.hub.pageId, root.hub.gate)
    readonly property string where: {
        if (!root.page)
            return I18n.tr("Home");
        var c = Registry.category(root.page.category);
        var cat = c ? I18n.tr(c.label) : "";
        var title = I18n.tr(root.page.title);
        return cat === title ? title : cat + "  /  " + title;
    }
    readonly property var place: Numbering.ordinal(root.hub.pageId, root.hub.gate)
    readonly property string clock: {
        var d = Ambient.now;
        var two = n => (n < 10 ? "0" : "") + n;
        return two(d.getHours()) + ":" + two(d.getMinutes()) + (Ambient.on ? ":" + two(d.getSeconds()) : "");
    }
    readonly property var facts: {
        var out = [];
        if (root.place.at > 0)
            out.push("P " + Numbering.pad(root.place.at) + "/" + Numbering.pad(root.place.of));
        if (Settings.provider)
            out.push(String(Settings.provider).toUpperCase());
        var bar = Settings.get("barStyle");
        if (bar)
            out.push(String(bar).toUpperCase());
        out.push(root.clock);
        return out;
    }

    implicitHeight: 34

    Row {
        id: lead
        x: 0
        anchors.verticalCenter: words.verticalCenter
        spacing: 10

        Label {
            text: "§ " + root.number
            typeRole: "monoSm"
            ink: Tone.accent
            font.letterSpacing: 0.8
        }
        Label {
            text: root.where
            typeRole: "monoSm"
            inkRole: "fg2"
            font.capitalization: Font.AllUppercase
            font.letterSpacing: 1.4
        }
    }

    Item {
        id: words
        y: 8
        width: root.width
        height: 14
    }

    Row {
        anchors.right: parent.right
        anchors.rightMargin: root.endRoom
        anchors.verticalCenter: words.verticalCenter
        spacing: 12
        layoutDirection: Qt.LeftToRight

        Repeater {
            model: root.facts
            delegate: Row {
                required property string modelData
                required property int index
                spacing: 12
                Rectangle {
                    visible: index > 0
                    anchors.verticalCenter: parent.verticalCenter
                    width: 3
                    height: 3
                    color: Lithic.fg3
                }
                Label {
                    text: modelData
                    typeRole: "monoSm"
                    inkRole: index === root.facts.length - 1 ? "fg2" : "fg3"
                    font.letterSpacing: 1.2
                }
            }
        }
    }

    // the ruler: a tick every 8 px, every fifth one tall, drawn once per size or ink
    Canvas {
        id: ruler
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.rightMargin: root.endRoom
        anchors.bottom: parent.bottom
        anchors.bottomMargin: 2
        height: 7
        property color ink: Lithic.vein
        onInkChanged: requestPaint()
        onWidthChanged: requestPaint()
        onPaint: {
            var c = getContext("2d");
            c.reset();
            c.fillStyle = ruler.ink;
            for (var i = 0; i * 8 <= width; i++) {
                var tall = i % 5 === 0;
                c.fillRect(i * 8, tall ? 0 : 3, 1, tall ? 7 : 4);
            }
            c.fillRect(0, 6, width, 1);
        }
    }

    // where the page is read to
    Rectangle {
        visible: root.progress >= 0
        anchors.bottom: ruler.bottom
        x: ruler.x + Math.max(0, Math.min(1, root.progress)) * Math.max(0, ruler.width - width)
        width: 24
        height: 3
        color: Tone.accent
        Behavior on x {
            NumberAnimation { duration: Tone.quick; easing.type: Easing.OutCubic }
        }
    }
}

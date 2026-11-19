import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../schema/style.js" as StyleSchema

// Ryoku's own notification card or pop-up as it moves on the desktop: the
// recording of the design in use, lying on the page itself with no ground of its
// own. A stamp and a caption say it is an example. It plays only while Settings
// shows it, and holds its first frame, the card at rest, under reduced motion.
Column {
    id: root

    // a recording made at twice its size: art/osd/ by bin/art/osd-previews,
    // art/notifications/ by bin/art/notification-previews
    required property url source
    // the caption, already translated; %1 is the design's name
    required property string caption
    readonly property string designName: {
        const entry = StyleSchema.designs.find(d => d.value === Lithic.design);
        return I18n.tr(entry ? entry.label : "Lithic");
    }

    spacing: LithicTokens.space.s2

    Item {
        width: parent.width
        height: Math.min(320, Math.max(LithicTokens.space.s6, motion.implicitHeight / 2))
            + LithicTokens.space.s4 * 2

        AnimatedImage {
            id: motion
            anchors.centerIn: parent
            width: Math.min(parent.width, implicitWidth / 2)
            height: Math.min(parent.height - LithicTokens.space.s4 * 2, implicitHeight / 2)
            source: root.source
            fillMode: Image.PreserveAspectFit
            asynchronous: true
            cache: false
            smooth: true
            mipmap: true
            playing: status === Image.Ready && root.visible && root.Window.window !== null
                && root.Window.window.visible && !Lithic.reduceMotion
            onPlayingChanged: if (!playing && status === Image.Ready) currentFrame = 0
        }

        // a stamp in the corner, as on the Style page's sample, so it never reads as a control
        Label {
            anchors.right: parent.right
            anchors.top: parent.top
            text: I18n.tr("Example")
            typeRole: "group"
            inkRole: "fg3"
            font.capitalization: Font.AllUppercase
            font.letterSpacing: 1.6
        }
    }

    Label {
        width: parent.width
        text: root.caption.arg(root.designName)
        typeRole: "caption"
        inkRole: "fg2"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }
}

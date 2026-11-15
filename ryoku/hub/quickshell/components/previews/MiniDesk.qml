import QtQuick
import Lithic
import "../../Singletons"

// the ground a page's preview stands on: a small desktop washed in the palette's own colours,
// so a see-through window has something to show through it
Rectangle {
    id: root

    default property alias scene: stage.data

    // a screen: square in Lithic, rounded as the design rounds its cards
    radius: Tone.flat ? 0 : Lithic.cornerRadius(Tone.cardCut, width, height)
    clip: true
    gradient: Gradient {
        orientation: Gradient.Horizontal
        GradientStop { position: 0; color: Lithic.mix(Lithic.lapis, Lithic.ground, Lithic.dark ? 0.55 : 0.35) }
        GradientStop { position: 1; color: Lithic.mix(Tone.accent, Lithic.ground, Lithic.dark ? 0.5 : 0.3) }
    }
    border.width: 1
    border.color: Tone.line

    // a soft light across the desk, so it reads as a picture rather than a fill
    Rectangle {
        x: root.width * 0.35
        y: -root.height * 0.6
        width: root.width
        height: root.height * 1.6
        radius: width / 2
        rotation: -18
        color: Lithic.alpha(Lithic.jade, 0.22)
    }

    Item {
        id: stage
        anchors.fill: parent
    }
}

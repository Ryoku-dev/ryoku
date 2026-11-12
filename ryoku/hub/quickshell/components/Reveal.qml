import QtQuick
import Lithic

// a group's entrance, ryoku.dev's "rise": it fades in and settles from a few pixels below,
// once, `order` stagger steps after the first group. Only its opacity and a translate move,
// so nothing lays out again and no glyph is redrawn. The start state is written when it
// plays, never before: a group it never reaches simply stands, and reduced motion never arms it
SequentialAnimation {
    id: reveal

    property Item item: null
    property int order: 0
    readonly property Translate shift: Translate {}

    function play() {
        if (Lithic.reduceMotion || !reveal.item || reveal.running)
            return;
        reveal.item.transform = [reveal.shift];
        reveal.shift.y = LithicTokens.size.workPaneRise;
        reveal.item.opacity = 0;
        reveal.start();
    }

    PauseAnimation {
        duration: reveal.order * LithicTokens.motion.revealStagger
    }
    ParallelAnimation {
        NumberAnimation {
            target: reveal.item
            property: "opacity"
            to: 1
            duration: LithicTokens.motion.navigate
            easing.type: Easing.BezierSpline
            easing.bezierCurve: Lithic.curve
        }
        NumberAnimation {
            target: reveal.shift
            property: "y"
            to: 0
            duration: LithicTokens.motion.navigate
            easing.type: Easing.BezierSpline
            easing.bezierCurve: Lithic.curve
        }
    }
}

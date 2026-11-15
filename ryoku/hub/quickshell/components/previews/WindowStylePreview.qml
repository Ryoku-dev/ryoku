import QtQuick
import Lithic
import "../../Singletons"

// Window style's preview: a window you are using in front of one you are not, in the corners,
// frame, see-through, dimming and shadow set on the page, following each change as it is made
MiniDesk {
    id: root

    function v(key, fallback) {
        Changes.revision;
        var said = Changes.value({ store: "desktop", key: "appearance." + key });
        return said === undefined || said === null ? fallback : said;
    }
    readonly property bool follows: root.v("borderFollowsPalette", true) === true
    readonly property bool dims: root.v("dimInactive", false) === true

    MiniWindow {
        x: parent.width * 0.08
        y: parent.height * 0.14
        width: parent.width * 0.5
        height: parent.height * 0.56
        active: false
        radius: root.v("rounding", 0)
        border: root.v("borderSize", 0)
        borderColor: root.follows ? Lithic.fg3 : root.v("inactiveBorder", "#808080")
        see: root.v("inactiveOpacity", 1)
        dim: root.dims ? root.v("dimStrength", 0.5) * 0.7 : 0
        shadow: root.v("shadowEnabled", false) === true
        shadowRange: root.v("shadowRange", 0)
        shadowX: root.v("shadowOffsetX", 0)
        shadowY: root.v("shadowOffsetY", 0)
        shadowColor: root.v("shadowColor", "#000000")
    }

    MiniWindow {
        x: parent.width * 0.36
        y: parent.height * 0.3
        width: parent.width * 0.56
        height: parent.height * 0.6
        active: true
        radius: root.v("rounding", 0)
        border: root.v("borderSize", 0)
        borderColor: root.follows ? Tone.accent : root.v("activeBorder", "#ffffff")
        see: root.v("activeOpacity", 1)
        shadow: root.v("shadowEnabled", false) === true
        shadowRange: root.v("shadowRange", 0)
        shadowX: root.v("shadowOffsetX", 0)
        shadowY: root.v("shadowOffsetY", 0)
        shadowColor: root.v("shadowColor", "#000000")
    }
}

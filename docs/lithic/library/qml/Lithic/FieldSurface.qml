import QtQuick

CutRect {
    id: root

    property bool hovered: false
    property bool pressed: false
    property bool focused: false
    property bool invalid: false
    property real restCut: LithicTokens.cut.mid

    fill: hovered ? Lithic.mix(Lithic.field, Lithic.fg, LithicTokens.alpha.hoverPlane) : Lithic.field
    strokeColor: invalid ? Lithic.jasper : focused ? Lithic.fg2 : Lithic.edge
    strokeWidth: LithicTokens.size.hairline
    cut: restCut + (pressed ? Lithic.pressCutGrow : 0)

    ChiselColorBehavior on fill {
        duration: root.hovered ? LithicTokens.motion.hoverIn : LithicTokens.motion.hoverOut
    }
    ChiselNumberBehavior on cut {
        duration: LithicTokens.motion.press
    }
}

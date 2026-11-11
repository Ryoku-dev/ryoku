import QtQuick

QtObject {
    property string kind: "gold"

    readonly property bool edged: kind === "line" || kind === "dangerLine"
    readonly property bool transparent: edged || kind === "quiet"
    readonly property color base: kind === "on" ? Lithic.sel
        : kind === "danger" ? Lithic.jasper
        : kind === "tonal" ? Lithic.field
        : transparent ? "transparent" : Lithic.gold
    readonly property color ink: kind === "on" ? Lithic.onSel
        : kind === "danger" ? Lithic.inkOn(Lithic.jasper)
        : kind === "dangerLine" ? Lithic.jasper
        : kind === "line" ? Lithic.goldInk
        : kind === "quiet" ? Lithic.fg2
        : kind === "tonal" ? Lithic.fg : Lithic.onGold
    readonly property color stroke: kind === "line" ? Lithic.goldInk
        : kind === "dangerLine" ? Lithic.jasper : "transparent"
    readonly property real strokeWidth: edged ? LithicTokens.size.hairline : 0

    function fill(hovered) {
        if (!hovered)
            return base;
        if (kind === "tonal")
            return Lithic.mix(base, Lithic.fg, LithicTokens.alpha.hoverPlane);
        return transparent ? Lithic.hoverInk(ink) : Lithic.hoverFill(base);
    }
}

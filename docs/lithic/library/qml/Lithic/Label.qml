import QtQuick

Text {
    id: root

    property string typeRole: "body"
    property string inkRole: "fg"
    property color ink: _roleColor(inkRole)
    readonly property var token: Lithic.type[typeRole] || Lithic.type.body

    color: ink
    font.family: token.family
    font.pixelSize: token.pixelSize
    font.weight: token.weight
    lineHeightMode: Text.FixedHeight
    lineHeight: token.lineHeight
    renderType: Text.NativeRendering
    elide: Text.ElideRight

    function _roleColor(name) {
        var value = Lithic[name];
        return value === undefined ? Lithic.fg : value;
    }
}

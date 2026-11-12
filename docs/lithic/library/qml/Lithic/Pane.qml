import QtQuick

Item {
    id: root

    property string kind: "list"
    property real inset: 0
    property color fill: kind === "work" ? Lithic.stone2 : Lithic.stone1
    default property alias content: contentItem.data
    readonly property alias bodyItem: contentItem

    readonly property real contentImplicitWidth: {
        var maximum = 0;
        for (var i = 0; i < contentItem.children.length; ++i)
            maximum = Math.max(maximum, contentItem.children[i].implicitWidth || 0);
        return maximum;
    }
    readonly property real contentImplicitHeight: {
        var maximum = 0;
        for (var i = 0; i < contentItem.children.length; ++i)
            maximum = Math.max(maximum, contentItem.children[i].implicitHeight || 0);
        return maximum;
    }

    implicitWidth: contentImplicitWidth + inset * 2
    implicitHeight: contentImplicitHeight + inset * 2

    // a pane lies on the ground's own tone: its vein hairline draws its cut
    CutRect {
        anchors.fill: parent
        fill: root.fill
        strokeColor: Lithic.vein
        strokeWidth: LithicTokens.size.hairline
        cutTopLeft: LithicTokens.cut.large
        cutTopRight: LithicTokens.cut.large
        cutBottomLeft: 0
        cutBottomRight: 0
    }

    Item {
        id: contentItem
        anchors.fill: parent
        anchors.margins: root.inset
    }
}

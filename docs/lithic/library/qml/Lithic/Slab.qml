import QtQuick

Item {
    id: root

    property real inset: LithicTokens.size.slabInset
    // A toned slab is the one hero plane of a view, dyed by its state
    property string tone: ""
    property color fill: tone !== "" && tone !== "idle" ? Lithic.mineralField(tone) : Lithic.slab
    property real cut: LithicTokens.cut.mid
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

    // a plain slab takes the tone of whatever it stands on, a pane or a dialog, and is drawn
    // by its vein hairline alone; a toned slab, or one a caller filled, is set off by its fill
    readonly property bool _plain: Qt.colorEqual(root.fill, Lithic.slab)
    CutRect {
        anchors.fill: parent
        fill: root._plain ? "transparent" : root.fill
        strokeColor: root._plain ? Lithic.vein : "transparent"
        strokeWidth: LithicTokens.size.hairline
        cut: root.cut
    }

    Item {
        id: contentItem
        anchors.fill: parent
        anchors.margins: root.inset
    }
}

import QtQuick

Item {
    id: root

    property string title: ""
    default property alias content: body.data
    readonly property alias bodyItem: body

    readonly property real contentImplicitWidth: {
        var maximum = 0;
        for (var i = 0; i < body.children.length; ++i)
            maximum = Math.max(maximum, body.children[i].implicitWidth || 0);
        return maximum;
    }
    readonly property real contentImplicitHeight: {
        var maximum = 0;
        for (var i = 0; i < body.children.length; ++i)
            maximum = Math.max(maximum, body.children[i].implicitHeight || 0);
        return maximum;
    }

    implicitWidth: Math.max(titleLabel.implicitWidth, contentImplicitWidth)
    implicitHeight: titleLabel.implicitHeight + LithicTokens.size.slabTitleGap
        + contentImplicitHeight + LithicTokens.size.slabBelow

    Label {
        id: titleLabel
        anchors.left: parent.left
        anchors.top: parent.top
        text: root.title
        typeRole: "section"
    }

    Item {
        id: body
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: titleLabel.bottom
        anchors.topMargin: LithicTokens.size.slabTitleGap
        height: root.contentImplicitHeight
    }
}

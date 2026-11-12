import QtQuick

Item {
    id: root

    property string title: ""
    // a ruled head draws a hairline from the title to the section's far edge, and a mark (an
    // ordinal, a count) stands at its end in mono: the head of a sheet's part
    property bool ruled: false
    property string mark: ""
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
    Label {
        id: markLabel
        visible: root.mark.length > 0
        anchors.right: parent.right
        anchors.verticalCenter: titleLabel.verticalCenter
        text: root.mark
        typeRole: "monoSm"
        inkRole: "fg3"
    }
    Rectangle {
        visible: root.ruled && root.title.length > 0
        anchors.verticalCenter: titleLabel.verticalCenter
        x: titleLabel.width + LithicTokens.space.s3
        width: Math.max(0, (markLabel.visible ? markLabel.x - LithicTokens.space.s3 : root.width) - x)
        height: LithicTokens.size.hairline
        color: Lithic.vein
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

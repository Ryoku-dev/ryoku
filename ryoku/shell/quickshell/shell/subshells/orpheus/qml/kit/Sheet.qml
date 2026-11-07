import QtQuick
import QtQuick.Controls as Controls
import Ryoku.Ui as Ui

Item {
    id: root

    property real pageWidth: 360
    property real maxHeight: 0
    property string title: ""
    property bool backVisible: false
    property list<Item> trailing
    default property alias content: contentColumn.data

    signal back()

    // Trailing items report their own visibility only while the row holding them
    // is shown, so the header never hides on their account.
    readonly property bool headerVisible: backVisible || title.length > 0 || trailing.length > 0
    readonly property real padding: Style.s4
    readonly property real headerHeight: headerVisible
        ? Math.max(32, headerRow.implicitHeight) + Style.s3 : 0
    readonly property real contentHeight: contentColumn.implicitHeight
    readonly property real naturalHeight: padding * 2 + headerHeight + contentHeight
    readonly property real sheetHeight: maxHeight > 0
        ? Math.min(naturalHeight, maxHeight) : naturalHeight
    readonly property real bodyHeight: Math.max(0,
        sheetHeight - padding * 2 - headerHeight)
    readonly property bool scrollable: contentHeight > bodyHeight + 0.5

    implicitWidth: pageWidth
    implicitHeight: naturalHeight
    width: implicitWidth
    height: sheetHeight

    function adoptTrailing() {
        for (let index = 0; index < trailing.length; ++index) {
            const item = trailing[index];
            if (item)
                item.parent = trailingRow;
        }
    }

    onTrailingChanged: adoptTrailing()
    Component.onCompleted: adoptTrailing()

    Item {
        id: headerRow
        visible: root.headerVisible
        x: root.padding
        y: root.padding
        width: Math.max(0, root.width - root.padding * 2)
        implicitHeight: Math.max(backButton.implicitHeight, titleLabel.implicitHeight,
            trailingRow.implicitHeight)
        height: Math.max(32, implicitHeight)

        IconButton {
            id: backButton
            visible: root.backVisible
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            icon: "arrow_back"
            size: 32
            variant: "plain"
            onClicked: root.back()
        }

        Label {
            id: titleLabel
            visible: root.title.length > 0
            anchors.left: backButton.visible ? backButton.right : parent.left
            anchors.leftMargin: backButton.visible ? Style.s2 : 0
            anchors.right: trailingRow.left
            anchors.rightMargin: trailingRow.width > 0 ? Style.s3 : 0
            anchors.verticalCenter: parent.verticalCenter
            text: root.title
            role: "title"
        }

        Row {
            id: trailingRow
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            spacing: Style.s2
        }
    }

    Flickable {
        id: body

        x: root.padding
        y: root.padding + root.headerHeight
        width: Math.max(0, root.width - root.padding * 2)
        height: root.bodyHeight
        contentWidth: width
        contentHeight: contentColumn.implicitHeight
        clip: root.scrollable
        interactive: root.scrollable
        boundsBehavior: Flickable.StopAtBounds
        flickDeceleration: 3000

        Column {
            id: contentColumn
            width: body.width
            spacing: Style.s4
        }

        Ui.WheelScroll {}

        Controls.ScrollBar.vertical: Ui.ScrollRail {
            policy: root.scrollable ? Controls.ScrollBar.AsNeeded : Controls.ScrollBar.AlwaysOff
        }
    }
}

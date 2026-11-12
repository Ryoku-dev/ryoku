import QtQuick
import "../kit"

// one page of the detail side. Every page is built once and kept, so a switch only moves finished layouts:
// material's fade through, the old page gone before the new one shows so their text never overlaps
Item {
    id: pane

    property string title: ""
    property string description: ""
    property bool current: false
    default property alias content: body.data

    opacity: pane.current ? 1 : 0
    visible: pane.opacity > 0.001
    z: pane.current ? 1 : 0
    layer.enabled: pane.opacity > 0.001 && pane.opacity < 0.999
    transform: Translate {
        x: pane.current ? 0 : 28
        Behavior on x {
            enabled: !Md.still
            Spring {}
        }
    }
    Behavior on opacity {
        enabled: !Md.still
        SequentialAnimation {
            PauseAnimation { duration: pane.current ? 90 : 0 }
            NumberAnimation { duration: pane.current ? Md.standard - 90 : 90 }
        }
    }

    function find(item, key) {
        if (item.objectName === key)
            return item;
        for (var i = 0; i < item.children.length; i++) {
            var hit = pane.find(item.children[i], key);
            if (hit)
                return hit;
        }
        return null;
    }
    function reveal(key) {
        var row = key.length > 0 ? pane.find(body, key) : null;
        if (!row) {
            flick.contentY = 0;
            return;
        }
        var at = row.mapToItem(column, 0, 0);
        flick.contentY = Math.max(0, Math.min(flick.contentHeight - flick.height, at.y - 24));
        flash.x = at.x;
        flash.y = at.y;
        flash.width = row.width;
        flash.height = row.height;
        flash.topR = row.topOuter === false ? Md.rXs : Md.rXl;
        flash.bottomR = row.bottomOuter === false ? Md.rXs : Md.rXl;
        pulse.restart();
    }

    Flickable {
        id: flick
        anchors.fill: parent
        contentWidth: width
        contentHeight: column.height + 24
        boundsBehavior: Flickable.StopAtBounds
        clip: true

        Column {
            id: column
            width: flick.width

            Label {
                width: parent.width
                leftPadding: 8
                topPadding: 26
                role: "display"
                font.pixelSize: 32
                text: pane.title
            }
            Label {
                width: parent.width
                leftPadding: 8
                topPadding: 6
                bottomPadding: 10
                role: "body"
                color: Md.inkMuted
                wrapMode: Text.Wrap
                elide: Text.ElideNone
                text: pane.description
            }
            Column {
                id: body
                width: parent.width
                spacing: Md.seam
            }
        }

        Rectangle {
            id: flash
            property real topR: Md.rXl
            property real bottomR: Md.rXl
            topLeftRadius: flash.topR
            topRightRadius: flash.topR
            bottomLeftRadius: flash.bottomR
            bottomRightRadius: flash.bottomR
            color: Md.accent
            opacity: 0

            SequentialAnimation {
                id: pulse
                PauseAnimation { duration: Md.still ? 0 : Md.standard }
                NumberAnimation { target: flash; property: "opacity"; to: 0.22; duration: Md.still ? 0 : Md.standard }
                PauseAnimation { duration: 700 }
                NumberAnimation { target: flash; property: "opacity"; to: 0; duration: Md.still ? 0 : Md.morph }
            }
        }
    }
}

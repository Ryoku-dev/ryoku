import QtQuick

Item {
    id: root

    property var spans: []
    property string typeRole: "body"

    readonly property real naturalWidth: {
        var total = 0;
        for (var i = 0; i < spanRepeater.count; ++i) {
            var item = spanRepeater.itemAt(i);
            if (item)
                total += item.implicitWidth;
        }
        return total;
    }
    readonly property real naturalHeight: {
        var maximum = 0;
        for (var i = 0; i < spanRepeater.count; ++i) {
            var item = spanRepeater.itemAt(i);
            if (item)
                maximum = Math.max(maximum, item.implicitHeight);
        }
        return maximum;
    }

    implicitWidth: naturalWidth
    implicitHeight: Math.max(naturalHeight, spanFlow.implicitHeight)

    Flow {
        id: spanFlow
        width: root.width > 0 ? root.width : root.naturalWidth
        spacing: 0

        Repeater {
            id: spanRepeater
            model: root.spans
            delegate: Label {
                required property var modelData
                text: modelData.text || ""
                typeRole: modelData.mono ? "mono" : root.typeRole
                ink: modelData.attention ? Lithic.amber : Lithic.fg
            }
        }
    }
}

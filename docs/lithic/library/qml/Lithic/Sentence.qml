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

    // A mono span is smaller than the text around it; every span sits on the
    // tallest one's baseline, as text does on the web and in Gio.
    readonly property real _baseline: {
        var maximum = 0;
        for (var i = 0; i < spanRepeater.count; ++i) {
            var item = spanRepeater.itemAt(i);
            if (item)
                maximum = Math.max(maximum, item.spanBaseline);
        }
        return maximum;
    }
    readonly property real _lineHeight: {
        var maximum = 0;
        for (var i = 0; i < spanRepeater.count; ++i) {
            var item = spanRepeater.itemAt(i);
            if (item)
                maximum = Math.max(maximum, _baseline - item.spanBaseline + item.spanHeight);
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
            delegate: Item {
                id: spanBox
                required property var modelData
                readonly property real spanBaseline: span.baselineOffset
                readonly property real spanHeight: span.implicitHeight

                implicitWidth: span.implicitWidth
                implicitHeight: root._lineHeight

                Label {
                    id: span
                    y: root._baseline - span.baselineOffset
                    text: spanBox.modelData.text || ""
                    typeRole: spanBox.modelData.mono ? "mono" : root.typeRole
                    inkRole: "fg"
                }
            }
        }
    }
}

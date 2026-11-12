import QtQuick

Item {
    id: root

    property var traits: []
    property real maximumWidth: -1
    readonly property var orderedTraits: _ordered()

    implicitWidth: marksRow.implicitWidth
    implicitHeight: marksRow.implicitHeight
    clip: maximumWidth >= 0

    function _ordered() {
        var first = [];
        var rest = [];
        var order = ["Disposable", "Instant", "Sealed"];
        for (var i = 0; i < order.length; ++i) {
            if (traits.indexOf(order[i]) !== -1)
                first.push(order[i]);
        }
        for (var j = 0; j < traits.length; ++j) {
            if (order.indexOf(traits[j]) === -1)
                rest.push(traits[j]);
        }
        return first.concat(rest);
    }

    // Widths in the detail type; traits are set apart by space, not a glyph
    function _through(index) {
        var width = 0;
        for (var i = 0; i <= index; ++i)
            width += metrics.advanceWidth(orderedTraits[i]);
        return width + index * marksRow.spacing;
    }

    FontMetrics {
        id: metrics
        font.family: Lithic.type.detail.family
        font.pixelSize: Lithic.type.detail.pixelSize
        font.weight: Lithic.type.detail.weight
    }

    Row {
        id: marksRow
        spacing: LithicTokens.space.s3

        Repeater {
            model: root.orderedTraits
            delegate: Label {
                required property string modelData
                required property int index
                visible: root.maximumWidth < 0 || root._through(index) <= root.maximumWidth
                text: modelData
                typeRole: "detail"
                ink: modelData === "Disposable" ? Lithic.amber : Lithic.fg2
            }
        }
    }
}

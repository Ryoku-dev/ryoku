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

    function _piece(index) {
        return (index === 0 ? "" : " · ") + orderedTraits[index];
    }

    function _through(index) {
        var text = "";
        for (var i = 0; i <= index; ++i)
            text += _piece(i);
        return metrics.advanceWidth(text);
    }

    FontMetrics {
        id: metrics
        font.family: Lithic.type.mono.family
        font.pixelSize: Lithic.type.mono.pixelSize
        font.weight: Lithic.type.mono.weight
    }

    Row {
        id: marksRow

        Repeater {
            model: root.orderedTraits
            delegate: Row {
                required property string modelData
                required property int index
                visible: root.maximumWidth < 0 || root._through(index) <= root.maximumWidth

                Label {
                    visible: index > 0
                    text: " · "
                    typeRole: "mono"
                    inkRole: "fg3"
                }
                Label {
                    text: modelData
                    typeRole: "mono"
                    ink: modelData === "Disposable" ? Lithic.amber : Lithic.fg2
                }
            }
        }
    }
}

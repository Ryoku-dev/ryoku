import QtQuick

Text {
    id: label

    property string role: "body"

    readonly property var spec: ({
        display: { size: 34, weight: Font.Normal },
        headline: { size: 22, weight: Font.Medium },
        title: { size: 16, weight: Font.DemiBold },
        body: { size: 14, weight: Font.Normal },
        label: { size: 13, weight: Font.Medium },
        caption: { size: 12, weight: Font.Normal }
    })[label.role] || { size: 14, weight: Font.Normal }

    color: Md.ink
    font.family: Md.font
    font.pixelSize: label.spec.size
    font.weight: label.spec.weight
    elide: Text.ElideRight
    verticalAlignment: Text.AlignVCenter
}

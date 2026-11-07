pragma ComponentBehavior: Bound

import QtQuick
import "../.." as Pill
import shell.services
import "../../../../components"

// The single quiet-card primitive every section of the weather surface is built
// from: a transparent surface at the widget radius, a hairline outline, a lit
// top edge, and the 8px inner padding of the pill idiom. An optional
// eyebrow (the vermilion tick, 力 seal, mono label) names the section; the
// default content stacks below it on the 4px rhythm.
Item {
    id: root

    property real s: 1
    property string eyebrow: ""
    property real spacing: 8 * root.s

    default property alias content: body.data

    implicitWidth: frame.width
    implicitHeight: frame.implicitHeight

    Rectangle {
        id: frame
        width: root.width
        radius: Theme.radiusWidget
        color: "transparent"
        border.width: Theme.borderWidth
        border.color: Theme.outline
        implicitHeight: stack.implicitHeight + 2 * (Theme.paddingMd * root.s)

        SumiEdge { radius: Theme.radiusWidget }

        Column {
            id: stack
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            anchors.margins: Theme.paddingMd * root.s
            spacing: 10 * root.s

            Eyebrow {
                visible: root.eyebrow.length > 0
                label: root.eyebrow
                mark: false
                s: root.s
            }

            Column {
                id: body
                width: parent.width
                spacing: root.spacing
            }
        }
    }
}

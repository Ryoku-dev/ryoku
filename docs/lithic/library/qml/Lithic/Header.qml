import QtQuick

Item {
    id: root

    property string name: ""
    property string proof: ""
    property string note: ""
    property var traits: []
    property var actions: []
    property Component mark: null
    property Component below: null

    signal actionTriggered(int index)

    implicitWidth: LithicTokens.size.settingsMediumMax
    implicitHeight: LithicTokens.size.headerTop + belowLoader.y
        + (belowLoader.active ? belowLoader.implicitHeight : 0)
        + LithicTokens.size.headerBottom

    Item {
        id: content
        x: LithicTokens.size.headerSide
        y: LithicTokens.size.headerTop
        width: root.width - LithicTokens.size.headerSide * 2
        height: root.height - y - LithicTokens.size.headerBottom

        Item {
            id: identity
            anchors.left: parent.left
            anchors.right: parent.right
            height: Math.max(LithicTokens.size.headerMark, nameLabel.implicitHeight)

            Loader {
                id: markLoader
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                width: LithicTokens.size.headerMark
                height: LithicTokens.size.headerMark
                sourceComponent: root.mark
            }

            Label {
                id: nameLabel
                anchors.left: markLoader.right
                anchors.leftMargin: LithicTokens.size.headerMarkGap
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                text: root.name
                typeRole: root.width < LithicTokens.size.headerNameCompactAt
                    ? "sentence"
                    : "name"
                maximumLineCount: 1
            }
        }

        Label {
            id: proofLabel
            visible: root.proof !== ""
            anchors.left: parent.left
            anchors.right: parent.right
            y: identity.y + identity.height + LithicTokens.size.headerProofGap
            text: root.proof
            typeRole: "proof"
            inkRole: "fg2"
            wrapMode: Text.Wrap
        }

        Label {
            id: noteLabel
            visible: root.note !== ""
            anchors.left: parent.left
            anchors.right: parent.right
            y: (proofLabel.visible ? proofLabel.y + proofLabel.implicitHeight
                                   : identity.y + identity.height)
               + LithicTokens.size.headerNoteGap
            text: root.note
            typeRole: "body"
            inkRole: "fg3"
            wrapMode: Text.Wrap
        }

        Marks {
            id: marks
            visible: root.traits.length > 0
            y: (noteLabel.visible ? noteLabel.y + noteLabel.implicitHeight
                                  : proofLabel.visible
                                    ? proofLabel.y + proofLabel.implicitHeight
                                    : identity.y + identity.height)
               + LithicTokens.size.headerMarksGap
            width: parent.width
            maximumWidth: width
            traits: root.traits
        }

        Row {
            id: actionRow
            visible: root.actions.length > 0
            y: (marks.visible ? marks.y + marks.implicitHeight
                              : noteLabel.visible
                                ? noteLabel.y + noteLabel.implicitHeight
                                : proofLabel.visible
                                  ? proofLabel.y + proofLabel.implicitHeight
                                  : identity.y + identity.height)
               + LithicTokens.size.headerActionsGap
            spacing: LithicTokens.space.s2

            Repeater {
                model: root.actions
                delegate: Plaque {
                    required property var modelData
                    required property int index
                    text: modelData.text || ""
                    kind: modelData.kind || (index === 0 ? "gold" : "line")
                    glyph: modelData.glyph || ""
                    busy: modelData.busy || false
                    enabled: modelData.enabled === undefined || modelData.enabled
                    onClicked: root.actionTriggered(index)
                }
            }
        }

        Loader {
            id: belowLoader
            active: root.below !== null
            sourceComponent: root.below
            anchors.left: parent.left
            anchors.right: parent.right
            y: (actionRow.visible ? actionRow.y + actionRow.implicitHeight
                                  : marks.visible
                                    ? marks.y + marks.implicitHeight
                                    : noteLabel.visible
                                      ? noteLabel.y + noteLabel.implicitHeight
                                      : proofLabel.visible
                                        ? proofLabel.y + proofLabel.implicitHeight
                                        : identity.y + identity.height)
               + (active ? LithicTokens.size.headerContentGap : 0)
        }
    }
}

import QtQuick
import Ryoku.Ui.Singletons
import "lib/store.js" as StoreLogic

// The compact app card: an icon and its name read at a glance, so a marketplace
// row holds several apps instead of one oversized cover.
Item {
    id: card

    required property var item
    property bool selected: false
    property bool focusVisible: false
    property bool reducedMotion: false
    property bool hovered: hover.hovered
    property bool pressed: tap.pressed

    signal activated()

    readonly property string title: String(item && (item.name || item.id) || "")
    readonly property string summary: String(item && item.summary || "")
    readonly property bool verified: String(item && item.verificationStatus || "").toLowerCase() === "verified"
    readonly property var status: StoreLogic.statusLabels(item)
    readonly property string statusTag: StoreLogic.statusTag(item)
    readonly property color fallbackSurface: item && item.surface ? item.surface : Tokens.paperLift
    readonly property string initials: StoreLogic.initials(item, title)

    Accessible.role: Accessible.Button
    Accessible.name: [title, summary, status.map(s => I18n.tr(s)).join(", ")].filter(Boolean).join(", ")
    Accessible.onPressAction: card.activated()

    Rectangle {
        id: frame
        anchors.fill: parent
        anchors.margins: Tokens.s2
        radius: Tokens.radius
        color: card.selected ? Tokens.tint5
              : (card.pressed ? Tokens.tint16 : (card.hovered ? Tokens.tint5 : Tokens.paper))
        border.width: card.focusVisible || card.selected ? Tokens.border * 2 : Tokens.border
        border.color: card.focusVisible ? Tokens.bone
                      : (card.selected || card.hovered ? Tokens.lineStrong : Tokens.line)
        scale: card.pressed ? 0.994 : 1

        Behavior on color {
            enabled: !card.reducedMotion
            ColorAnimation { duration: Tokens.snap }
        }
        Behavior on border.color {
            enabled: !card.reducedMotion
            ColorAnimation { duration: Tokens.snap }
        }
        Behavior on scale {
            enabled: !card.reducedMotion
            NumberAnimation { duration: Tokens.snap; easing.type: Tokens.easeSnap }
        }

        Row {
            anchors.fill: parent
            anchors.margins: Tokens.s3
            spacing: Tokens.s3

            ProductMedia {
                width: Tokens.px(56)
                height: Tokens.px(56)
                anchors.verticalCenter: parent.verticalCenter
                source: card.item && card.item.art || ""
                mode: "icon"
                active: card.enabled && card.Window.window && card.Window.window.active
                fallbackText: card.initials
                fallbackSurface: card.fallbackSurface
                fallbackInk: Tokens.inkMuted
            }

            Column {
                width: parent.width - Tokens.px(56) - parent.spacing
                anchors.verticalCenter: parent.verticalCenter
                spacing: Tokens.s1

                Item {
                    width: parent.width
                    height: Math.max(nameText.implicitHeight, statusText.implicitHeight)

                    Text {
                        id: nameText
                        anchors {
                            left: parent.left
                            right: card.verified ? verifiedMark.left : statusText.left
                            rightMargin: Tokens.s2
                            verticalCenter: parent.verticalCenter
                        }
                        text: card.title
                        color: Tokens.ink
                        font.family: Tokens.display
                        font.pixelSize: Tokens.fRow
                        font.weight: Font.Medium
                        elide: Text.ElideRight
                    }

                    Text {
                        id: verifiedMark
                        anchors {
                            right: statusText.left
                            rightMargin: Tokens.s2
                            verticalCenter: parent.verticalCenter
                        }
                        text: "✓"
                        visible: card.verified
                        color: Tokens.inkDim
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fMicro
                    }

                    Text {
                        id: statusText
                        anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                        text: card.statusTag === "" ? "" : I18n.tr(card.statusTag)
                        visible: text !== ""
                        color: StoreLogic.isUnavailable(card.item) ? Tokens.alert : Tokens.inkMuted
                        font.family: Tokens.mono
                        font.pixelSize: Tokens.fTiny
                        font.letterSpacing: Tokens.trackLabel
                    }
                }

                Text {
                    width: parent.width
                    text: card.summary
                    visible: text !== ""
                    color: Tokens.inkDim
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                    maximumLineCount: 2
                    wrapMode: Text.Wrap
                    elide: Text.ElideRight
                }
            }
        }
    }

    HoverHandler {
        id: hover
        cursorShape: Qt.PointingHandCursor
    }

    TapHandler {
        id: tap
        onTapped: card.activated()
    }
}

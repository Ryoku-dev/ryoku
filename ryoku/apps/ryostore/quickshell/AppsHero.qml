import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "lib/store.js" as StoreLogic

// The featured shelf at the top of the Apps catalogue: one app at a time, with
// arrows and dots as the only controls, so it reads as a shop front.
Item {
    id: hero

    property var items: []
    property int currentIndex: 0
    property bool reducedMotion: false

    signal activated(var item)

    readonly property int index: items.length > 0
            ? Math.max(0, Math.min(items.length - 1, currentIndex)) : 0
    readonly property var current: items.length > 0 ? items[index] : ({})
    readonly property bool hasMultiple: items.length > 1
    readonly property bool hovered: hover.hovered
    readonly property string title: String(current.name || current.id || "")
    readonly property string summary: String(current.summary || "")
    readonly property string categoryTag: current.tags && current.tags.length
            ? String(current.tags[0]).toUpperCase() : ""
    readonly property string statusTag: StoreLogic.statusTag(current)
    readonly property bool verified: String(current.verificationStatus || "").toLowerCase() === "verified"
    readonly property color fallbackSurface: current.surface || Tokens.paperLift
    readonly property string initials: StoreLogic.initials(current, title)

    activeFocusOnTab: true
    Accessible.role: Accessible.Button
    Accessible.name: [title, summary].filter(Boolean).join(", ")
    Accessible.onPressAction: hero.activated(hero.current)

    component Chip: Text {
        color: Tokens.inkMuted
        font.family: Tokens.mono
        font.pixelSize: Tokens.fTiny
        font.letterSpacing: Tokens.trackLabel
    }

    function step(delta) {
        if (items.length === 0)
            return;
        currentIndex = (index + delta + items.length) % items.length;
    }

    Keys.onPressed: event => {
        if (event.key === Qt.Key_Left) {
            step(-1);
        } else if (event.key === Qt.Key_Right) {
            step(1);
        } else if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter) {
            activated(current);
        } else {
            return;
        }
        event.accepted = true;
    }

    onItemsChanged: if (currentIndex >= items.length) currentIndex = 0

    Rectangle {
        id: frame
        anchors.fill: parent
        radius: Tokens.radius
        color: hero.hovered ? Tokens.tint5 : Tokens.paperLift
        border.width: hero.activeFocus ? Tokens.border * 2 : Tokens.border
        border.color: hero.activeFocus ? Tokens.bone
                      : (hero.hovered ? Tokens.lineStrong : Tokens.line)
        scale: tap.pressed ? 0.996 : 1

        Behavior on color {
            enabled: !hero.reducedMotion
            ColorAnimation { duration: Tokens.snap }
        }
        Behavior on border.color {
            enabled: !hero.reducedMotion
            ColorAnimation { duration: Tokens.snap }
        }
        Behavior on scale {
            enabled: !hero.reducedMotion
            NumberAnimation { duration: Tokens.snap; easing.type: Tokens.easeSnap }
        }

        Row {
            anchors {
                left: parent.left; leftMargin: Tokens.s5
                top: parent.top; topMargin: Tokens.s5
                bottom: parent.bottom; bottomMargin: Tokens.s5
                right: arrows.left; rightMargin: Tokens.s4
            }
            spacing: Tokens.s5

            ProductMedia {
                width: Tokens.px(104)
                height: Tokens.px(104)
                anchors.verticalCenter: parent.verticalCenter
                source: hero.current.art || ""
                mode: "icon"
                active: hero.visible && hero.Window.window && hero.Window.window.active
                fallbackText: hero.initials
                fallbackSurface: hero.fallbackSurface
                fallbackInk: Tokens.inkMuted
            }

            Column {
                width: parent.width - Tokens.px(104) - parent.spacing
                anchors.verticalCenter: parent.verticalCenter
                spacing: Tokens.s2

                Text {
                    text: I18n.tr("FEATURED APPS") + "  /  "
                          + String(hero.index + 1).padStart(2, "0") + " / "
                          + String(hero.items.length).padStart(2, "0")
                    color: Tokens.inkDim
                    font.family: Tokens.mono
                    font.pixelSize: Tokens.fMicro
                    font.letterSpacing: Tokens.trackMark
                }

                Text {
                    width: parent.width
                    text: hero.title
                    color: Tokens.ink
                    font.family: Tokens.display
                    font.pixelSize: Tokens.fHero
                    font.weight: Font.Medium
                    elide: Text.ElideRight
                }

                Text {
                    width: parent.width
                    text: hero.summary
                    visible: text !== ""
                    color: Tokens.inkDim
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                    maximumLineCount: 2
                    wrapMode: Text.Wrap
                    elide: Text.ElideRight
                }

                Row {
                    spacing: Tokens.s3

                    Chip {
                        text: hero.categoryTag
                        visible: text !== ""
                    }
                    Chip {
                        text: I18n.tr("VERIFIED")
                        visible: hero.verified
                    }
                    Chip {
                        text: hero.statusTag === "" ? "" : I18n.tr(hero.statusTag)
                        visible: text !== ""
                        color: StoreLogic.isUnavailable(hero.current) ? Tokens.alert : Tokens.ink
                    }
                }
            }
        }

        Column {
            id: arrows
            anchors {
                right: parent.right; rightMargin: Tokens.s4
                verticalCenter: parent.verticalCenter
                verticalCenterOffset: -Tokens.s3
            }
            spacing: Tokens.s2

            IconBtn {
                width: 30
                height: 30
                glyph: "‹"
                armed: hero.hasMultiple
                onAct: hero.step(-1)
            }
            IconBtn {
                width: 30
                height: 30
                glyph: "›"
                armed: hero.hasMultiple
                onAct: hero.step(1)
            }
        }

        Row {
            anchors { horizontalCenter: parent.horizontalCenter; bottom: parent.bottom; bottomMargin: Tokens.s3 }
            spacing: Tokens.s2
            visible: hero.hasMultiple

            Repeater {
                model: hero.items.length
                delegate: Rectangle {
                    required property int index
                    width: 4
                    height: 4
                    radius: 2
                    color: index === hero.index ? Tokens.ink : Tokens.lineStrong
                    HoverHandler { cursorShape: Qt.PointingHandCursor }
                    TapHandler { margin: Tokens.s1; onTapped: hero.currentIndex = index }
                }
            }
        }

        HoverHandler {
            id: hover
            cursorShape: Qt.PointingHandCursor
        }
        TapHandler {
            id: tap
            onTapped: hero.activated(hero.current)
        }
    }
}

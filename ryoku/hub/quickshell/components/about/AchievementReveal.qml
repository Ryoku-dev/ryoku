import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import ".."

FocusScope {
    id: reveal

    property var badge: null
    property bool opened: false
    property real burst: 0
    readonly property color metal: badge ?
        Progression.rarityColours[badge.rarity] || Lithic.gold : Lithic.gold

    visible: opened
    z: 400
    focus: opened

    function present(entry) {
        if (!entry) return;
        badge = entry;
        burst = 0;
        opened = true;
        forceActiveFocus();
        if (!Lithic.reduceMotion) entrance.restart();
        else burst = 1;
    }

    function dismiss() {
        opened = false;
        badge = null;
    }

    Keys.onEscapePressed: event => {
        reveal.dismiss();
        event.accepted = true;
    }

    SequentialAnimation {
        id: entrance
        NumberAnimation {
            target: reveal
            property: "burst"
            from: 0
            to: 1
            duration: 820
            easing.type: Easing.OutCubic
        }
    }

    Rectangle {
        anchors.fill: parent
        color: "#07090d"
        opacity: 0.93
        MouseArea {
            anchors.fill: parent
            onClicked: reveal.dismiss()
        }
    }

    Item {
        anchors.centerIn: parent
        width: Math.min(parent.width - 32, 530)
        height: Math.min(parent.height - 24, 585)
        scale: Lithic.reduceMotion ? 1 : 0.87 + reveal.burst * 0.13
        opacity: Lithic.reduceMotion ? 1 : Math.min(1, reveal.burst * 2)

        CutRect {
            anchors.fill: parent
            cut: 30
            fill: "#10131a"
            strokeColor: reveal.metal
            strokeWidth: 2
        }

        Repeater {
            model: 4
            delegate: Rectangle {
                required property int index
                width: parent.width * (0.30 + index * 0.14)
                height: width
                anchors.horizontalCenter: parent.horizontalCenter
                y: 65 - index * 16
                rotation: 45 + (Lithic.reduceMotion ? 0 : reveal.burst * (index % 2 ? -12 : 12))
                color: "transparent"
                border.color: reveal.metal
                border.width: 1
                opacity: 0.28 - index * 0.04
            }
        }

        Column {
            anchors.fill: parent
            anchors.margins: 22
            spacing: 12

            Label {
                anchors.horizontalCenter: parent.horizontalCenter
                text: reveal.badge && reveal.badge.category === "Classified" ?
                    I18n.tr("CLASSIFIED / DECRYPTED") :
                    reveal.badge && reveal.badge.category === "Mastery" ?
                    I18n.tr("MASTERY / ACHIEVED") : I18n.tr("THE ARCHIVE REMEMBERS")
                typeRole: "mono"
                ink: reveal.metal
            }
            Label {
                anchors.horizontalCenter: parent.horizontalCenter
                text: I18n.tr("ACHIEVEMENT UNLOCKED")
                typeRole: "monoSm"
                inkRole: "fg2"
            }

            Item {
                width: parent.width
                height: Math.min(245, Math.max(150, reveal.height * 0.4))

                ArchiveSeal {
                    badgeId: reveal.badge ? reveal.badge.id : ""
                    anchors.centerIn: parent
                    width: Math.min(230, parent.height)
                    height: width
                    title: reveal.badge ? reveal.badge.title : ""
                    sigil: reveal.badge ? reveal.badge.sigil : "力"
                    rarity: reveal.badge ? reveal.badge.rarity : "common"
                    category: reveal.badge ? reveal.badge.category : "RYOKU"
                    animateFoil: reveal.opened && reveal.badge &&
                        reveal.badge.rarity === "holographic" && !Lithic.reduceMotion
                }
            }
            Label {
                width: parent.width
                text: reveal.badge ? I18n.tr(reveal.badge.title) : ""
                typeRole: "title"
                horizontalAlignment: Text.AlignHCenter
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Label {
                width: parent.width
                text: reveal.badge ? reveal.badge.rarity.toUpperCase() + " / +" +
                    reveal.badge.reward + " ◆" : ""
                typeRole: "mono"
                ink: reveal.metal
                horizontalAlignment: Text.AlignHCenter
            }
            Label {
                width: parent.width
                text: I18n.tr("Collected. Your badge is now available to feature or place on your profile.")
                typeRole: "detail"
                inkRole: "fg2"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
                horizontalAlignment: Text.AlignHCenter
            }
            Item { width: 1; height: 4 }
            Plaque {
                anchors.horizontalCenter: parent.horizontalCenter
                kind: "gold"
                text: I18n.tr("Return to archive")
                onClicked: reveal.dismiss()
            }
        }
    }
}

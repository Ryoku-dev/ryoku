pragma ComponentBehavior: Bound

import QtQuick
import "."
import Ryoku.Ui.Singletons

Item {
    id: tab

    property var kit
    property string query: ""
    property bool current: false
    readonly property int matches: -1
    readonly property var rail: [
        { id: "test", label: I18n.tr("Test"), count: -1 },
        { id: "history", label: I18n.tr("History"), count: -1 },
        { id: "stats", label: I18n.tr("Stats"), count: -1 },
        { id: "settings", label: I18n.tr("Settings"), count: -1 }
    ]
    property string railCurrent: "test"
    property bool loadedOnce: false
    property int focusPulse: 0
    signal requestClose()

    function activate() {
        loadedOnce = true;
        focusPulse++;
    }

    function handleEscape() {
        if (railCurrent !== "test") {
            railCurrent = "test";
            Qt.callLater(activate);
            return true;
        }
        return false;
    }

    onCurrentChanged: if (current) activate()

    Item {
        anchors.fill: parent
        opacity: tab.loadedOnce && content.status !== Loader.Ready ? 1 : 0
        visible: opacity > 0
        Behavior on opacity {
            enabled: !Tokens.reduceMotion
            NumberAnimation { duration: Tokens.durSmall; easing.type: Easing.OutCubic }
        }

        Column {
            anchors.centerIn: parent
            spacing: Tokens.s2
            Text {
                anchors.horizontalCenter: parent.horizontalCenter
                text: I18n.tr("PREPARING TEST")
                color: Tokens.inkDim
                font.family: Tokens.mono
                font.pixelSize: Tokens.fMicro
                font.letterSpacing: Tokens.trackLabel
            }
            Text {
                anchors.horizontalCenter: parent.horizontalCenter
                text: I18n.tr("Loading words and keyboard")
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
            }
        }
    }

    Loader {
        id: content
        anchors.fill: parent
        active: tab.loadedOnce
        asynchronous: true
        onLoaded: if (tab.current) tab.focusPulse++
        sourceComponent: TypingContent {
            kit: tab.kit
            current: tab.current
            section: tab.railCurrent
            focusPulse: tab.focusPulse
            onSectionChanged: if (tab.railCurrent !== section) tab.railCurrent = section
        }
    }
}

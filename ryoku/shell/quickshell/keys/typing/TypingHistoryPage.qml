pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Ryoku.Ui
import Ryoku.Ui.Singletons

Item {
    id: history

    required property var store

    function describe(entry) {
        if (entry.mode === "time") return entry.modeValue + "s";
        if (entry.mode === "words") return I18n.tr("%1 words").arg(entry.modeValue);
        return I18n.tr("zen");
    }

    function when(timestamp) {
        const date = new Date(Number(timestamp));
        return Qt.formatDateTime(date, "MMM d  hh:mm");
    }

    ColumnLayout {
        anchors.fill: parent
        spacing: Tokens.s4

        RowLayout {
            Layout.fillWidth: true
            spacing: Tokens.s6
            Repeater {
                model: [
                    { label: I18n.tr("TESTS"), value: history.store.testsCompleted },
                    { label: I18n.tr("AVERAGE WPM"), value: history.store.results.length
                        ? Math.round(history.store.results.reduce((sum, item) => sum + Number(item.wpm || 0), 0) / history.store.results.length) : 0 },
                    { label: I18n.tr("AVERAGE ACCURACY"), value: history.store.results.length
                        ? Math.round(history.store.results.reduce((sum, item) => sum + Number(item.accuracy || 0), 0) / history.store.results.length) + "%" : "0%" }
                ]
                delegate: ColumnLayout {
                    required property var modelData
                    spacing: 0
                    Text { text: parent.modelData.label; color: Tokens.inkMuted; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny; font.letterSpacing: Tokens.trackLabel }
                    Text { text: String(parent.modelData.value); color: Tokens.ink; font.family: Tokens.mono; font.pixelSize: Tokens.fValue }
                }
            }
            Item { Layout.fillWidth: true }
            Btn { text: I18n.tr("Clear history"); armed: history.store.results.length > 0; onAct: history.store.clearHistory() }
        }

        Rectangle { Layout.fillWidth: true; Layout.preferredHeight: 1; color: Tokens.lineSoft }

        Empty {
            Layout.alignment: Qt.AlignCenter
            visible: history.store.results.length === 0
            caption: history.store.historyEnabled
                ? I18n.tr("Finish a test to build your local history.")
                : I18n.tr("History is disabled in typing settings.")
        }

        ListView {
            id: list
            visible: history.store.results.length > 0
            Layout.fillWidth: true
            Layout.fillHeight: true
            clip: true
            spacing: Tokens.s1
            model: history.store.results
            ScrollRail.vertical: ScrollRail { }
            WheelScroll { }
            delegate: Rectangle {
                id: resultRow
                required property var modelData
                required property int index
                property bool entered: Tokens.reduceMotion
                width: list.width
                height: 46
                radius: Tokens.radius
                color: index % 2 ? Tokens.tint5 : "transparent"
                border.width: Tokens.border
                border.color: Tokens.lineSoft
                opacity: entered ? 1 : 0
                transform: Translate {
                    y: resultRow.entered ? 0 : Tokens.s2
                    Behavior on y {
                        enabled: !Tokens.reduceMotion
                        NumberAnimation { duration: Tokens.durSmall; easing.type: Easing.OutCubic }
                    }
                }
                Behavior on opacity {
                    enabled: !Tokens.reduceMotion
                    NumberAnimation { duration: Tokens.durSmall; easing.type: Easing.OutCubic }
                }
                Timer {
                    interval: Math.min(160, Math.max(1, resultRow.index * 18))
                    running: history.visible && !Tokens.reduceMotion && !resultRow.entered
                    repeat: false
                    onTriggered: resultRow.entered = true
                }
                RowLayout {
                    anchors.fill: parent
                    anchors.leftMargin: Tokens.s4
                    anchors.rightMargin: Tokens.s4
                    spacing: Tokens.s5
                    Text { Layout.preferredWidth: 72; text: Math.round(resultRow.modelData.wpm) + " wpm"; color: Tokens.ink; font.family: Tokens.mono; font.pixelSize: Tokens.fSmall }
                    Text { Layout.preferredWidth: 56; text: Math.round(resultRow.modelData.accuracy) + "%"; color: Tokens.inkDim; font.family: Tokens.mono; font.pixelSize: Tokens.fSmall }
                    Text { Layout.preferredWidth: 78; text: history.describe(resultRow.modelData); color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall }
                    Text { Layout.fillWidth: true; text: String(resultRow.modelData.language).replace("_extended", ""); color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall; elide: Text.ElideRight }
                    Text { text: history.when(resultRow.modelData.timestamp); color: Tokens.inkMuted; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny }
                }
            }
        }
    }
}

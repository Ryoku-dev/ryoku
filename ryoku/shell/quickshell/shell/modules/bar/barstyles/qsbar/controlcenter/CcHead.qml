import QtQuick
import "../modules"
import Ryoku.Ui
import Ryoku.Ui.Singletons

// The body's head: the panel's fixed eyebrow (QS BAR // SETTINGS) in tracked
// mono, then the route's name in the display face with its kanji seal, and the
// route's one-line summary. A running-head marginalia strip dresses the top-right
// corner beside the close mark, so the summary line owns the full width and is
// never cut by chrome. It carries no live state readout: every route already
// opens with a live figure of its own (the bar silhouette, the lanes, the
// widget at bar density, the dock), so the head only has to name where you are.
Item {
    id: head

    property var root
    property var tk
    property string title: ""
    property string gloss: ""
    property string desc: ""
    property int index: 0
    signal closed()

    implicitHeight: head.tk ? head.tk.headH : 80

    // ── the eyebrow, name and summary ──
    UiText {
        id: eyebrow
        anchors.left: parent.left
        anchors.top: parent.top
        anchors.topMargin: head.tk ? head.tk.gap / 2 : 6
        text: I18n.tr("QS BAR // SETTINGS")
        color: Tokens.inkFaint
        font.family: Tokens.mono
        font.pixelSize: Tokens.fMicro
        font.letterSpacing: Tokens.trackMark
        font.weight: Font.DemiBold
    }

    // The name row is sized by the display face; the seal hangs on its baseline
    // (not the other way round, which lifted the name up into the eyebrow).
    Item {
        id: name
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: eyebrow.bottom
        anchors.topMargin: 2
        height: nameText.implicitHeight

        UiText {
            id: nameText
            anchors.left: parent.left
            text: I18n.tr(head.title)
            color: Tokens.ink
            font.family: Tokens.display
            font.pixelSize: Tokens.fHero
        }
        UiText {
            id: glossText
            anchors.left: nameText.right
            anchors.leftMargin: head.tk ? head.tk.gap : 12
            anchors.baseline: nameText.baseline
            text: UiPreferences.text(head.gloss, "")
            color: Tokens.inkFaint
            font.family: Tokens.jp
            font.pixelSize: Tokens.fBody
        }
    }

    UiText {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: name.bottom
        anchors.topMargin: 2
        text: I18n.tr(head.desc)
        color: Tokens.inkFaint
        font.family: Tokens.ui
        font.pixelSize: Tokens.fSmall
        elide: Text.ElideRight
    }

    // ── the running head: a printed marginalia strip beside the close mark ──
    Marginalia {
        id: marg
        anchors.right: shut.left
        anchors.rightMargin: head.tk ? head.tk.gap : 12
        anchors.verticalCenter: shut.verticalCenter
        kana: head.gloss
        index: String(head.index + 1).padStart(2, "0")
        label: String(head.title).toUpperCase()
        chevrons: false
    }

    // ── close ──
    Rectangle {
        id: shut
        anchors.right: parent.right
        anchors.top: parent.top
        width: Tokens.ctlH
        height: Tokens.ctlH
        radius: Tokens.radius
        color: shutMa.containsMouse ? Tokens.tint5 : "transparent"
        Behavior on color { ColorAnimation { duration: Tokens.snap } }

        UiText {
            anchors.centerIn: parent
            text: "\u2715"
            color: shutMa.containsMouse ? Tokens.ink : Tokens.inkFaint
            font.family: Tokens.mono
            font.pixelSize: Tokens.fSmall
        }
        MouseArea {
            id: shutMa
            anchors.fill: parent
            hoverEnabled: true
            cursorShape: Qt.PointingHandCursor
            onClicked: head.closed()
        }
    }

    Rectangle {
        id: rule
        anchors { bottom: parent.bottom; left: parent.left; right: parent.right }
        height: 1
        color: Tokens.lineSoft
    }
}

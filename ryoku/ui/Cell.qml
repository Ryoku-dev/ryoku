import QtQuick
import Ryoku.Ui.Singletons

// One setting, drawn. A cell never places itself: Section flows it, and its
// span comes from Spans.of(kind, optionCount), so adding a setting cannot
// require a layout decision.
//
// The control's footprint is reserved by the text column rather than drawn on
// top of it -- that is what makes overlap impossible instead of merely tuned
// away. Controls that need their own band (gallery, multi, chips) declare more
// rows and the text stops above them.
Item {
    id: cell

    property string label: ""
    property string desc: ""
    property string value: ""
    property string unit: ""
    property string def: ""
    property string source: ""
    property bool changed: value !== def
    property bool block: false      // control gets its own band under the text
    property int footH: 0           // reserved bar band at the cell foot (pick)
    property int controlWidth: 0    // reserved when inline

    // the height this cell actually needs: block cells hug their control (a
    // gallery is as tall as its tiles, a lone chip row stays a row), inline
    // cells keep the fixed cell height plus any reserved foot band. The sheet
    // binds to this, so a control can never overflow into the next cell.
    readonly property real neededHeight: cell.block
        ? Tokens.s3 + txtCol.height + Tokens.s3 + slot.height + Tokens.s3
        : Tokens.cellH + cell.footH
    default property alias control: slot.data

    readonly property bool hovered: hh.hovered

    implicitHeight: Tokens.cellH

    Rectangle {
        anchors.fill: parent
        radius: Tokens.radius
        color: hh.hovered ? Qt.rgba(205 / 255, 196 / 255, 186 / 255, 0.05) : "transparent"
        border.width: Tokens.border
        border.color: hh.hovered ? Tokens.lineStrong : Tokens.line
        Behavior on color { ColorAnimation { duration: Tokens.snap } }
        Behavior on border.color { ColorAnimation { duration: Tokens.snap } }
    }
    HoverHandler { id: hh }

    // changed reads as a solid edge. no colour is spent on it.
    Rectangle {
        visible: cell.changed
        x: 0
        y: Tokens.s2
        width: 2
        height: parent.height - Tokens.s4
        color: Tokens.ink
    }

    // which file this lands in, parked in the corner off the reading path
    Text {
        anchors { right: parent.right; top: parent.top; margins: Tokens.s3 }
        visible: cell.source !== ""
        text: cell.source.replace(".json", "")
        color: Tokens.inkFaint
        font.family: Tokens.mono
        font.pixelSize: Tokens.fTiny
        opacity: hh.hovered ? 1 : 0.8
        Behavior on opacity { NumberAnimation { duration: Tokens.snap } }
    }

    Column {
        id: txtCol
        anchors { left: parent.left; top: parent.top; margins: Tokens.s3 }
        anchors.leftMargin: Tokens.s4
        width: Math.max(0, cell.width - Tokens.s4 - Tokens.s3 - ((cell.block || cell.footH > 0) ? 0 : cell.controlWidth + Tokens.s4) - 46)
        spacing: 0

        Text {
            width: parent.width
            text: I18n.tr(cell.label)
            color: Tokens.inkMuted
            font.family: Tokens.ui
            font.pixelSize: 10
            font.weight: Font.Medium
            font.letterSpacing: Tokens.trackLabel
            elide: Text.ElideRight
        }
        Row {
            spacing: Tokens.s1
            Text {
                id: vTxt
                text: cell.value
                color: Tokens.ink
                font.family: Tokens.ui
                font.pixelSize: cell.value.length > 8 ? 18 : Tokens.fValue
                font.weight: Font.Light
                // a long value (a file path) elides in the middle instead of
                // running over the neighbouring cell.
                width: Math.min(implicitWidth, txtCol.width
                    - (uTxt.visible ? uTxt.implicitWidth + Tokens.s1 : 0)
                    - (dTxt.visible ? dTxt.implicitWidth + Tokens.s1 : 0))
                elide: Text.ElideMiddle
            }
            Text {
                id: uTxt
                visible: cell.unit !== ""
                text: cell.unit
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: 10
                anchors.bottom: parent.bottom
                anchors.bottomMargin: 5
            }
            Text {
                id: dTxt
                visible: cell.changed && cell.def !== ""
                text: cell.def + (cell.unit ? " " + cell.unit : "")
                color: Tokens.inkFaint
                font.family: Tokens.mono
                font.pixelSize: Tokens.fTiny
                font.strikeout: true
                anchors.bottom: parent.bottom
                anchors.bottomMargin: 6
            }
        }
        Item { width: 1; height: 2 }
        Text {
            width: parent.width
            text: I18n.tr(cell.desc)
            color: Tokens.inkMuted
            font.family: Tokens.ui
            font.pixelSize: 12
            wrapMode: Text.WordWrap
            maximumLineCount: 2
            elide: Text.ElideRight
        }
    }

    Item {
        id: slot
        readonly property bool banded: cell.block || cell.footH > 0
        anchors.right: parent.right
        anchors.rightMargin: Tokens.s4
        anchors.verticalCenter: slot.banded ? undefined : parent.verticalCenter
        anchors.bottom: slot.banded ? parent.bottom : undefined
        anchors.bottomMargin: Tokens.s3
        anchors.left: slot.banded ? parent.left : undefined
        anchors.leftMargin: Tokens.s4
        // a block control's band is its content's implicit height (a Flow
        // reports it from its width), never a guessed row count; reading the
        // loader's implicit size avoids the childrenRect feedback loop. A foot
        // control (pick) gets a fixed full-width bar band instead.
        height: cell.block
            ? Math.max(Tokens.ctlH, slot.children.length > 0 ? slot.children[0].implicitHeight : 0)
            : Tokens.ctlH
        width: slot.banded ? undefined : cell.controlWidth
    }
}

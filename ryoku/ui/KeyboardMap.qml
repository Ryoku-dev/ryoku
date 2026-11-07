pragma ComponentBehavior: Bound
import QtQuick
import Ryoku.Ui.Singletons

// A live keyboard diagram: it draws a keyboard and reflects the caller's input
// settings -- the layout's letter legends (AZERTY, QWERTZ, Dvorak, Colemak, else
// QWERTY), and every key a remap touches lit to bone: Caps Lock's new job, a
// swapped Alt/Super, the Compose key, the layout-switch chord. Read only, it
// shows what the controls did; it never edits. Ink diagram, bone for a remapped
// key (the sheet's one emphasis mechanism, no colour spent).
Item {
    id: kmap

    property string layoutCode: "us"
    property string layoutName: I18n.tr("English (US)")
    property string styleName: ""
    property string capsFn: ""          // caps:escape | ctrl:nocaps | caps:swapescape | caps:none | ""
    property bool swapAltSuper: false
    property string composeKey: ""      // compose:ralt | compose:menu | ""
    property string switchChord: ""     // grp:alt_shift_toggle | grp:win_space_toggle | ""
    property bool numlock: false

    // compact pins the map as a slim strip: smaller caps, no number row, no
    // note -- just the letter/modifier rows that carry the remaps.
    property real keyMax: 46
    property bool compact: false

    // letter-row legends per layout family; only the three letter rows differ.
    readonly property var legends: ({
        "qwerty":  ["QWERTYUIOP", "ASDFGHJKL", "ZXCVBNM"],
        "azerty":  ["AZERTYUIOP", "QSDFGHJKLM", "WXCVBN"],
        "qwertz":  ["QWERTZUIOP", "ASDFGHJKL", "YXCVBNM"],
        "dvorak":  ["'\u002c.PYFGCRL", "AOEUIDHTNS", ";QJKXBMWVZ"],
        "colemak": ["QWFPGJLUY;", "ARSTDHNEIO", "ZXCVBKM"]
    })
    // Which family a layout code belongs to. The variant decides first, because
    // a French Dvorak is a Dvorak; only then does the country code speak.
    // ch is split: its default is QWERTZ but ch(fr) is AZERTY, and the same
    // holds for be and lu variants, so the variant is checked for a French
    // marker before the code is trusted.
    readonly property var azertyCodes: ["fr", "be", "lu", "ma", "cm", "cd", "cf", "ci", "sn", "tg", "bj", "ml", "ne", "bf"]
    readonly property var qwertzCodes: ["de", "at", "ch", "cz", "sk", "hu", "si", "hr", "rs", "ba", "me", "mk", "al", "li"]
    readonly property string family: {
        var v = kmap.styleName.toLowerCase();
        if (v.indexOf("dvorak") >= 0) return "dvorak";
        if (v.indexOf("colemak") >= 0) return "colemak";
        if (v.indexOf("azerty") >= 0) return "azerty";
        if (v.indexOf("qwertz") >= 0) return "qwertz";
        var c = kmap.layoutCode.toLowerCase();
        // a French variant of an otherwise QWERTZ country (ch, be, lu) is AZERTY
        if (v.indexOf("french") >= 0 || v.indexOf("fr") === 0)
            return "azerty";
        if (kmap.azertyCodes.indexOf(c) >= 0) return "azerty";
        if (kmap.qwertzCodes.indexOf(c) >= 0) return "qwertz";
        return "qwerty";
    }
    readonly property var rows: kmap.legends[kmap.family] || kmap.legends["qwerty"]

    function capsLabel() {
        return kmap.capsFn === "caps:escape" ? "Esc"
            : kmap.capsFn === "ctrl:nocaps" ? "Ctrl"
            : kmap.capsFn === "caps:swapescape" ? "\u21c4Esc"
            : kmap.capsFn === "caps:none" ? "-"
            : I18n.tr("Caps");
    }

    readonly property real gap: kmap.compact ? 3 : 4
    readonly property real u: Math.min(kmap.keyMax, Math.floor((width - 14 * gap) / 15))
    readonly property real keyH: kmap.u
    readonly property int rowCount: kmap.compact ? 4 : 5
    implicitHeight: (kmap.compact ? 0 : head.height + Tokens.s4) + rowCount * keyH + (rowCount - 1) * gap

    // one key: a hairline cap; a remapped key inverts to bone.
    component Key: Rectangle {
        property string cap: ""
        property bool lit: false
        property real units: 1
        width: kmap.u * units + kmap.gap * (units - 1)
        height: kmap.keyH
        radius: Tokens.radius
        color: lit ? Tokens.bone : "transparent"
        border.width: Tokens.border
        border.color: lit ? Tokens.bone : Tokens.line
        Text {
            anchors.centerIn: parent
            text: parent.cap
            color: parent.lit ? Tokens.inkOnBone : Tokens.inkDim
            font.family: Tokens.ui
            font.pixelSize: parent.cap.length > 2 ? 9 : 12
            font.weight: parent.lit ? Font.Medium : Font.Normal
        }
    }

    // head: the resolved layout name, and the read-key note.
    Row {
        id: head
        visible: !kmap.compact
        anchors { left: parent.left; right: parent.right; top: parent.top }
        spacing: Tokens.s4
        Column {
            spacing: 1
            Text {
                text: I18n.tr("LAYOUT"); color: Tokens.inkMuted; font.family: Tokens.ui
                font.pixelSize: 10; font.weight: Font.Medium; font.letterSpacing: Tokens.trackLabel
            }
            Text {
                text: kmap.layoutName + (kmap.styleName !== "" && kmap.styleName !== "Default" ? "  \u00b7  " + kmap.styleName : "")
                color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: 15; font.weight: Font.Light
            }
        }
        Item { width: 1; height: 1 }
        Text {
            visible: !kmap.compact
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr("\u003c\u003c  lit keys are your remaps")
            color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: 10; font.letterSpacing: 1.2
        }
    }

    Column {
        anchors { left: parent.left; right: parent.right; top: kmap.compact ? parent.top : head.bottom; topMargin: kmap.compact ? 0 : Tokens.s4 }
        spacing: kmap.gap

        // number row
        Row {
            visible: !kmap.compact
            anchors.horizontalCenter: parent.horizontalCenter
            spacing: kmap.gap
            Key { cap: "`" }
            Repeater { model: ["1", "2", "3", "4", "5", "6", "7", "8", "9", "0", "-", "="]
                Key { required property var modelData; cap: modelData } }
            Key { cap: I18n.tr("Bksp"); units: 2 }
        }
        // top letter row
        Row {
            anchors.horizontalCenter: parent.horizontalCenter
            spacing: kmap.gap
            Key { cap: "Tab"; units: 1.5 }
            Repeater { model: kmap.rows[0].length
                Key { required property int index; cap: kmap.rows[0].charAt(index) } }
        }
        // home row: Caps carries its remap
        Row {
            anchors.horizontalCenter: parent.horizontalCenter
            spacing: kmap.gap
            Key { cap: kmap.capsLabel(); lit: kmap.capsFn !== ""; units: 1.75 }
            Repeater { model: kmap.rows[1].length
                Key { required property int index; cap: kmap.rows[1].charAt(index) } }
            Key { cap: "Enter"; units: 2 }
        }
        // bottom letter row: left Shift lights for the Alt+Shift switch chord
        Row {
            anchors.horizontalCenter: parent.horizontalCenter
            spacing: kmap.gap
            Key { cap: "Shift"; lit: kmap.switchChord === "grp:alt_shift_toggle"; units: 2 }
            Repeater { model: kmap.rows[2].length
                Key { required property int index; cap: kmap.rows[2].charAt(index) } }
            Key { cap: "Shift"; units: 2 }
        }
        // modifier row: swap, compose, and the switch chord all read here
        Row {
            anchors.horizontalCenter: parent.horizontalCenter
            spacing: kmap.gap
            Key { cap: "Ctrl"; units: 1.25 }
            Key { cap: kmap.swapAltSuper ? "Alt" : "Super"
                  lit: kmap.swapAltSuper || kmap.switchChord === "grp:win_space_toggle"; units: 1.25 }
            Key { cap: kmap.swapAltSuper ? "Super" : "Alt"
                  lit: kmap.swapAltSuper || kmap.switchChord === "grp:alt_shift_toggle"; units: 1.25 }
            Key { cap: I18n.tr("Space"); lit: kmap.switchChord === "grp:win_space_toggle"; units: 6 }
            Key { cap: kmap.composeKey === "compose:ralt" ? I18n.tr("Compose") : (kmap.swapAltSuper ? "Super" : "Alt")
                  lit: kmap.composeKey === "compose:ralt"; units: 1.25 }
            Key { cap: kmap.swapAltSuper ? "Alt" : "Super"; lit: kmap.swapAltSuper; units: 1.25 }
            Key { cap: kmap.composeKey === "compose:menu" ? I18n.tr("Compose") : I18n.tr("Menu")
                  lit: kmap.composeKey === "compose:menu"; units: 1.25 }
            Key { cap: "Ctrl"; units: 1.25 }
        }
    }
}

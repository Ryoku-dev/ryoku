import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import ".."

// A role seal in the certificate tradition: a disc with the role spelled around its rim, its code
// running clockwise round a diamond, and one mark at the centre. Everything is drawn flat in two
// inks, so it stays crisp from a card corner up to the certificate.
Item {
    id: root
    property string role: "tester"
    property var spec: null
    property bool live: false
    property real size: 256
    property bool showText: true

    readonly property bool founder: root.role === "founder"
    readonly property bool customMark: !!(root.spec && root.spec.emblem)
    readonly property bool ready: !root.customMark || centreArt.status === Image.Ready
    readonly property bool moving: root.live && root.visible && !Lithic.reduceMotion
    // a seal is an object, not part of the theme: each role keeps the ink its Discord role wears, on the
    // same near-black, whatever the wallpaper colours are
    readonly property color accent: root.spec && root.spec.accent ? root.spec.accent
        : ({ tester: "#a274e0", contributor: "#e08a3c", nixos: "#7eb3e0", core: "#d9c08c", founder: "#f25623" })[root.role] || "#a274e0"
    readonly property color paper: "#efe6da"
    readonly property color disc: Qt.tint("#0d0b0a", Qt.rgba(root.accent.r, root.accent.g, root.accent.b, 0.08))
    readonly property color faint: Qt.rgba(root.paper.r, root.paper.g, root.paper.b, 0.26)
    // the rim words ride this circle, inside the ruler ticks and outside the diamond's corner stops
    readonly property real rimRadius: root.size * 0.395
    readonly property var roleText: ({
        tester: { code: "TESTER", name: "TESTER", kanji: "試験" },
        contributor: { code: "CONTRIB", name: "CONTRIBUTOR", kanji: "貢献" },
        nixos: { code: "NIXOS", name: "NIXOS MAINTAINER", kanji: "雪" },
        core: { code: "CORE", name: "CORE MAINTAINER", kanji: "核心" },
        founder: { code: "ORIGIN", name: "FOUNDER", kanji: "力の源" }
    })
    readonly property var words: root.roleText[root.role] || root.roleText.tester
    readonly property string code: (root.spec && root.spec.title ? root.spec.title : root.words.code).toUpperCase()
    readonly property string kanji: root.spec && root.spec.kanji ? root.spec.kanji : root.words.kanji
    readonly property string centreGlyph: ({ tester: "試", contributor: "貢", nixos: "雪", core: "核" })[root.role] || ""
    // one slow turn of the ruler rim and one pass of light round the code while the seal is in view
    property real turn: 0
    property real pass: -1

    width: root.size
    height: root.size
    implicitWidth: root.size
    implicitHeight: root.size
    Accessible.role: Accessible.Graphic
    Accessible.name: "Ryoku " + root.words.name.toLowerCase()

    NumberAnimation on turn {
        from: 0; to: 360; duration: 90000
        loops: Animation.Infinite
        running: root.moving
    }
    SequentialAnimation on pass {
        loops: Animation.Infinite
        running: root.moving && root.showText
        NumberAnimation { from: -1; to: 5; duration: 2600; easing.type: Easing.InOutSine }
        PauseAnimation { duration: 5200 }
    }

    // the disc and its hairline
    Rectangle {
        anchors.fill: parent
        radius: width / 2
        color: root.disc
        border.width: Math.max(1, root.size * 0.006)
        border.color: Lithic.alpha(root.accent, root.founder ? 0.85 : 0.5)
    }
    Rectangle {
        visible: root.founder
        anchors.centerIn: parent
        width: root.size * 0.9
        height: width
        radius: width / 2
        color: "transparent"
        border.width: Math.max(1, root.size * 0.003)
        border.color: Lithic.alpha(root.accent, 0.45)
    }

    // the ruler rim, the card code's own ticks bent round the seal
    Item {
        anchors.fill: parent
        rotation: root.turn
        Repeater {
            model: 96
            delegate: Rectangle {
                id: tick
                required property int index
                readonly property bool major: index % 8 === 0
                x: root.size / 2 - width / 2
                y: root.size * 0.018
                width: Math.max(1, root.size * (major ? 0.007 : 0.004))
                height: root.size * (major ? 0.034 : index % 2 ? 0.014 : 0.022)
                color: major && root.founder ? root.accent : Lithic.alpha(root.accent, major ? 0.7 : 0.38)
                transform: Rotation { origin.x: tick.width / 2; origin.y: root.size / 2 - root.size * 0.018; angle: tick.index * 360 / 96 }
            }
        }
    }

    // rim words: RYOKU and the kanji over the top either side of the mark, the role along the bottom
    component ArcText: Item {
        id: arc
        property string text
        property real radius
        property real centreAngle
        property bool underside: false
        property real pixelSize
        property color ink
        readonly property real step: (arc.pixelSize * 0.86) / arc.radius * 180 / Math.PI
        anchors.fill: parent
        Repeater {
            model: arc.text.length
            delegate: Text {
                required property int index
                readonly property real angle: arc.underside
                    ? arc.centreAngle + ((arc.text.length - 1) / 2 - index) * arc.step
                    : arc.centreAngle + (index - (arc.text.length - 1) / 2) * arc.step
                readonly property real rad: angle * Math.PI / 180
                x: root.size / 2 + Math.cos(rad) * arc.radius - width / 2
                y: root.size / 2 + Math.sin(rad) * arc.radius - height / 2
                rotation: arc.underside ? angle - 90 : angle + 90
                text: arc.text.charAt(index)
                textFormat: Text.PlainText
                renderType: Text.QtRendering
                color: arc.ink
                font.family: Lithic.font.mono
                font.weight: Font.Bold
                font.pixelSize: arc.pixelSize
            }
        }
    }
    ArcText {
        visible: root.showText
        text: root.words.name
        radius: root.rimRadius
        centreAngle: 90
        underside: true
        pixelSize: root.size * (root.words.name.length > 12 ? 0.052 : 0.06)
        ink: root.founder ? Lithic.alpha(root.accent, 0.75) : root.faint
    }
    ArcText {
        visible: root.showText
        text: "RYOKU"
        radius: root.rimRadius
        centreAngle: 225
        pixelSize: root.size * 0.06
        ink: root.faint
    }
    Text {
        visible: root.showText && UiPreferences.showJapaneseGloss
        readonly property real rad: 315 * Math.PI / 180
        x: root.size / 2 + Math.cos(rad) * root.rimRadius - width / 2
        y: root.size / 2 + Math.sin(rad) * root.rimRadius - height / 2
        rotation: 45
        text: root.kanji
        textFormat: Text.PlainText
        renderType: Text.QtRendering
        color: root.faint
        font.family: "Noto Sans CJK JP"
        font.weight: Font.Black
        font.pixelSize: root.size * (root.kanji.length > 2 ? 0.05 : 0.06)
    }
    RyokuMark {
        visible: root.showText
        anchors.horizontalCenter: parent.horizontalCenter
        y: root.size * 0.07
        width: root.size * 0.085
        height: width
        ink: Qt.rgba(root.paper.r, root.paper.g, root.paper.b, 0.55)
        accent: root.accent
    }
    // stops between the top and bottom words
    Repeater {
        model: root.showText ? [180, 0] : []
        delegate: Rectangle {
            required property var modelData
            readonly property real rad: modelData * Math.PI / 180
            width: root.size * 0.018
            height: width
            rotation: 45
            x: root.size / 2 + Math.cos(rad) * root.rimRadius - width / 2
            y: root.size / 2 + Math.sin(rad) * root.rimRadius - height / 2
            color: root.faint
        }
    }

    // the code four times round a diamond, read clockwise, with a stop past every corner
    Item {
        id: diamond
        readonly property real side: root.size * 0.4
        visible: root.showText
        width: diamond.side
        height: diamond.side
        anchors.centerIn: parent
        rotation: 45
        Repeater {
            model: 4
            delegate: Item {
                id: edge
                required property int index
                width: diamond.side
                height: diamond.side
                rotation: index * 90
                Row {
                    anchors.horizontalCenter: parent.horizontalCenter
                    y: -height / 2
                    spacing: root.size * 0.004
                    Repeater {
                        model: root.code.length
                        delegate: Text {
                            required property int index
                            // the light runs clockwise round the diamond, letter by letter
                            readonly property real lit: Math.max(0, 1 - Math.abs(root.pass - (edge.index + index / root.code.length)) * 3)
                            text: root.code.charAt(index)
                            textFormat: Text.PlainText
                            renderType: Text.QtRendering
                            color: Qt.tint(root.accent, Qt.rgba(root.paper.r, root.paper.g, root.paper.b, 0.55 * lit))
                            font.family: Lithic.font.mono
                            font.weight: Font.ExtraBold
                            font.pixelSize: root.size * (root.code.length > 5 ? 0.056 : 0.068)
                        }
                    }
                }
            }
        }
    }
    Repeater {
        model: root.showText ? 4 : 0
        delegate: Rectangle {
            required property int index
            readonly property real rad: (index * 90 - 90) * Math.PI / 180
            readonly property real reach: root.size * 0.4 * 0.7071 + root.size * 0.035
            width: root.size * 0.022
            height: width
            rotation: 45
            x: root.size / 2 + Math.cos(rad) * reach - width / 2
            y: root.size / 2 + Math.sin(rad) * reach - height / 2
            color: root.accent
        }
    }

    // the centre: a team design's own mark, the Ryoku mark for the founder, else the role's kanji
    Image {
        id: centreArt
        visible: root.customMark
        anchors.centerIn: parent
        width: root.size * 0.27
        height: width
        source: root.customMark ? "file://" + root.spec.emblem : ""
        sourceSize.width: Math.max(1, Math.ceil(centreArt.width * 2))
        sourceSize.height: Math.max(1, Math.ceil(centreArt.height * 2))
        fillMode: Image.PreserveAspectFit
        smooth: true
        mipmap: true
        asynchronous: true
    }
    CutRect {
        visible: !root.customMark && root.founder
        anchors.centerIn: parent
        width: root.size * 0.26
        height: width
        cut: root.size * 0.04
        fill: root.accent
        RyokuMark {
            anchors.centerIn: parent
            width: parent.width * 0.78
            height: width
            ink: root.disc
            accent: root.paper
        }
    }
    // a hanko: the role's kanji knocked out of a cut plate in the role's ink
    CutRect {
        visible: !root.customMark && !root.founder
        anchors.centerIn: parent
        width: root.size * 0.24
        height: width
        cut: root.size * 0.035
        fill: root.accent
        Text {
            anchors.centerIn: parent
            anchors.verticalCenterOffset: -root.size * 0.006
            text: root.centreGlyph
            textFormat: Text.PlainText
            renderType: Text.QtRendering
            color: root.disc
            font.family: "Noto Sans CJK JP"
            font.weight: Font.Black
            font.pixelSize: root.size * 0.17
        }
    }
}

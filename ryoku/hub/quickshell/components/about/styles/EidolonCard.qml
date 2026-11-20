import QtQuick
import QtQuick.Effects
import QtQuick.Shapes
import Quickshell
import Lithic
import Ryoku.Ui.Singletons
import "../../../Singletons"
import "../../badge"
import ".."

Item {
    id: relic
    required property Item host

    width: 630
    height: 880
    clip: false

    readonly property color ink: "#0b0909"
    readonly property color paper: "#f3e9da"
    readonly property color vermilion: "#fb4817"
    readonly property color silver: "#c3bcb3"
    readonly property bool wanted: host.moving && !Lithic.reduceMotion
    readonly property Item figure: figureCarrier
    readonly property bool ready: marble.status === Image.Ready && (!seal.visible || seal.ready)

    property real enterT: 0
    property real intensity: wanted ? 1 : 0
    Behavior on intensity {
        NumberAnimation { duration: relic.wanted ? 280 : 850; easing.type: Easing.OutCubic }
    }
    onWantedChanged: if (wanted) enterT = 0
    FrameAnimation {
        running: relic.wanted
        onTriggered: relic.enterT += frameTime
    }

    readonly property real reveal: Math.min(1, Math.max(0, (enterT - 0.11) / 0.66))
    readonly property real shock: wanted && enterT < 0.75
        ? Math.sin(enterT * 40) * Math.exp(-3.8 * enterT) : 0
    readonly property real xTilt: wanted ? host.foil.x : 0
    readonly property real yTilt: wanted ? host.foil.y : 0
    readonly property string edition: "NIXOS / HYPERION"

    component Inked: Text {
        textFormat: Text.PlainText
        renderType: Text.QtRendering
        color: relic.paper
        font.family: "Inter Display"
        font.weight: Font.Black
        font.italic: true
    }
    component Data: Text {
        textFormat: Text.PlainText
        renderType: Text.QtRendering
        color: relic.paper
        font.family: Lithic.font.mono
        font.weight: Font.DemiBold
        font.pixelSize: 9
        font.letterSpacing: 1.4
    }
    component Press: Shape {
        id: press
        property var points: []
        property color pigment: "transparent"
        property color edge: "transparent"
        property real thickness: 0
        preferredRendererType: Shape.CurveRenderer
        ShapePath {
            fillColor: press.pigment
            strokeColor: press.edge
            strokeWidth: press.thickness
            joinStyle: ShapePath.MiterJoin
            PathPolyline { path: press.points.length ? press.points.concat([press.points[0]]) : [] }
        }
    }

    Item {
        id: printSurface
        anchors.fill: parent
        clip: true

    Rectangle {
        anchors.fill: parent
        color: relic.ink
    }

    // Dense physical-print underlay, not a generic interface background.
    Image {
        id: underprint
        x: -16 * relic.xTilt + 12 * relic.shock
        y: -12 * relic.yTilt - 8 * relic.shock
        width: relic.width + 10
        height: relic.height + 10
        fillMode: Image.Stretch
        source: Qt.resolvedUrl("../../../art/eidolon/vermilion-press.png")
        opacity: 0.70
    }

    Press {
        points: [Qt.point(-38, 135), Qt.point(115, 85), Qt.point(360, 32),
                 Qt.point(540, 89), Qt.point(424, 142), Qt.point(125, 197)]
        pigment: relic.vermilion
        opacity: 0.31
        x: 16 * relic.reveal + 9 * relic.shock
        y: -11 * relic.reveal
        rotation: relic.intensity * 5.5
    }
    Press {
        points: [Qt.point(-18, 340), Qt.point(160, 294), Qt.point(378, 367),
                 Qt.point(441, 427), Qt.point(186, 390), Qt.point(-18, 468)]
        pigment: relic.vermilion
        opacity: 0.42
        x: -19 * relic.reveal + 11 * relic.shock
        y: 12 * relic.reveal
        rotation: relic.intensity * -3.5
    }
    Press {
        points: [Qt.point(102, 76), Qt.point(170, 12), Qt.point(210, -10),
                 Qt.point(145, 190), Qt.point(82, 237)]
        pigment: relic.ink
        opacity: 0.89
    }

    Inked {
        x: -35 + 24 * relic.xTilt + 12 * relic.shock
        y: 42 - 17 * relic.yTilt
        text: "機"
        color: relic.vermilion
        font.family: "Noto Serif CJK JP"
        font.pixelSize: 190
        rotation: -14 + relic.intensity * 8 + relic.shock * 6
        opacity: 0.90
    }


    // Oversized typography collides with the picture. The name is real host data.
    Inked {
        id: oversize
        x: -24 - 17 * relic.reveal + 23 * relic.shock + relic.xTilt * 12
        y: 130 + 13 * relic.yTilt - 7 * relic.reveal
        width: 690
        height: 139
        text: relic.host.name.toUpperCase()
        font.pixelSize: 118
        minimumPixelSize: 65
        fontSizeMode: Text.Fit
        font.letterSpacing: -7
        rotation: -9 - relic.reveal * 5 + relic.shock * 2
        style: Text.Outline
        styleColor: relic.ink
        opacity: 0.98
    }

    // The marble portrait is a cut-out print plate, not an image in a UI card.
    Item {
        id: figureCarrier
        x: 1; y: 96
        width: 628; height: 406
        clip: true
        readonly property bool ready: marble.status === Image.Ready
        Image {
            id: marble
            x: 157 + 28 * relic.xTilt - 23 * relic.shock
            y: 46 - 22 * relic.yTilt - relic.reveal * 10
            width: 555
            height: 585
            fillMode: Image.PreserveAspectFit
            smooth: true
            mipmap: true
            source: Qt.resolvedUrl("../../../art/caracalla.png")
            layer.enabled: relic.intensity > 0.01
            layer.effect: ShaderEffect {
                property vector2d split: Qt.vector2d(
                    (0.0078 + 0.003 * Math.abs(relic.shock)) * relic.intensity,
                    -0.005 * relic.intensity
                )
                property real strength: relic.intensity * Math.min(1, relic.enterT / 0.23)
                fragmentShader: Qt.resolvedUrl("../../../shaders/eidolon-register.frag.qsb")
            }
            scale: 1 + 0.090 * relic.reveal + 0.015 * Math.abs(relic.shock)
            transformOrigin: Item.Center
        }
    }

    Image {
        id: fracture
        anchors.fill: parent
        fillMode: Image.Stretch
        source: Qt.resolvedUrl("../../../art/eidolon/silver-fracture.png")
        opacity: 0.33 + 0.20 * relic.intensity
        x: -9 * relic.xTilt - 11 * relic.shock
        y: 6 * relic.yTilt
    }
    Image {
        id: orangeTop
        x: 19 * relic.xTilt + 17 * relic.shock
        y: 13 * relic.yTilt - 9 * relic.reveal
        width: relic.width + 8
        height: relic.height + 8
        source: Qt.resolvedUrl("../../../art/eidolon/vermilion-press.png")
        opacity: 0.15 + 0.18 * relic.intensity
    }

    // Hand-painted vertical Japanese, rather than meaningless HUD symbols.
    Inked {
        x: 7 - 18 * relic.reveal + 9 * relic.shock; y: 309
        text: "力\nと\n美"
        color: relic.paper
        font.family: "Noto Serif CJK JP"
        font.pixelSize: 58
        font.weight: Font.Black
        font.letterSpacing: -3
        lineHeight: 0.90
        rotation: -6
        style: Text.Outline
        styleColor: relic.ink
    }
    Inked {
        x: 548 + 14 * relic.reveal
        y: 128 - 10 * relic.yTilt
        text: "機\n械\nの\n記\n憶"
        font.family: "Noto Serif CJK JP"
        font.pixelSize: 34
        color: relic.paper
        lineHeight: 0.89
        style: Text.Outline
        styleColor: relic.ink
    }

    Press {
        points: [Qt.point(22, 422), Qt.point(196, 392), Qt.point(300, 456),
                 Qt.point(431, 486), Qt.point(225, 474), Qt.point(28, 493)]
        pigment: relic.vermilion
        opacity: 0.64
        rotation: -relic.intensity * 2
    }

    Repeater {
        model: 4
        delegate: Item {
            required property int index
            x: index % 2 ? 611 : 14
            y: index < 2 ? 13 : 854
            width: 14; height: 14
            Rectangle {
                x: index % 2 ? 12 : 0
                height: 12; width: 2
                color: relic.paper
            }
            Rectangle {
                y: index < 2 ? 0 : 10
                width: 14; height: 2
                color: relic.paper
            }
        }
    }

    // The power number anchors the transition from the chaotic picture to encoded data.
    Press {
        points: [Qt.point(23, 497), Qt.point(585, 490), Qt.point(607, 508),
                 Qt.point(590, 542), Qt.point(25, 541), Qt.point(10, 518)]
        pigment: relic.ink
        opacity: 0.94
    }
    Inked {
        x: 30; y: 490
        text: "力"
        font.family: "Noto Serif CJK JP"
        font.pixelSize: 26
        color: relic.vermilion
    }
    Data {
        x: 66; y: 505
        text: "POWER"
        font.pixelSize: 10
    }
    Inked {
        x: 137; y: 482
        text: String(relic.host.power)
        font.pixelSize: 65
        color: relic.paper
        font.letterSpacing: -5
    }
    Data {
        x: 420; y: 501
        text: relic.host.tier.name.toUpperCase() + "  /  " + relic.host.rarity.points + " PTS"
        font.pixelSize: 12
    }

    // These three quiet mechanical bands leave the machine-readable tick rulers untouched.
    Press {
        points: [Qt.point(28, 564), Qt.point(601, 564), Qt.point(587, 602),
                 Qt.point(45, 602), Qt.point(30, 594)]
        pigment: relic.paper
    }
    Press {
        points: [Qt.point(42, 622), Qt.point(590, 622), Qt.point(605, 636),
                 Qt.point(585, 660), Qt.point(26, 660)]
        pigment: relic.paper
    }
    Press {
        points: [Qt.point(28, 680), Qt.point(604, 681), Qt.point(590, 717),
                 Qt.point(42, 721), Qt.point(24, 710)]
        pigment: relic.paper
    }

    Repeater {
        model: Math.min(3, relic.host.moves.length)
        delegate: Item {
            required property int index
            readonly property var stat: relic.host.moves[index]
            x: 42; y: 567 + index * 58
            width: 543; height: 36
            Data {
                x: 0; y: 5
                text: "0" + (parent.index + 1) + "   " + parent.stat.name.toUpperCase()
                font.pixelSize: 8
                color: relic.ink
            }
            Data {
                x: 0; y: 21
                text: parent.stat.detail
                color: "#484442"
                font.pixelSize: 9
                font.letterSpacing: 0.1
            }
            Inked {
                x: 268; y: -8
                width: 267
                height: 45
                horizontalAlignment: Text.AlignRight
                text: String(parent.stat.value) + " " + String(parent.stat.unit).toUpperCase()
                color: relic.ink
                font.pixelSize: 33
                font.italic: true
                minimumPixelSize: 21
                fontSizeMode: Text.Fit
                font.letterSpacing: -1
            }
            Rectangle {
                x: 242; y: 4
                width: 3; height: 25
                color: relic.vermilion
                rotation: 22
            }
        }
    }

    // Second monumental word, partially off the page like a torn poster.
    Inked {
        x: -24 + relic.reveal * 19 - 10 * relic.shock
        y: 696 - relic.reveal * 10
        width: 716; height: 114
        text: "NIXOS"
        font.pixelSize: 115
        minimumPixelSize: 110
        fontSizeMode: Text.Fit
        font.letterSpacing: 3
        color: relic.paper
        rotation: -5 - 4 * relic.reveal + relic.shock * 2
        style: Text.Outline
        styleColor: relic.ink
    }
    Rectangle { x: 33; y: 838; width: 545; height: 1; color: relic.paper; opacity: 0.76 }
    Data {
        x: 33; y: 846
        width: 365
        text: relic.host.typeLine.toUpperCase()
        elide: Text.ElideRight
        color: relic.paper
        font.pixelSize: 8
    }
    Data {
        x: 438; y: 846
        width: 156
        horizontalAlignment: Text.AlignRight
        text: "NO. " + relic.host.serial.toUpperCase()
        color: relic.silver
        font.pixelSize: 8
    }
    Rectangle {
        x: 342; y: 803
        width: 262; height: 20
        color: relic.ink
        opacity: 0.84
    }
    Data {
        x: 352; y: 806
        text: relic.host.tagline.trim() !== "" ? relic.host.tagline : relic.host.stockTagline
        font.pixelSize: 9
        font.letterSpacing: 0
        font.italic: true
        width: 240
        elide: Text.ElideRight
        color: relic.paper
    }

    }

    // The core seal is a separately printed medallion that breaks the border.
    Item {
        id: coreMedallion
        x: 537 + relic.xTilt * 5
        y: -24 + relic.yTilt * 4
        width: 124
        height: 124
        visible: relic.host.sealed && !!relic.host.badge
        rotation: -12 + relic.reveal * 14 + relic.shock * 12
        scale: 1 + relic.intensity * 0.06
        z: 10

        Rectangle {
            x: 0; y: 0
            width: 124; height: 124
            radius: 62
            color: "#0c090b"
            border.color: relic.ink
            border.width: 4
        }
        Rectangle {
            x: 4; y: 4
            width: 116; height: 116
            radius: 58
            color: "transparent"
            border.color: relic.paper
            border.width: 2
        }
        Rectangle {
            x: 9; y: 9
            width: 106; height: 106
            radius: 53
            color: "transparent"
            border.color: relic.vermilion
            border.width: 1
            opacity: 0.64 + relic.intensity * 0.30
        }

        Emblem {
            id: seal
            x: 12
            y: 12
            size: 100
            role: relic.host.badge ? relic.host.badge.role : "tester"
            spec: relic.host.designSpec ? relic.host.designSpec.badge : null
            live: relic.wanted
        }
    }
}

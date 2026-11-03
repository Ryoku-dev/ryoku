// iRiS login, Lens ("Lente"): the picture out of focus, and the time cut out of it as a lens onto the sharp picture.
// What you type arrives as Material shapes in the accent.
import QtQuick 2.15
import QtQuick.Effects
import "."

LoginSurface {
    id: root
    field: entry.input

    readonly property color ink: "#ffffff"
    readonly property color inkSoft: Qt.rgba(1, 1, 1, 0.74)
    readonly property color inkFaint: Qt.rgba(1, 1, 1, 0.56)

    component Figure: LoginText {
        color: "#ffffff"
        font.family: root.core.fontClock
        font.pixelSize: root.clockSize
        font.weight: Font.Bold
        font.features: ({ "tnum": 1 })
        font.letterSpacing: -Math.round(root.clockSize * 0.03)
    }
    component Corner: LoginAction {
        d: root.d
        size: root.px(42)
        fontSize: root.type(16)
        fontFamily: root.core.fontMain
        symbolFont: root.core.symbolFont
        ink: root.ink
    }

    Rectangle { anchors.fill: parent; color: root.core.surface }
    Image {
        id: picture
        anchors.fill: parent
        source: root.core.picture
        fillMode: Image.PreserveAspectCrop
        sourceSize: Qt.size(root.width, root.height)
        asynchronous: true
        cache: false
        visible: false
    }
    // Out of focus and dimmed, a little richer than the picture so it never reads grey; signing in brings it back.
    MultiEffect {
        anchors.fill: parent
        source: picture
        scale: 1.08 - 0.08 * root.release
        blurEnabled: true
        blur: 1
        blurMax: Math.round(56 * (1 - root.release))
        saturation: 0.12 * (1 - root.release)
        brightness: -0.04 * (1 - root.release)
        opacity: Math.min(1, root.entrance * 2)
    }
    Rectangle { anchors.fill: parent; color: Qt.rgba(0, 0, 0, (0.36 + root.core.lift) * (1 - root.release)) }

    // ── The lens ──
    readonly property real clockSize: root.px(372)
    readonly property real clockCentreY: Math.round(root.height * (root.primary ? 0.4 : 0.47))
    Item {
        id: lensMask
        anchors.fill: parent
        layer.enabled: true
        visible: false
        Row {
            id: lensRow
            anchors.horizontalCenter: parent.horizontalCenter
            y: root.clockCentreY - Math.round(height / 2)
            Figure { text: root.core.hours }
            Figure { text: ":" }
            Figure { text: root.core.minutes }
        }
    }
    Item {
        anchors.fill: parent
        opacity: root.entrance * (1 - root.release)
        scale: 1 - 0.04 * (1 - root.entrance) + 0.06 * root.release
        // A soft fall of shadow lifts the lens off the blur.
        MultiEffect {
            anchors.fill: parent
            y: root.px(14)
            source: lensMask
            blurEnabled: true
            blur: 1
            blurMax: 64
            colorization: 1
            colorizationColor: "#000000"
            opacity: 0.55
        }
        // A pale base under the lens: where the picture is dark, the figures still read lighter than the blur.
        MultiEffect {
            anchors.fill: parent
            source: lensMask
            opacity: 0.3
        }
        MultiEffect {
            anchors.fill: parent
            source: picture
            opacity: 0.9
            maskEnabled: true
            maskSource: lensMask
            maskThresholdMin: 0.5
            maskSpreadAtMin: 1
            brightness: 0.16
            saturation: 0.2
        }
    }
    LoginText {
        anchors.horizontalCenter: parent.horizontalCenter
        y: root.clockCentreY + Math.round(lensRow.height * 0.36)
        opacity: root.entrance * (1 - root.release)
        text: root.core.meridiem.length > 0 ? root.core.date + "  ·  " + root.core.meridiem : root.core.date
        color: root.ink
        font.family: root.core.fontMain
        font.pixelSize: root.type(26)
        font.weight: Font.DemiBold
    }

    // ── Who, and the password ──
    Column {
        id: signIn
        visible: root.primary
        anchors.horizontalCenter: parent.horizontalCenter
        y: Math.round(root.height * 0.72)
        spacing: root.px(18)
        opacity: Math.max(0, Math.min(1, root.entrance * 1.3 - 0.3)) * (1 - root.release)
        transform: Translate { y: Math.round((1 - root.entrance) * 28 * root.d + root.release * 20 * root.d) }

        Row {
            id: who
            visible: root.core.showAvatar || root.core.showName
            anchors.horizontalCenter: parent.horizontalCenter
            spacing: root.px(12)
            LoginAvatar {
                visible: root.core.showAvatar
                anchors.verticalCenter: parent.verticalCenter
                width: root.px(38)
                height: width
                icon: root.core.userIcon
                name: root.core.userName
                fontFamily: root.core.fontMain
            }
            LoginText {
                visible: root.core.showName
                anchors.verticalCenter: parent.verticalCenter
                text: root.core.userName
                color: root.ink
                font.family: root.core.fontMain
                font.pixelSize: root.type(20)
                font.weight: Font.DemiBold
            }
            MSymbol {
                visible: root.core.manyUsers
                anchors.verticalCenter: parent.verticalCenter
                text: "unfold_more"
                symFont: root.core.symbolFont
                iconSize: root.px(20)
                iconColor: root.inkSoft
            }
        }

        Item {
            id: lozenge
            anchors.horizontalCenter: parent.horizontalCenter
            width: root.px(460)
            height: root.px(64)
            transform: Translate { x: root.shakeX }
            Rectangle {
                anchors.fill: parent
                radius: height / 2
                color: Qt.rgba(0, 0, 0, 0.3)
            }
            Rectangle {
                anchors.fill: parent
                radius: height / 2
                color: "transparent"
                border.width: Math.max(1, root.px(2))
                border.color: root.core.failed ? root.core.danger : root.core.accentInk
                opacity: entry.input.activeFocus || root.core.failed ? 1 : 0
                Behavior on opacity { NumberAnimation { duration: 160; easing.type: Easing.OutCubic } }
            }
            LoginEntry {
                id: entry
                anchors.fill: parent
                anchors.leftMargin: root.px(64)
                anchors.rightMargin: root.px(64)
                d: root.d
                marks: "shapes"
                centred: true
                busy: root.core.signingIn
                markColour: root.core.accent
                markSize: root.px(24)
                markGap: root.px(6)
                caretColour: root.core.accentInk
                caretHeight: root.px(26)
                placeholder: root.core.text.password
                placeholderColour: root.inkFaint
                fontFamily: root.core.fontMain
                fontSize: root.type(18)
                onSubmitted: root.core.signIn()
                onEdited: root.core.typed()
            }
            LoginSubmit {
                anchors.right: parent.right
                anchors.rightMargin: root.px(8)
                anchors.verticalCenter: parent.verticalCenter
                width: root.px(48)
                height: width
                d: root.d
                shown: entry.count > 0
                busy: root.core.signingIn
                fill: root.core.accent
                ink: root.core.onAccent
                symbolFont: root.core.symbolFont
                onActivated: root.core.signIn()
            }
        }
        LoginText {
            anchors.horizontalCenter: parent.horizontalCenter
            visible: root.core.showHint
            width: Math.min(implicitWidth, root.px(560))
            horizontalAlignment: Text.AlignHCenter
            wrapMode: Text.Wrap
            maximumLineCount: 2
            text: root.core.hint.length > 0 ? root.core.hint : " "
            color: root.core.hintIsWarning ? root.core.danger : root.core.highlight
            font.family: root.core.fontMain
            font.pixelSize: root.type(15)
            font.weight: Font.Medium
        }
    }
    // With more than one account, who you are is the switch.
    MouseArea {
        x: signIn.x + who.x
        y: signIn.y + who.y
        width: who.width
        height: who.height
        visible: root.primary
        enabled: root.core.manyUsers
        cursorShape: Qt.PointingHandCursor
        onClicked: root.core.nextUser()
    }

    // ── Corners: session and layout left, power right ──
    Row {
        visible: root.primary
        anchors.left: parent.left
        anchors.bottom: parent.bottom
        anchors.margins: root.px(40)
        opacity: root.entrance * (1 - root.release)
        spacing: root.px(4)
        Corner { visible: root.core.manySessions; glyph: "desktop_windows"; label: root.core.sessionName; onActivated: root.core.nextSession() }
        Corner { visible: root.core.manyLayouts; glyph: "keyboard"; label: root.core.layoutName; onActivated: root.core.nextLayout() }
    }
    Row {
        visible: root.primary
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        anchors.margins: root.px(40)
        opacity: root.entrance * (1 - root.release)
        spacing: root.px(4)
        Corner { visible: sddm.canSuspend; glyph: "bedtime"; onActivated: sddm.suspend() }
        Corner { visible: sddm.canReboot; glyph: "restart_alt"; onActivated: sddm.reboot() }
        Corner { visible: sddm.canPowerOff; glyph: "power_settings_new"; onActivated: sddm.powerOff() }
    }
}

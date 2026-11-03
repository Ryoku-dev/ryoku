// iRiS login, Cover ("Portada"): the picture as a cover, sharp and in full colour; one editorial column in the
// bottom-left corner. A huge light clock, then who you are on a baseline that is the password; the accent
// only marks what acts.
import QtQuick 2.15
import QtQuick.Shapes
import "."

LoginSurface {
    id: root
    field: entry.input

    readonly property real mx: root.px(112)
    readonly property real my: root.px(92)
    readonly property color ink: "#ffffff"
    readonly property color inkSoft: Qt.rgba(1, 1, 1, 0.74)
    readonly property color inkFaint: Qt.rgba(1, 1, 1, 0.52)

    // ── The cover ──
    Rectangle { anchors.fill: parent; color: root.core.surface }
    Image {
        anchors.fill: parent
        source: root.core.picture
        fillMode: Image.PreserveAspectCrop
        sourceSize: Qt.size(root.width, root.height)
        asynchronous: true
        cache: false
        opacity: Math.min(1, root.entrance * 2)
        // Exactly 1 at rest (1.035 - 0.035 is not): a picture a hair off 1:1 is resampled and goes soft.
        scale: 1 + 0.035 * (1 - root.entrance) + 0.02 * root.release
    }
    Rectangle { anchors.fill: parent; color: Qt.rgba(0, 0, 0, root.core.lift * (1 - root.release)) }
    // Ink where the type sits and nowhere else: deepest in the bottom-left corner, a breath under the masthead.
    Item {
        anchors.fill: parent
        opacity: 1 - root.release
        Rectangle {
            anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
            height: parent.height * 0.74
            gradient: Gradient {
                GradientStop { position: 0; color: "transparent" }
                GradientStop { position: 0.45; color: Qt.rgba(0, 0, 0, 0.1) }
                GradientStop { position: 1; color: Qt.rgba(0, 0, 0, 0.42) }
            }
        }
        Shape {
            anchors.fill: parent
            visible: root.primary
            ShapePath {
                strokeWidth: -1
                fillGradient: RadialGradient {
                    centerX: 0; centerY: root.height
                    centerRadius: root.height * 1.25
                    focalX: 0; focalY: root.height
                    GradientStop { position: 0; color: Qt.rgba(0, 0, 0, 0.72) }
                    GradientStop { position: 0.38; color: Qt.rgba(0, 0, 0, 0.56) }
                    GradientStop { position: 0.7; color: Qt.rgba(0, 0, 0, 0.2) }
                    GradientStop { position: 1; color: "transparent" }
                }
                startX: 0; startY: 0
                PathLine { x: root.width; y: 0 }
                PathLine { x: root.width; y: root.height }
                PathLine { x: 0; y: root.height }
                PathLine { x: 0; y: 0 }
            }
        }
        Rectangle {
            anchors { left: parent.left; right: parent.right; top: parent.top }
            height: parent.height * 0.22
            visible: root.primary
            gradient: Gradient {
                GradientStop { position: 0; color: Qt.rgba(0, 0, 0, 0.4) }
                GradientStop { position: 1; color: "transparent" }
            }
        }
    }

    // ── Masthead: the machine on the left, the session and power on the right ──
    component Figure: LoginText {
        color: root.ink
        font.family: root.core.fontClock
        font.pixelSize: root.clockSize
        font.weight: Font.Light
        font.features: ({ "tnum": 1 })
        font.letterSpacing: -Math.round(root.clockSize * 0.035)
    }
    component Folio: LoginAction {
        d: root.d
        size: root.px(38)
        fontSize: root.type(15)
        fontFamily: root.core.fontMain
        symbolFont: root.core.symbolFont
        ink: root.ink
    }
    LoginText {
        x: root.mx
        y: Math.round(root.my * 0.62)
        visible: root.primary && text.length > 0
        opacity: root.entrance * (1 - root.release)
        text: sddm.hostName
        color: root.ink
        font.family: root.core.fontMain
        font.pixelSize: root.type(17)
        font.weight: Font.DemiBold
    }
    Row {
        anchors.right: parent.right
        anchors.rightMargin: root.mx - root.px(10)
        y: Math.round(root.my * 0.62) - root.px(9)
        visible: root.primary
        opacity: root.entrance * (1 - root.release)
        spacing: root.px(4)
        Folio { visible: root.core.manySessions; glyph: "desktop_windows"; label: root.core.sessionName; onActivated: root.core.nextSession() }
        Folio { visible: root.core.manyLayouts; glyph: "keyboard"; label: root.core.layoutName; onActivated: root.core.nextLayout() }
        Folio { visible: sddm.canSuspend; glyph: "bedtime"; onActivated: sddm.suspend() }
        Folio { visible: sddm.canReboot; glyph: "restart_alt"; onActivated: sddm.reboot() }
        Folio { visible: sddm.canPowerOff; glyph: "power_settings_new"; onActivated: sddm.powerOff() }
    }

    // ── The column ──
    readonly property real clockSize: root.px(296)
    TextMetrics { id: clockInk; font: hoursText.font; text: root.core.hours + ":" + root.core.minutes }
    readonly property real columnWidth: Math.max(root.px(560), Math.round(clockInk.tightBoundingRect.width))

    Item {
        id: column
        x: root.mx
        width: root.columnWidth
        height: root.height
        opacity: root.entrance * (1 - root.release)
        transform: Translate { y: Math.round((1 - root.entrance) * 36 * root.d - root.release * 24 * root.d) }

        // Kicker: a short accent bar, then the day.
        Row {
            y: clockRow.y + hoursText.baselineOffset + Math.round(clockInk.tightBoundingRect.y) - root.px(44) - height
            spacing: root.px(14)
            Rectangle {
                anchors.verticalCenter: parent.verticalCenter
                width: root.px(28)
                height: root.px(4)
                radius: height / 2
                color: root.core.accentInk
            }
            LoginText {
                text: root.core.date
                color: root.ink
                font.family: root.core.fontMain
                font.pixelSize: root.type(22)
                font.weight: Font.DemiBold
            }
        }

        Row {
            id: clockRow
            // Optical flush-left: the figure's ink starts on the column, not its side bearing.
            x: -Math.round(clockInk.tightBoundingRect.x)
            y: (root.primary ? signIn.y : root.height - root.my) - root.px(52) - hoursText.baselineOffset
            Figure { id: hoursText; text: root.core.hours }
            Figure { text: ":" }
            Figure { text: root.core.minutes }
            LoginText {
                visible: root.core.meridiem.length > 0
                anchors.baseline: hoursText.baseline
                leftPadding: root.px(18)
                text: root.core.meridiem
                color: root.inkSoft
                font.family: root.core.fontMain
                font.pixelSize: root.type(30)
                font.weight: Font.DemiBold
            }
        }

        // Who, then the password on the same baseline.
        Item {
            id: signIn
            visible: root.primary
            width: parent.width
            height: root.px(60)
            y: root.height - root.my - root.px(34) - height
            transform: Translate { x: root.shakeX }

            LoginAvatar {
                id: avatar
                visible: root.core.showAvatar
                anchors.verticalCenter: parent.verticalCenter
                width: root.px(44)
                height: width
                icon: root.core.userIcon
                name: root.core.userName
                fontFamily: root.core.fontMain
            }
            Row {
                id: who
                visible: root.core.showName
                anchors.left: avatar.visible ? avatar.right : parent.left
                anchors.leftMargin: avatar.visible ? root.px(16) : 0
                anchors.verticalCenter: parent.verticalCenter
                spacing: root.px(4)
                LoginText {
                    text: root.core.userName
                    color: root.ink
                    font.family: root.core.fontMain
                    font.pixelSize: root.type(22)
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
            MouseArea {
                anchors.fill: who
                enabled: root.core.manyUsers
                cursorShape: Qt.PointingHandCursor
                onClicked: root.core.nextUser()
            }

            LoginEntry {
                id: entry
                anchors.left: who.visible ? who.right : avatar.visible ? avatar.right : parent.left
                anchors.leftMargin: who.visible || avatar.visible ? root.px(30) : 0
                anchors.right: submit.left
                anchors.rightMargin: root.px(12)
                anchors.top: parent.top
                anchors.bottom: parent.bottom
                d: root.d
                busy: root.core.signingIn
                markColour: root.ink
                markSize: root.px(11)
                markGap: root.px(10)
                caretColour: root.core.failed ? root.core.danger : root.core.accentInk
                caretHeight: root.px(28)
                placeholder: root.core.text.password
                placeholderColour: root.inkFaint
                fontFamily: root.core.fontMain
                fontSize: root.type(20)
                onSubmitted: root.core.signIn()
                onEdited: root.core.typed()
            }
            LoginSubmit {
                id: submit
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                width: root.px(44)
                height: width
                d: root.d
                shown: entry.count > 0
                busy: root.core.signingIn
                fill: root.core.accent
                ink: root.core.onAccent
                symbolFont: root.core.symbolFont
                onActivated: root.core.signIn()
            }

            // The baseline: quiet at rest, swept by the accent when it has your attention.
            Rectangle {
                id: rule
                anchors.bottom: parent.bottom
                width: parent.width
                height: Math.max(2, root.px(3))
                radius: height / 2
                color: Qt.rgba(1, 1, 1, 0.3)
            }
            Rectangle {
                anchors.bottom: parent.bottom
                height: rule.height
                radius: rule.height / 2
                color: root.core.failed ? root.core.danger : root.core.accentInk
                property real lit: entry.input.activeFocus || root.core.failed ? 1 : 0
                Behavior on lit { NumberAnimation { duration: 520; easing.type: Easing.BezierSpline; easing.bezierCurve: root.expressive } }
                width: rule.width * lit
                visible: !root.core.signingIn
            }
            // Working: a segment travels the line.
            Rectangle {
                id: runner
                anchors.bottom: parent.bottom
                height: rule.height
                radius: rule.height / 2
                width: rule.width * 0.22
                color: root.core.accentInk
                visible: root.core.signingIn && !root.core.succeeded
                NumberAnimation on x {
                    running: runner.visible
                    loops: Animation.Infinite
                    from: -runner.width; to: rule.width
                    duration: 1100; easing.type: Easing.InOutCubic
                }
            }
        }
        LoginText {
            anchors.top: signIn.bottom
            anchors.topMargin: root.px(14)
            width: signIn.width
            visible: root.primary && root.core.showHint
            wrapMode: Text.Wrap
            maximumLineCount: 2
            text: root.core.hint
            color: root.core.hintIsWarning ? root.core.danger : root.core.highlight
            font.family: root.core.fontMain
            font.pixelSize: root.type(15)
            font.weight: Font.Medium
        }
    }
}

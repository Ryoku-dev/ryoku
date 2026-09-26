import QtQuick
import QtQuick.Window
import Qt5Compat.GraphicalEffects
import Qt.labs.folderlistmodel
import SddmComponents 2.0
import "i18n"

Rectangle {
    // Wayland Cursor Fix
    MouseArea {
        anchors.fill: parent
        cursorShape: Qt.ArrowCursor
        z: -1
    }
    id: root
    readonly property real s: Screen.height / 768
    width: Screen.width
    height: Screen.height
    color: root.bgColor

    property bool isQuickshell: typeof sddm === "undefined" || sddm.hostName === undefined

    // --- Software cursor for the SDDM login screen --------------------------
    // weston (the greeter's compositor) implements no wp_cursor_shape, and its
    // wl_pointer.set_cursor fallback never renders on this hybrid GPU, so the
    // login pointer is invisible (issue #191). Draw one in the scene instead: a
    // passive HoverHandler tracks the pointer without stealing clicks or hover,
    // and an arrow follows it. Only under the SDDM greeter -- the in-session lock
    // runs on the shell's own compositor, which draws a real cursor, so off there.
    HoverHandler {
        id: swPointer
        enabled: !root.isQuickshell
    }
    Canvas {
        id: swCursor
        width: 26 * root.s
        height: 26 * root.s
        z: 2000000
        visible: !root.isQuickshell && swPointer.hovered
        x: swPointer.point.position.x
        y: swPointer.point.position.y
        antialiasing: true
        onPaint: {
            var ctx = getContext("2d");
            ctx.reset();
            ctx.scale(root.s, root.s);
            ctx.beginPath();
            ctx.moveTo(0, 0);
            ctx.lineTo(0, 16);
            ctx.lineTo(4, 12.5);
            ctx.lineTo(6.5, 19);
            ctx.lineTo(9, 18);
            ctx.lineTo(6.5, 11.5);
            ctx.lineTo(11, 11.5);
            ctx.closePath();
            ctx.fillStyle = "#ffffff";
            ctx.fill();
            ctx.lineWidth = 1.2;
            ctx.strokeStyle = "#000000";
            ctx.stroke();
        }
    }

    // The 60fps clock timer below only feeds the smooth second-hand, so keep it
    // awake only while this surface is on screen. Under the SDDM greeter that is
    // when the window is focused; a backgrounded or orphaned greeter then stops
    // waking the CPU ~60x/s for a clock nobody can see. The in-session lock shim
    // (which alone defines sddm.fingerprintHint) lives only while shown, so it
    // always animates.
    readonly property bool clockAwake: ((typeof sddm !== "undefined") && sddm.fingerprintHint === true) || Window.active

    // ── fingerprint sensor hint ─────────────────────────────────────────────
    // Driven by the lock shim's carry-over props (fingerprintHint,
    // fingerprintReady, fingerprintState). These are undefined under a real
    // SDDM greeter, so every gate evaluates false and the hint is invisible
    // there. Under the lock screen shim, they drive the sensor hint UI.
    readonly property bool fpVisible: (typeof sddm !== "undefined") && sddm.fingerprintHint === true && sddm.fingerprintReady === true
    readonly property var fpState: typeof sddm !== "undefined" ? (sddm.fingerprintState || "idle") : "idle"
    readonly property string fpHintText: {
        if (root.fpState === "success") return I18n.tr("ACCESS GRANTED ✦")
        if (root.fpState === "fail")    return I18n.tr("SCAN DENIED ✦ TYPE YOUR KEY")
        if (root.fpState === "scanning") return I18n.tr("SENSOR ACTIVE ✦ TOUCH OR TYPE")
        return I18n.tr("TOUCH SENSOR ✦ TO UNLOCK")
    }

    // Theme Config
    readonly property string themeMode: config.themeMode || "dark"
    readonly property bool enableWindup: config.enableWindup !== "false" && config.enableWindup !== false
    readonly property bool isLight: themeMode === "light"

    readonly property color bgColor: isLight ? "#ffffff" : "#000000"
    readonly property color mainText: isLight ? "#000000" : "#ffffff"
    readonly property color dimText: isLight ? "#666666" : "#666666"
    readonly property color subColor: isLight ? "#666666" : "#555555"
    readonly property color pillColor: isLight ? "#e8e8e8" : "#080808"
    readonly property color pillBorder: isLight ? (root.isWindup ? "#aaaaaa" : "#cccccc") : (root.isWindup ? "#444" : "#1a1a1a")
    readonly property color pillInnerLine: isLight ? (root.isWindup ? "#000000" : "#bbbbbb") : (root.isWindup ? "#ffffff" : "#222222")
    readonly property color sparkColor: isLight ? "#000000" : "#ffffff"
    readonly property color blastColor: isLight ? "#000000" : "#ffffff"
    readonly property color userItemInactive: isLight ? "#cccccc" : "#444"
    readonly property color inputWaitColor: isLight ? "#bbbbbb" : "#333333"

    // UI State
    property int sessionIndex: (typeof sessionModel !== "undefined" && sessionModel.lastIndex >= 0) ? sessionModel.lastIndex : 0
    property int userIndex: (typeof userModel !== "undefined" && userModel.lastIndex >= 0) ? userModel.lastIndex : 0
    property bool userMenuOpen: false
    property bool isWindup: false
    property real uiOpacity: 0
    property string authInfo: ""
    readonly property bool fidoInfo: authInfo.toLowerCase().indexOf("fido") >= 0 || authInfo.toLowerCase().indexOf("authenticator") >= 0 || authInfo.toLowerCase().indexOf("pin") >= 0
    readonly property bool touchInfo: authInfo.toLowerCase().indexOf("touch") >= 0
    readonly property string inputHint: authInfo !== "" ? authInfo.toUpperCase() : I18n.tr("TYPE PASSWORD OR FIDO PIN")
    readonly property real marginR: 80 * s

    // Time Logic
    property int curH: new Date().getHours()
    property int curM: new Date().getMinutes()
    property int curS: new Date().getSeconds()
    property int curMS: new Date().getMilliseconds()
    readonly property real localTimeMS: (curH * 3600000) + (curM * 60000) + (curS * 1000) + curMS

    Timer {
        interval: 16; running: root.clockAwake; repeat: true
        onTriggered: {
            var d = new Date()
            root.curH = d.getHours(); root.curM = d.getMinutes(); root.curS = d.getSeconds(); root.curMS = d.getMilliseconds()
        }
    }

    // Animation Props
    property real windupOffset: 0
    property real windupProgress: windupOffset / 150000
    property real boomScale: 1.0
    property real boomOpacity: 0.0
    property real jitterX: 0
    property real jitterY: 0
    property real sparkIntensity: 0.0

    Timer {
        interval: 16; running: root.isWindup; repeat: true
        onTriggered: {
            var intensity = root.windupProgress * 32 * s
            root.jitterX = (Math.random() - 0.5) * intensity
            root.jitterY = (Math.random() - 0.5) * intensity
            root.sparkIntensity = root.windupProgress > 0.2 ? (root.windupProgress - 0.2) * 2.2 : 0
        }
    }

    NumberAnimation { id: windupAnim; target: root; property: "windupOffset"; from: 0; to: 150000; duration: 1600; easing.type: Easing.InQuint }

    ParallelAnimation {
        id: boomSequence
        onFinished: {
            root.doLogin()
            // A sensor win committed auth before the wind-up ran; the reveal
            // that the password path plays on loginSucceeded belongs here.
            if (root._sensorWindup) {
                root._sensorWindup = false
                root.playUnlockReveal()
            }
        }
        NumberAnimation { target: root; property: "boomScale"; to: 35.0; duration: 150; easing.type: Easing.InQuad }
        NumberAnimation { target: root; property: "boomOpacity"; to: 1.0; duration: 120; easing.type: Easing.InQuad }
    }

    // Fingerprint unlock flourish: the windup's reveal without the windup, run
    // only on a genuine sensor win (never triggers a second authentication).
    ParallelAnimation {
        id: boomReveal
        NumberAnimation { target: root; property: "boomScale"; to: 35.0; duration: 190; easing.type: Easing.InQuad }
        NumberAnimation { target: root; property: "boomOpacity"; to: 1.0; duration: 130; easing.type: Easing.InQuad }
    }

    readonly property real smoothSecAngle: -((localTimeMS % 60000) / 60000.0) * 360.0 - windupOffset * 10.0
    readonly property real smoothMinAngle: -((localTimeMS % 3600000) / 3600000.0) * 360.0 - windupOffset * 5.0

    // Font Loading
    FolderListModel { id: fontFolder; folder: Qt.resolvedUrl("font"); nameFilters: ["*.ttf", "*.otf"] }
    FontLoader { id: outfitFont; source: fontFolder.count > 0 ? "font/" + fontFolder.get(0, "fileName") : "" }
    TextConstants { id: textConstants }

    // Data Models
    ListView {
        id: userHelper; width: 1; height: 1; opacity: 0; currentIndex: root.userIndex
        model: typeof userModel !== "undefined" ? userModel : null
        delegate: Item { property string uName: model.realName || model.name || ""; property string uLogin: model.name || "" }
    }
    ListView {
        id: sessionHelper; width: 1; height: 1; opacity: 0; currentIndex: root.sessionIndex
        model: typeof sessionModel !== "undefined" ? sessionModel : null
        delegate: Item { property string sName: model.name || "" }
    }

    // Input Focus
    Timer { interval: 300; running: true; onTriggered: passInput.forceActiveFocus() }

    Component.onCompleted: {
        keyboard.numLock = true
        if (typeof sessionModel !== "undefined")
            root.sessionIndex = preferredSessionIndex()
        // in-session lock paints only once secure; the greeter is up at load
        if (root.enableWindup) {
            if (!(typeof sddm !== "undefined" && sddm.fingerprintHint === true))
                playLockReveal()
        } else {
            fadeIn.start()
        }
    }
    NumberAnimation { id: fadeIn; target: root; property: "uiOpacity"; to: 1; duration: 350; easing.type: Easing.OutCubic }

    // Main Layout
    Item {
        id: blastContainer
        anchors.fill: parent; opacity: root.uiOpacity
        x: root.jitterX; y: root.jitterY
        transform: Scale { origin.x: 400 * s; origin.y: blastContainer.height * 0.5; xScale: root.boomScale; yScale: root.boomScale }

        Item {
            id: clockContainer
            anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
            width: 800 * s; height: parent.height
            readonly property real cx: 40 * s 
            readonly property real cy: height * 0.5
            readonly property real minR: 320 * s 
            readonly property real secR: 480 * s 

            Rectangle {
                id: indicatorPill; z: 1; x: clockContainer.cx + 230 * s; anchors.verticalCenter: parent.verticalCenter
                width: 330 * s; height: 90 * s; radius: 45 * s; color: root.pillColor; border.color: root.pillBorder; border.width: 1 * s
                Rectangle { x: 170 * s; anchors.verticalCenter: parent.verticalCenter; width: 1 * s; height: 35 * s; color: root.pillInnerLine }
            }

            Repeater {
                model: 60
                delegate: Rectangle {
                    z: 50; property real randA: Math.random() * 6.28; property real randV: 400 * s + Math.random() * 900 * s
                    x: (clockContainer.cx + 400 * s) + Math.cos(randA) * (randV * root.sparkIntensity)
                    y: (clockContainer.cy) + Math.sin(randA) * (randV * root.sparkIntensity)
                    width: (1 + Math.random() * 2) * s; height: (1 + 12 * root.sparkIntensity) * s 
                    rotation: randA * 180 / Math.PI + 90; radius: width / 2; color: root.sparkColor
                    opacity: root.sparkIntensity * (Math.random() > 0.4 ? 1.0 : 0.2); visible: root.sparkIntensity > 0
                }
            }

            Repeater {
                model: 60
                delegate: Item {
                    z: 10; property real base: index * 6
                    property real relAngle: { var a = (base + root.smoothMinAngle) % 360; if (a > 180) a -= 360; if (a < -180) a += 360; return a }
                    property real spotlight: Math.max(0, 1.0 - Math.abs(relAngle) / 4.0)
                    property bool isMajor: index % 5 == 0
                    property real disp: (base + root.smoothMinAngle) * Math.PI / 180
                    property real tx: clockContainer.cx + clockContainer.minR * Math.cos(disp)
                    property real ty: clockContainer.cy + clockContainer.minR * Math.sin(disp)
                    visible: tx > -600 * s && tx < 1800 * s
                    Rectangle {
                        x: parent.tx - width/2; y: parent.ty - height/2; width: isMajor ? 2 * s : 1 * s; height: isMajor ? 18 * s : 10 * s
                        color: isLight ? Qt.rgba(0, 0, 0, spotlight > 0 ? 1.0 : (isMajor ? 0.8 : 0.6)) : Qt.rgba(1, 1, 1, spotlight > 0 ? 1.0 : (isMajor ? 0.3 : 0.15))
                        rotation: disp * 180 / Math.PI + 90
                    }
                    Text {
                        visible: isMajor; property real nRad: clockContainer.minR - 35 * s
                        x: clockContainer.cx + nRad * Math.cos(disp) - width/2
                        y: clockContainer.cy + nRad * Math.sin(disp) - height/2
                        text: String(index).padStart(2, '0'); font.family: outfitFont.name; font.pixelSize: 22 * s; font.weight: spotlight > 0.5 ? Font.Bold : Font.Normal
                        color: isLight ? Qt.rgba(0, 0, 0, spotlight > 0 ? (0.6 + 0.4 * spotlight) : 0.6) : Qt.rgba(1, 1, 1, spotlight > 0 ? (0.4 + spotlight * 0.6) : 0.25)
                        rotation: disp * 180 / Math.PI; transformOrigin: Item.Center
                    }
                }
            }

            Repeater {
                model: 60
                delegate: Item {
                    z: 10; property real base: index * 6
                    property real relAngle: { var a = (base + root.smoothSecAngle) % 360; if (a > 180) a -= 360; if (a < -180) a += 360; return a }
                    property real spotlight: Math.max(0, 1.0 - Math.abs(relAngle) / 4.0)
                    property bool isMajor: index % 5 == 0
                    property real disp: (base + root.smoothSecAngle) * Math.PI / 180
                    property real tx: clockContainer.cx + clockContainer.secR * Math.cos(disp)
                    property real ty: clockContainer.cy + clockContainer.secR * Math.sin(disp)
                    visible: tx > -600 * s && tx < 1800 * s
                    Rectangle {
                        x: parent.tx - width/2; y: parent.ty - height/2; width: isMajor ? 1.5 * s : 1 * s; height: isMajor ? 13 * s : 8 * s
                        color: isLight ? Qt.rgba(0, 0, 0, spotlight > 0 ? 1.0 : (isMajor ? 0.8 : 0.6)) : Qt.rgba(1, 1, 1, spotlight > 0 ? 1.0 : (isMajor ? 0.3 : 0.15))
                        rotation: disp * 180 / Math.PI + 90
                    }
                    Text {
                        visible: isMajor; property real nRad: clockContainer.secR - 30 * s
                        x: clockContainer.cx + nRad * Math.cos(disp) - width/2
                        y: clockContainer.cy + nRad * Math.sin(disp) - height/2
                        text: String(index).padStart(2, '0'); font.family: outfitFont.name; font.pixelSize: 16 * s; font.weight: spotlight > 0.5 ? Font.Bold : Font.Normal
                        color: isLight ? Qt.rgba(0, 0, 0, spotlight > 0 ? (0.6 + 0.4 * spotlight) : 0.6) : Qt.rgba(1, 1, 1, spotlight > 0 ? (0.4 + spotlight * 0.6) : 0.25)
                        rotation: disp * 180 / Math.PI; transformOrigin: Item.Center
                    }
                }
            }

            Text {
                anchors.right: indicatorPill.left; anchors.rightMargin: 40 * s; anchors.verticalCenter: parent.verticalCenter
                text: String(root.curH).padStart(2, '0'); font.family: outfitFont.name; font.pixelSize: 110 * s; font.weight: Font.Black; color: root.mainText
            }
            Column {
                anchors.left: indicatorPill.right; anchors.leftMargin: 110 * s; anchors.verticalCenter: parent.verticalCenter; spacing: 5 * s
                Text { text: Qt.formatDate(new Date(), "dd MMM yyyy").toUpperCase(); font.family: outfitFont.name; font.pixelSize: 13 * s; font.letterSpacing: 4 * s; color: root.subColor }
                Text { text: Qt.formatDate(new Date(), "dddd").toUpperCase(); font.family: outfitFont.name; font.pixelSize: 18 * s; font.letterSpacing: 8 * s; font.weight: Font.Bold; color: root.mainText }
            }
        }
    }

    // Flash Effect
    Rectangle { anchors.fill: parent; color: root.blastColor; opacity: root.boomOpacity; z: 9999 }

    // HUD Layer
    Item {
        id: hudContainer; anchors.fill: parent; opacity: root.uiOpacity * (root.boomOpacity > 0 ? 0 : 1)
        Row {
            anchors.right: parent.right; anchors.rightMargin: root.marginR; anchors.top: parent.top; anchors.topMargin: 50 * s; spacing: 25 * s
            CwAction { visible: !root.isQuickshell; label: (sessionHelper.currentItem ? sessionHelper.currentItem.sName : I18n.tr("Session")); onClicked: { if (typeof sessionModel !== "undefined") root.sessionIndex = (root.sessionIndex + 1) % sessionModel.rowCount() } }
            Rectangle { visible: !root.isQuickshell; width: 1 * s; height: 10 * s; color: root.pillBorder; anchors.verticalCenter: parent.verticalCenter }
            CwAction { label: I18n.tr("Reboot"); onClicked: { if (typeof sddm !== "undefined") sddm.reboot() } }
            Rectangle { width: 1 * s; height: 10 * s; color: root.pillBorder; anchors.verticalCenter: parent.verticalCenter }
            CwAction { label: I18n.tr("Shutdown"); onClicked: { if (typeof sddm !== "undefined") sddm.powerOff() } }
        }
        Column {
            id: loginPanel; anchors.right: parent.right; anchors.rightMargin: root.marginR; anchors.bottom: parent.bottom; anchors.bottomMargin: 80 * s; width: 350 * s; spacing: 8 * s
            Item {
                width: parent.width; height: 32 * s; z: 5000
                Item {
                    id: uMenuContainer; anchors.bottom: userNameDisp.top; anchors.bottomMargin: 15 * s; anchors.right: parent.right; width: 280 * s
                    height: root.userMenuOpen ? ((typeof userModel !== "undefined" ? userModel.rowCount() : 0) * 30 * s) + 20 * s : 0; clip: true
                    Behavior on height { NumberAnimation { duration: 400; easing.type: Easing.OutExpo } }
                    Column {
                        anchors.bottom: parent.bottom; anchors.right: parent.right; spacing: 6 * s
                        Repeater {
                            model: typeof userModel !== "undefined" ? userModel : null
                            delegate: Item {
                                width: 260 * s; height: 26 * s; property bool itemHover: uItemMa.containsMouse
                                Text {
                                    id: uItemTxt; text: (model.realName || model.name || "").toUpperCase(); font.family: outfitFont.name; font.pixelSize: 13 * s; font.letterSpacing: 2 * s; color: (root.userIndex === index || itemHover) ? root.mainText : root.userItemInactive; anchors.right: parent.right; anchors.rightMargin: itemHover ? 30 * s : 10 * s; anchors.verticalCenter: parent.verticalCenter; Behavior on anchors.rightMargin { NumberAnimation { duration: 200 } }
                                }
                                Text { text: "✦"; anchors.left: uItemTxt.right; anchors.leftMargin: 8 * s; anchors.verticalCenter: parent.verticalCenter; color: root.mainText; opacity: itemHover ? 1.0 : 0; font.pixelSize: 10 * s; Behavior on opacity { NumberAnimation { duration: 200 } } }
                                MouseArea { id: uItemMa; anchors.fill: parent; hoverEnabled: true; onClicked: { root.userIndex = index; root.userMenuOpen = false } }
                            }
                        }
                    }
                }
                Text {
                    id: userNameDisp; anchors.right: parent.right; anchors.rightMargin: (uMa.containsMouse || root.userMenuOpen) ? 25 * s : 0
                    text: ((userHelper.currentItem && userHelper.currentItem.uName) ? userHelper.currentItem.uName : ((typeof userModel !== "undefined" && userModel.lastUser) ? capitalizeFirst(userModel.lastUser) : I18n.tr("USER"))).toUpperCase()
                    font.family: outfitFont.name; font.pixelSize: 18 * s; font.weight: Font.Bold; font.letterSpacing: 8 * s; color: (uMa.containsMouse || root.userMenuOpen) ? root.mainText : root.dimText; Behavior on color { ColorAnimation { duration: 200 } } Behavior on anchors.rightMargin { NumberAnimation { duration: 250; easing.type: Easing.OutCubic } }
                }
                Text { text: "✦"; anchors.left: userNameDisp.right; anchors.leftMargin: 8 * s; anchors.verticalCenter: userNameDisp.verticalCenter; color: root.mainText; opacity: (uMa.containsMouse || root.userMenuOpen) ? 1.0 : 0; font.pixelSize: 12 * s; Behavior on opacity { NumberAnimation { duration: 200 } } }
                MouseArea { id: uMa; anchors.fill: parent; hoverEnabled: true; cursorShape: Qt.PointingHandCursor; onClicked: { root.userMenuOpen = !root.userMenuOpen } }
            }
            // ── fingerprint sensor hint ─────────────────────────────────────
            // Sits between the user name and the password field so the eye
            // naturally hits it before typing. Uses mainText for full brightness
            // while scanning (not dim subColor). A pulse animation makes it
            // clear the sensor is active.
            Text {
                id: fpHint
                width: parent.width
                height: root.fpVisible ? 16 * s : 0
                visible: root.fpVisible
                horizontalAlignment: Text.AlignRight
                verticalAlignment: Text.AlignVCenter
                text: root.fpHintText
                color: root.fpState === "success" ? root.mainText
                    : (root.fpState === "fail" ? "#ff4444"
                    : (root.fpState === "scanning" ? root.mainText : root.dimText))
                font.family: outfitFont.name
                font.pixelSize: 9 * s
                font.letterSpacing: 2 * s
                opacity: root.fpState === "scanning" ? fpPulse.value : 1.0
                Behavior on color { ColorAnimation { duration: 200 } }
                Behavior on height { NumberAnimation { duration: 200; easing.type: Easing.OutCubic } }
            }
            // Gentle opacity pulse while the sensor listens.
            SequentialAnimation {
                id: fpPulse
                loops: Animation.Infinite
                running: root.fpVisible && root.fpState === "scanning"
                NumberAnimation { target: fpPulseVal; property: "value"; from: 1.0; to: 0.45; duration: 600; easing.type: Easing.InOutSine }
                NumberAnimation { target: fpPulseVal; property: "value"; from: 0.45; to: 1.0; duration: 600; easing.type: Easing.InOutSine }
            }
            Item { id: fpPulseVal; property real value: 1.0 }
            Item {
                width: parent.width; height: 30 * s
                TextInput {
                    id: passInput; anchors.fill: parent; echoMode: TextInput.Password; passwordCharacter: "✦"; color: root.dimText; font.family: outfitFont.name; font.pixelSize: 14 * s; font.letterSpacing: 10 * s; horizontalAlignment: TextInput.AlignRight; verticalAlignment: TextInput.AlignVCenter; focus: true; property bool wasClicked: false; cursorVisible: false; cursorDelegate: Item { width: 0; height: 0 }
                    Keys.onReturnPressed: startLoginSequence()
                    Text { anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter; text: root.inputHint; font.family: outfitFont.name; font.pixelSize: 10 * s; font.letterSpacing: 4 * s; color: root.fidoInfo ? root.mainText : root.inputWaitColor; opacity: passInput.text.length === 0 ? 0.55 : 0; Behavior on opacity { NumberAnimation { duration: 400; easing.type: Easing.InOutSine } } }
                    Rectangle {
                        id: needleCursor; width: 1.5 * s; height: 12 * s; color: root.mainText; anchors.verticalCenter: parent.verticalCenter; x: passInput.cursorRectangle.x; visible: passInput.focus && (passInput.text.length > 0 || passInput.wasClicked)
                        SequentialAnimation { loops: Animation.Infinite; running: needleCursor.visible; NumberAnimation { target: needleCursor; property: "opacity"; from: 1; to: 0.1; duration: 450 } NumberAnimation { target: needleCursor; property: "opacity"; from: 0.1; to: 1; duration: 450 } }
                    }
                }
                MouseArea { id: pMa_FixedFinal_Simple; anchors.fill: parent; cursorShape: Qt.ArrowCursor; onClicked: { passInput.forceActiveFocus(); passInput.wasClicked = true } }
            }
            Item {
                width: parent.width; height: 40 * s
                Text {
                    id: loginBtn; anchors.right: parent.right; anchors.rightMargin: btnMa.containsMouse ? 25 * s : 0; text: passInput.text.length > 0 ? I18n.tr("UNLOCK") : (root.touchInfo ? I18n.tr("TOUCH KEY") : I18n.tr("TRY TOUCH ONLY")); font.family: outfitFont.name; font.pixelSize: 11 * s; font.letterSpacing: 4 * s; font.weight: Font.Bold; color: btnMa.containsMouse ? root.mainText : root.dimText; opacity: 1.0; Behavior on anchors.rightMargin { NumberAnimation { duration: 250; easing.type: Easing.OutCubic } }
                }
                Text { text: "✦"; anchors.left: loginBtn.right; anchors.leftMargin: 8 * s; anchors.verticalCenter: loginBtn.verticalCenter; color: root.mainText; opacity: btnMa.containsMouse ? 1.0 : 0; font.pixelSize: 10 * s; Behavior on opacity { NumberAnimation { duration: 200 } } }
                MouseArea { id: btnMa; anchors.fill: parent; hoverEnabled: true; cursorShape: Qt.PointingHandCursor; onClicked: { startLoginSequence() } }
            }
            Text { id: errText; width: parent.width; height: 15 * s; verticalAlignment: Text.AlignBottom; horizontalAlignment: Text.AlignRight; text: ""; color: "#ff4444"; font.family: outfitFont.name; font.pixelSize: 10 * s; font.letterSpacing: 2 * s }
        }
    }

    Timer { id: boomTriggerTimer; interval: 1450; onTriggered: { boomSequence.start() } }
    function startLoginSequence() {
        if (isWindup) return
        // PAM never answers an empty key; the reveal blast would strand white.
        if (passInput.text.length === 0) {
            errText.text = ""
            passInput.forceActiveFocus()
            return
        }
        if (root.enableWindup) {
            isWindup = true
            windupAnim.start()
            boomTriggerTimer.start()
        } else {
            doLogin()
        }
    }
    // ── race condition guard ────────────────────────────────────────────────
    // _unlocked prevents boomSequence.onFinished: doLogin() from firing a
    // second PAM auth if the fingerprint wins mid-windup (after
    // boomTriggerTimer at 1450ms). boomSequence.stop() kills the animation.
    property bool _unlocked: false
    property bool _sensorWindup: false
    function doLogin() { if (_unlocked) return; _unlocked = true; root.authInfo = ""; var uname = (userHelper.currentItem && userHelper.currentItem.uLogin) ? userHelper.currentItem.uLogin : (typeof userModel !== "undefined" ? userModel.lastUser : "user"); if (typeof sddm !== "undefined") sddm.login(uname, passInput.text, root.sessionIndex); authWatchdog.restart() }
    // The flash stays up until auth answers; the watchdog lowers it if it never does.
    function clearUnlockFlash() {
        _unlocked = false; _sensorWindup = false; isWindup = false
        windupAnim.stop(); boomTriggerTimer.stop(); boomSequence.stop()
        root.windupOffset = 0; root.boomScale = 1.0; root.boomOpacity = 0.0; root.sparkIntensity = 0
    }
    Timer {
        id: authWatchdog
        interval: 8000
        onTriggered: { clearUnlockFlash(); root.authInfo = ""; errText.text = ""; passInput.text = ""; passInput.forceActiveFocus() }
    }
    function capitalizeFirst(str) { if (!str) return ""; return str.charAt(0).toUpperCase() + str.slice(1) }
    // ── login event handlers ────────────────────────────────────────────────
    Connections {
        target: typeof sddm !== "undefined" ? sddm : null
        ignoreUnknownSignals: true
        function onSurfaceRevealed() { playLockReveal() }
        function onInformationMessage(message) { root.authInfo = message || ""; errText.text = root.authInfo.toUpperCase(); passInput.forceActiveFocus() }
        function onErrorMessage(message) { errText.text = (message || "").toUpperCase() }
        function onLoginSucceeded() {
            authWatchdog.stop()
            if (typeof sddm !== "undefined" && sddm.fingerprintUnlock === true) {
                // Sensor win: auth is already committed. Run the same wind-up
                // the password path plays; _unlocked keeps boomSequence's
                // doLogin() from re-authenticating.
                _unlocked = true
                if (root.enableWindup) {
                    _sensorWindup = true
                    isWindup = true
                    windupAnim.start()
                    boomTriggerTimer.start()
                } else {
                    boomReveal.start()
                }
            } else if (root.enableWindup) {
                // Password win: the wind-up already ran; lower the curtain.
                playUnlockReveal()
            }
        }
        function onLoginFailed() { authWatchdog.stop(); clearUnlockFlash(); root.authInfo = ""; errText.text = I18n.tr("ACCESS DENIED"); passInput.text = ""; passInput.forceActiveFocus(); shake.start() }
    }
    SequentialAnimation {
        id: shake
        NumberAnimation { target: loginPanel; property: "anchors.rightMargin"; from: root.marginR; to: root.marginR + 10 * s; duration: 50; easing.type: Easing.InOutSine }
        NumberAnimation { target: loginPanel; property: "anchors.rightMargin"; to: root.marginR - 10 * s; duration: 50; easing.type: Easing.InOutSine }
        NumberAnimation { target: loginPanel; property: "anchors.rightMargin"; to: root.marginR; duration: 50; easing.type: Easing.InOutSine }
    }
    component CwAction: Item {
        id: actItem; width: actTxt.width + 20 * s; height: 15 * s; property string label: ""; signal clicked()
        Text { id: actTxt; anchors.right: parent.right; anchors.rightMargin: actM.containsMouse ? 15 * s : 0; text: label.toUpperCase(); color: actM.containsMouse ? root.mainText : root.dimText; font.family: outfitFont.name; font.pixelSize: 10 * s; font.letterSpacing: 3 * s; Behavior on color { ColorAnimation { duration: 200 } } Behavior on anchors.rightMargin { NumberAnimation { duration: 200 } } }
        Text { text: "✦"; anchors.left: actTxt.right; anchors.leftMargin: 4 * s; anchors.verticalCenter: actTxt.verticalCenter; color: root.mainText; opacity: actM.containsMouse ? 1.0 : 0; font.pixelSize: 8 * s; Behavior on opacity { NumberAnimation { duration: 200 } } }
        MouseArea { id: actM; anchors.fill: parent; hoverEnabled: true; onClicked: { actItem.clicked() } cursorShape: Qt.PointingHandCursor }
    }
    // swww-style circular reveal: an inverted OpacityMask punches a growing
    // hole in a surface-colour curtain; the layers drop when idle.
    readonly property real revealDiag: Math.sqrt(width * width + height * height)
    property real revealR: 0
    property bool revealActive: false

    Item {
        id: revealCurtain
        anchors.fill: parent
        z: 20000
        visible: root.revealActive
        layer.enabled: root.revealActive
        layer.effect: OpacityMask { invert: true; maskSource: revealMaskSrc }
        Rectangle { anchors.fill: parent; color: root.bgColor }
    }
    Item {
        id: revealMaskSrc
        anchors.fill: parent
        visible: false
        layer.enabled: root.revealActive
        Rectangle {
            anchors.centerIn: parent
            width: root.revealR * 2
            height: root.revealR * 2
            radius: root.revealR
            color: "white"
        }
    }

    NumberAnimation {
        id: revealIn
        target: root; property: "revealR"
        from: 0; to: root.revealDiag; duration: 1150; easing.type: Easing.InOutCubic
        onFinished: root.revealActive = false
    }
    NumberAnimation {
        id: revealOut
        target: root; property: "revealR"
        from: root.revealDiag; to: 0; duration: 560; easing.type: Easing.InOutCubic
    }

    function playLockReveal() {
        revealOut.stop()
        root.uiOpacity = 1
        root.revealR = 0
        root.revealActive = true
        revealIn.restart()
    }
    function playUnlockReveal() {
        revealIn.stop()
        root.revealR = root.revealDiag
        root.revealActive = true
        revealOut.restart()
    }

    // Prefer the session matching the running compositor, resolved from the
    // environment by the shim (sessionModel.desktopName), never a name literal.
    Item {
        visible: false
        Repeater {
            id: sessionScan
            model: (typeof sessionModel !== "undefined") ? sessionModel : null
            delegate: Item { property string sName: model.name || "" }
        }
    }
    function preferredSessionIndex() {
        if (typeof sessionModel === "undefined")
            return 0
        var want = (typeof sessionModel.desktopName === "string") ? sessionModel.desktopName : ""
        var exact = -1, loose = -1
        for (var i = 0; i < sessionScan.count; i++) {
            var it = sessionScan.itemAt(i)
            if (!it)
                continue
            var low = (it.sName || "").toLowerCase()
            // uwsm-managed is a duplicate entry Ryoku's logout path does not drive.
            if (low.indexOf("uwsm") >= 0)
                continue
            if (want !== "" && low === want)
                exact = i
            else if (want !== "" && low.indexOf(want) >= 0 && loose < 0)
                loose = i
        }
        if (exact >= 0)
            return exact
        if (loose >= 0)
            return loose
        return (sessionModel.lastIndex >= 0) ? sessionModel.lastIndex : 0
    }
}

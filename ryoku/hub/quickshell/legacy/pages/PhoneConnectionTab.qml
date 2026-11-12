pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../Singletons"

Item {
    id: pg

    property bool busy: false
    property string busyLabel: ""
    property string error: ""
    property var status: ({ installed: false, configured: false, supported: false, reason: "", devices: [] })
    property bool statusReady: false
    property string actionDevice: ""
    property bool entered: false
    property bool sectionsReady: false
    readonly property bool enabledOnDisk: value("phone.enabled", false) === true
    readonly property bool phoneActive: enabledOnDisk && status.installed
    readonly property bool featureReady: phoneActive
    readonly property var pairedDevices: {
        var out = [];
        var devices = status.devices || [];
        for (var i = 0; i < devices.length; i++) {
            if (devices[i].paired === true)
                out.push(devices[i]);
        }
        return out;
    }
    readonly property var primaryDevice: status.primaryDevice || (pairedDevices.length > 0 ? pairedDevices[0] : null)
    readonly property string statusLine: {
        if (busy)
            return busyLabel;
        if (!statusReady)
            return I18n.tr("Checking...");
        if (!enabledOnDisk || !status.installed)
            return I18n.tr("Not set up");
        if (!primaryDevice)
            return I18n.tr("No phone paired yet");
        var name = primaryDevice.name || primaryDevice.id;
        if (primaryDevice.reachable) {
            if (primaryDevice.battery >= 0)
                return I18n.tr("Connected to %1 · %2%").arg(name).arg(primaryDevice.battery);
            return I18n.tr("Connected to %1").arg(name);
        }
        return I18n.tr("Paired with %1 · not connected").arg(name);
    }
    readonly property string qualityMode: {
        var size = Number(value("phone.scrcpy.maxSize", 0));
        var fps = Number(value("phone.scrcpy.maxFps", 60));
        var rate = String(value("phone.scrcpy.videoBitRate", "8M"));
        if (size === 1024 && fps === 30 && rate === "4M") return "saver";
        if (size === 1600 && fps === 60 && rate === "8M") return "balanced";
        if (size === 0 && fps === 60 && rate === "16M") return "sharp";
        return "custom";
    }
    readonly property var qualityOptions: qualityMode === "custom"
        ? ["saver", "balanced", "sharp", "custom"] : ["saver", "balanced", "sharp"]
    readonly property string resolutionMode: {
        var width = Number(value("phone.webcam.width", 1280));
        var height = Number(value("phone.webcam.height", 720));
        if (width === 1280 && height === 720) return "720p";
        if (width === 1920 && height === 1080) return "1080p";
        return "";
    }

    opacity: entered ? 1 : 0
    x: entered ? 0 : Tokens.s3
    Behavior on opacity {
        enabled: !Tokens.reduceMotion
        NumberAnimation { duration: Tokens.durSmall; easing.type: Easing.OutCubic }
    }
    Behavior on x {
        enabled: !Tokens.reduceMotion
        NumberAnimation { duration: Tokens.durSmall; easing.type: Easing.OutCubic }
    }
    onPhoneActiveChanged: stageSections()
    Component.onCompleted: {
        entered = true;
        stageSections();
    }

    function stageSections() {
        pg.sectionsReady = false;
        if (!pg.phoneActive)
            return;
        Qt.callLater(function() {
            if (!pg.phoneActive)
                return;
            cards.lay();
            Qt.callLater(function() {
                if (pg.phoneActive)
                    pg.sectionsReady = true;
            });
        });
    }

    function value(path, fallback) {
        void Settings.revision;
        var current = Settings.get(path);
        return current === undefined ? fallback : current;
    }

    function setValue(path, next) {
        pg.error = "";
        Settings.patch(path, next, function(ok, why) {
            if (!ok)
                pg.error = why || I18n.tr("Could not save this phone setting.");
        });
    }

    function setValues(changes, index) {
        var at = index || 0;
        if (at >= changes.length)
            return;
        Settings.patch(changes[at].path, changes[at].value, function(ok, why) {
            if (!ok) {
                pg.error = why || I18n.tr("Could not save this phone setting.");
                return;
            }
            pg.setValues(changes, at + 1);
        });
    }

    function setQuality(mode) {
        var presets = {
            saver: { size: 1024, fps: 30, rate: "4M" },
            balanced: { size: 1600, fps: 60, rate: "8M" },
            sharp: { size: 0, fps: 60, rate: "16M" }
        };
        var preset = presets[mode];
        if (!preset)
            return;
        pg.error = "";
        setValues([
            { path: "phone.scrcpy.maxSize", value: preset.size },
            { path: "phone.scrcpy.maxFps", value: preset.fps },
            { path: "phone.scrcpy.videoBitRate", value: preset.rate }
        ], 0);
    }

    function setResolution(mode) {
        if (mode !== "720p" && mode !== "1080p")
            return;
        pg.error = "";
        setValues([
            { path: "phone.webcam.width", value: mode === "720p" ? 1280 : 1920 },
            { path: "phone.webcam.height", value: mode === "720p" ? 720 : 1080 }
        ], 0);
    }

    function refresh() {
        if (!statusProc.running)
            statusProc.running = true;
    }

    function setEnabled(on) {
        if (pg.busy || !pg.status.supported)
            return;
        pg.busy = true;
        pg.error = "";
        pg.busyLabel = on ? I18n.tr("Installing...") : I18n.tr("Removing...");
        if (!on)
            pg.setValue("phone.enabled", false);
        switchProc.turningOn = on;
        switchProc.command = ["ryoku-hub", "phone", on ? "on" : "off"];
        switchProc.running = true;
    }

    function unpair(id) {
        if (deviceProc.running)
            return;
        pg.actionDevice = id;
        pg.error = "";
        deviceProc.command = ["ryoku-hub", "phone", "unpair", id];
        deviceProc.running = true;
    }

    Process {
        id: statusProc
        command: ["ryoku-hub", "phone", "status"]
        environment: Spawn.env
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    pg.status = JSON.parse(this.text || "{}");
                } catch (e) {
                    pg.error = I18n.tr("Phone status returned invalid data.");
                }
            }
        }
        stderr: StdioCollector {
            onStreamFinished: if (this.text.trim()) pg.error = this.text.trim()
        }
        onExited: function(code) {
            pg.statusReady = true;
            if (code !== 0 && pg.error === "")
                pg.error = I18n.tr("Could not read phone support on this system.");
        }
    }

    Process {
        id: switchProc
        property bool turningOn: false
        environment: Spawn.env
        stderr: StdioCollector { id: switchError }
        onExited: function(code) {
            if (code === 0 && turningOn) {
                Settings.patch("phone.enabled", true, function(ok, why) {
                    pg.busy = false;
                    pg.busyLabel = "";
                    if (!ok)
                        pg.error = why || I18n.tr("Phone tools installed, but Phone could not be enabled.");
                    pg.refresh();
                });
                return;
            }
            pg.busy = false;
            pg.busyLabel = "";
            if (code !== 0) {
                var lines = switchError.text.trim().split("\n");
                var detail = "";
                for (var i = 0; i < lines.length; i++) {
                    if (lines[i].trim().indexOf("error:") === 0) {
                        detail = lines[i].trim();
                        break;
                    }
                }
                pg.error = detail || switchError.text.trim()
                    || (turningOn ? I18n.tr("Phone tools could not be installed.") : I18n.tr("Phone tools could not be removed."));
            }
            pg.refresh();
        }
    }

    Process {
        id: deviceProc
        environment: Spawn.env
        stderr: StdioCollector {
            onStreamFinished: if (this.text.trim()) pg.error = this.text.trim()
        }
        onExited: function(code) {
            if (code !== 0 && pg.error === "")
                pg.error = I18n.tr("The phone could not be unpaired.");
            pg.actionDevice = "";
            pg.refresh();
        }
    }

    component PhoneSetting: SettingRow {
        id: row
        required property var spec
        required property int rowIndex
        readonly property var currentValue: pg.value(spec.path, spec.fallback)
        width: parent ? parent.width : 0
        label: spec.label
        desc: spec.desc || ""
        divider: rowIndex > 0
        source: "shell.json"
        enabled: Settings.ready && pg.featureReady && !pg.busy && (spec.available === undefined || spec.available)
        block: spec.kind === "seg" && (spec.options || []).length > 2
        footH: spec.kind === "text" || spec.kind === "list" ? 32 : 0
        controlWidth: spec.kind === "sw" ? 54
            : spec.kind === "step" ? 82
            : spec.kind === "seg" ? Math.max(150, (spec.options || []).length * 72)
            : spec.kind === "slid" ? 210 : 240
        value: spec.kind === "slid" ? String(Math.round(Number(currentValue))) : ""
        unit: spec.kind === "slid" ? (spec.unit || "") : ""
        Loader {
            anchors.fill: parent
            sourceComponent: row.spec.kind === "sw" ? swControl
                : row.spec.kind === "step" ? stepControl
                : row.spec.kind === "seg" ? segControl
                : row.spec.kind === "slid" ? slidControl : textControl
        }
        Component {
            id: swControl
            Sw {
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                on: row.currentValue === true
                onToggled: function(next) { pg.setValue(row.spec.path, next); }
            }
        }
        Component {
            id: stepControl
            Step {
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                from: row.spec.lo
                to: row.spec.hi
                stepBy: row.spec.step || 1
                value: Number(row.currentValue)
                onModified: function(next) { pg.setValue(row.spec.path, next); }
            }
        }
        Component {
            id: segControl
            Seg {
                anchors.left: row.block ? parent.left : undefined
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                options: row.spec.options || []
                labels: row.spec.labels || ({})
                current: String(row.currentValue)
                onChose: function(next) { pg.setValue(row.spec.path, next); }
            }
        }
        Component {
            id: slidControl
            Slid {
                anchors.fill: parent
                from: row.spec.lo
                to: row.spec.hi
                value: Number(row.currentValue)
                onModified: function(next) { pg.setValue(row.spec.path, Math.round(next)); }
            }
        }
        Component {
            id: textControl
            Field {
                anchors.fill: parent
                tabular: true
                placeholder: row.spec.placeholder || ""
                text: row.spec.kind === "list" && Array.isArray(row.currentValue)
                    ? row.currentValue.join(", ") : String(row.currentValue)
                onCommitted: function(next) {
                    if (row.spec.kind === "list") {
                        var values = next.split(",").map(function(v) { return v.trim(); }).filter(function(v) { return v !== ""; });
                        pg.setValue(row.spec.path, values);
                    } else {
                        pg.setValue(row.spec.path, next);
                    }
                }
            }
        }
    }

    component PhoneSettingEntry: Item {
        id: entry
        required property var spec
        required property int rowIndex
        property bool entered: Tokens.reduceMotion
        width: parent ? parent.width : 0
        implicitHeight: setting.implicitHeight
        opacity: entered ? 1 : 0
        x: entered ? 0 : Tokens.s2
        Behavior on opacity {
            enabled: !Tokens.reduceMotion
            NumberAnimation { duration: Tokens.durSmall; easing.type: Easing.OutCubic }
        }
        Behavior on x {
            enabled: !Tokens.reduceMotion
            NumberAnimation { duration: Tokens.durSmall; easing.type: Easing.OutCubic }
        }
        Timer {
            running: !Tokens.reduceMotion
            interval: 30 + Math.min(entry.rowIndex, 8) * 18
            onTriggered: entry.entered = true
        }
        PhoneSetting {
            id: setting
            anchors.left: parent.left
            anchors.right: parent.right
            spec: entry.spec
            rowIndex: entry.rowIndex
        }
    }

    component PhoneSectionCard: SettingCard {
        id: sectionCard
        opacity: pg.sectionsReady ? 1 : 0
        enabled: pg.sectionsReady
        transform: Translate {
            id: sectionShift
            y: pg.sectionsReady || Tokens.reduceMotion ? 0 : Tokens.s2
            Behavior on y {
                enabled: !Tokens.reduceMotion
                NumberAnimation { duration: Tokens.durSmall; easing.type: Easing.OutCubic }
            }
        }
        Behavior on opacity {
            enabled: !Tokens.reduceMotion
            NumberAnimation { duration: Tokens.durSmall; easing.type: Easing.OutCubic }
        }
    }

    component PhoneGroupLabel: Item {
        id: groupLabel
        required property string title
        width: parent ? parent.width : 0
        implicitHeight: 38
        Text {
            anchors.left: parent.left
            anchors.leftMargin: Tokens.s4
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr(groupLabel.title)
            color: Tokens.inkFaint
            font.family: Tokens.mono
            font.pixelSize: Tokens.fTiny
            font.letterSpacing: Tokens.trackMark
        }
    }

    readonly property var notificationRows: [
        { path: "phone.desktopNotifications", label: "Show phone notifications", desc: "Show phone alerts on this PC.", kind: "sw", fallback: true },
        { path: "phone.contacts.enabled", label: "Load contacts", desc: "Use names for calls, messages and search.", kind: "sw", fallback: true }
    ]
    readonly property var mirrorRows: [
        { path: "phone.scrcpy.turnScreenOff", label: "Phone screen off", desc: "Keep the phone display dark while mirroring.", kind: "sw", fallback: false },
        { path: "phone.scrcpy.audio", label: "Phone audio here", desc: "Play phone audio through this PC.", kind: "sw", fallback: true },
        { path: "phone.scrcpy.clipboardSync", label: "Shared clipboard", desc: "Copy and paste between both devices.", kind: "sw", fallback: true },
        { path: "phone.scrcpy.stayAwake", label: "Keep phone awake", desc: "Prevent sleep while the phone is connected.", kind: "sw", fallback: true }
    ]
    readonly property var microphoneRows: [
        { path: "phone.microphone.micGain", label: "Gain", desc: "Adjust the phone microphone level.", kind: "slid", lo: 0, hi: 200, unit: "%", fallback: 100 },
        { path: "phone.microphone.monitor", label: "Hear yourself", desc: "Play the phone microphone locally.", kind: "sw", fallback: false },
        { path: "phone.microphone.setDefault", label: "Use as default", desc: "Make the phone this PC's default microphone.", kind: "sw", fallback: false }
    ]
    readonly property var advancedWirelessRows: [
        { path: "phone.scrcpy.useWireless", label: "Wireless debugging", desc: "Use Android debugging over the local network.", kind: "sw", fallback: false },
        { path: "phone.scrcpy.autoWirelessIp", label: "Find address automatically", desc: "Use the phone's discovered network address.", kind: "sw", fallback: true },
        { path: "phone.scrcpy.wirelessIp", label: "Wireless address", desc: "Manual address when discovery is off.", kind: "text", placeholder: "192.168.1.20", fallback: "" },
        { path: "phone.scrcpy.wirelessPort", label: "Wireless port", desc: "Android's wireless debugging port.", kind: "step", lo: 1, hi: 65535, step: 1, fallback: 5555 },
        { path: "phone.scrcpy.pinAdbPort", label: "Keep this port", desc: "Ignore ports found automatically.", kind: "sw", fallback: false }
    ]
    readonly property var advancedSessionRows: [
        { path: "phone.scrcpy.keyboardMode", label: "Keyboard mode", desc: "Hardware mode supports layouts and shortcuts.", kind: "seg", options: ["uhid", "sdk"], labels: ({ uhid: "Hardware", sdk: "Compatible" }), fallback: "uhid" },
        { path: "phone.scrcpy.autoResume", label: "Resume sessions", desc: "Reconnect interrupted phone windows.", kind: "sw", fallback: true },
        { path: "phone.scrcpy.showTouches", label: "Show touches", desc: "Draw touch points on the mirrored screen.", kind: "sw", fallback: false },
        { path: "phone.scrcpy.recording.folder", label: "Recording folder", desc: "Leave empty to use Videos.", kind: "text", placeholder: "~/Videos", fallback: "" }
    ]
    readonly property var advancedAppRows: [
        { path: "phone.scrcpy.appMode.showAppIcons", label: "App icons", desc: "Load icons for Android app windows.", kind: "sw", fallback: true },
        { path: "phone.scrcpy.appMode.autoUnlock", label: "Unlock before launch", desc: "Open a trusted lock screen before an app.", kind: "sw", fallback: true },
        { path: "phone.scrcpy.appMode.flexDisplay", label: "Flexible display", desc: "Use a dedicated Android display for apps.", kind: "sw", fallback: false },
        { path: "phone.scrcpy.appMode.displayWidth", label: "Display width", desc: "Virtual display width in pixels.", kind: "step", lo: 320, hi: 7680, step: 80, fallback: 1280 },
        { path: "phone.scrcpy.appMode.displayHeight", label: "Display height", desc: "Virtual display height in pixels.", kind: "step", lo: 320, hi: 7680, step: 80, fallback: 960 },
        { path: "phone.scrcpy.appMode.displayDpi", label: "Display density", desc: "Virtual display density in DPI.", kind: "step", lo: 72, hi: 640, step: 8, fallback: 240 },
        { path: "phone.scrcpy.appMode.onSessionEnd", label: "After closing", desc: "Choose what the phone does when an app closes.", kind: "seg", options: ["home", "continue", "lock"], labels: ({ "home": "Home", "continue": "Continue", "lock": "Lock" }), fallback: "home" }
    ]
    readonly property var advancedDeviceRows: [
        { path: "phone.contacts.hideUnnamed", label: "Hide unnamed contacts", desc: "Leave unsaved numbers out of contacts.", kind: "sw", fallback: true },
        { path: "phone.webcam.videoDevice", label: "Webcam device", desc: "Leave empty to use the managed device.", kind: "text", placeholder: "/dev/video10", fallback: "" },
        { path: "phone.microphone.enabled", label: "Phone microphone source", desc: "Expose the phone as a PipeWire input.", kind: "sw", fallback: false }
    ]
    readonly property var advancedAlarmRows: [
        { path: "phone.alarm.enabled", label: "Phone alarms", desc: "Show the next alarm and alert when it rings.", kind: "sw", fallback: true },
        { path: "phone.alarm.pollMinutes", label: "Check every", desc: "Minutes between alarm checks.", kind: "step", lo: 1, hi: 1440, step: 1, fallback: 10 },
        { path: "phone.alarm.clockApps", label: "Clock apps", desc: "Comma-separated Android clock packages.", kind: "list", placeholder: "com.google.android.deskclock", fallback: [] },
        { path: "phone.alarm.ignorePackages", label: "Ignored packages", desc: "Comma-separated packages that never count as alarms.", kind: "list", placeholder: "com.example.timer", fallback: [] }
    ]

    Flickable {
        id: flick
        anchors.fill: parent
        clip: true
        contentWidth: width
        contentHeight: content.implicitHeight + Tokens.s4
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
        WheelScroll { }

        Column {
            id: content
            width: flick.width - Tokens.s4
            spacing: Tokens.s4

            CardColumns {
                id: cards
                width: parent.width
                minColumnWidth: 400

                SettingCard {
                    property bool fullWidth: true
                    width: cards.width
                    title: I18n.tr("PHONE")
                    collapsible: false

                    SettingRow {
                        width: parent.width
                        label: pg.statusLine
                        desc: !pg.statusReady ? I18n.tr("Checking phone support...")
                            : !pg.status.supported ? (pg.status.reason || I18n.tr("Phone is unavailable on this system."))
                            : pg.primaryDevice ? I18n.tr("Setup and everyday controls live in Kit.")
                            : I18n.tr("Kit guides pairing, mirroring and sharing.")
                        controlWidth: statusActions.implicitWidth
                        enabled: Settings.ready && !pg.busy
                        Row {
                            id: statusActions
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            spacing: Tokens.s2
                            Sw {
                                enabled: pg.statusReady && pg.status.supported
                                opacity: pg.statusReady ? 1 : 0.4
                                Behavior on opacity {
                                    enabled: !Tokens.reduceMotion
                                    NumberAnimation { duration: Tokens.durSmall; easing.type: Easing.OutCubic }
                                }
                                anchors.verticalCenter: parent.verticalCenter
                                on: pg.enabledOnDisk && pg.status.installed
                                onToggled: function(next) { pg.setEnabled(next); }
                            }
                            Btn {
                                anchors.verticalCenter: parent.verticalCenter
                                primary: true
                                armed: pg.statusReady && !pg.busy
                                opacity: pg.statusReady ? (armed ? 1 : 0.5) : 0
                                text: pg.primaryDevice ? I18n.tr("Open Phone") : I18n.tr("Set up your phone")
                                onAct: Spawn.runApp(["ryoku-kit", "phone"], { label: "ryoku-kit" })
                            }
                        }
                    }

                    Item {
                        width: parent.width
                        implicitHeight: pg.busy ? 4 : 0
                        opacity: pg.busy ? 1 : 0
                        clip: true
                        Behavior on implicitHeight {
                            enabled: !Tokens.reduceMotion
                            NumberAnimation { duration: Tokens.durSmall; easing.type: Easing.OutCubic }
                        }
                        Behavior on opacity {
                            enabled: !Tokens.reduceMotion
                            NumberAnimation { duration: Tokens.durSmall; easing.type: Easing.OutCubic }
                        }
                        Rectangle {
                            anchors.left: parent.left
                            anchors.right: parent.right
                            height: 2
                            color: Tokens.tint5
                            Rectangle {
                                id: installProgress
                                width: Math.max(32, parent.width * 0.36)
                                height: parent.height
                                color: Tokens.ink
                                x: Tokens.reduceMotion ? 0 : -width
                                NumberAnimation on x {
                                    running: pg.busy && !Tokens.reduceMotion && pg.visible
                                    loops: Animation.Infinite
                                    from: -installProgress.width
                                    to: installProgress.parent.width
                                    duration: Tokens.durXl
                                    easing.type: Easing.OutCubic
                                }
                            }
                        }
                    }

                    Item {
                        visible: pg.error !== ""
                        width: parent.width
                        implicitHeight: visible ? errorText.implicitHeight + Tokens.s3 * 2 : 0
                        Text {
                            id: errorText
                            anchors.left: parent.left
                            anchors.right: parent.right
                            anchors.margins: Tokens.s4
                            anchors.verticalCenter: parent.verticalCenter
                            text: pg.error
                            color: Tokens.ink
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall
                            wrapMode: Text.WordWrap
                        }
                    }
                }

                PhoneSectionCard {
                    width: cards.colWidth
                    visible: pg.phoneActive
                    title: I18n.tr("NOTIFICATIONS")
                    collapsible: false
                    Repeater {
                        model: pg.notificationRows
                        delegate: PhoneSettingEntry {
                            required property var modelData
                            required property int index
                            spec: modelData
                            rowIndex: index
                        }
                    }
                }

                PhoneSectionCard {
                    width: cards.colWidth
                    visible: pg.phoneActive
                    title: I18n.tr("SCREEN MIRRORING")
                    collapsible: false
                    SettingRow {
                        width: parent.width
                        label: I18n.tr("Quality")
                        desc: I18n.tr("Choose detail and network use together.")
                        block: true
                        divider: false
                        enabled: pg.featureReady && !pg.busy
                        source: "shell.json"
                        Seg {
                            anchors.left: parent.left
                            anchors.right: parent.right
                            options: pg.qualityOptions
                            labels: ({ saver: "Saver", balanced: "Balanced", sharp: "Sharp", custom: "Custom" })
                            current: pg.qualityMode
                            onChose: function(next) { pg.setQuality(next); }
                        }
                    }
                    Repeater {
                        model: pg.mirrorRows
                        delegate: PhoneSettingEntry {
                            required property var modelData
                            required property int index
                            spec: modelData
                            rowIndex: index + 1
                        }
                    }
                }

                PhoneSectionCard {
                    width: cards.colWidth
                    visible: pg.phoneActive
                    title: I18n.tr("WEBCAM")
                    collapsible: false
                    SettingRow {
                        id: webcamSetup
                        visible: pg.statusReady && pg.phoneActive && !pg.status.configured
                        width: parent.width
                        label: I18n.tr("Webcam setup incomplete")
                        desc: I18n.tr("Restore the camera module used by video apps.")
                        controlWidth: finishSetup.implicitWidth
                        enabled: !pg.busy
                        Btn {
                            id: finishSetup
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            primary: true
                            text: I18n.tr("Finish setup")
                            armed: !pg.busy
                            onAct: pg.setEnabled(true)
                        }
                    }
                    SettingRow {
                        width: parent.width
                        label: I18n.tr("Camera")
                        divider: webcamSetup.visible
                        desc: I18n.tr("Choose the lens used as a webcam.")
                        block: true
                        enabled: pg.featureReady && !pg.busy
                        source: "shell.json"
                        Seg {
                            anchors.left: parent.left
                            anchors.right: parent.right
                            options: ["back", "front"]
                            labels: ({ back: "Back", front: "Front" })
                            current: String(pg.value("phone.webcam.cameraFacing", "back"))
                            onChose: function(next) { pg.setValue("phone.webcam.cameraFacing", next); }
                        }
                    }
                    SettingRow {
                        width: parent.width
                        divider: true
                        label: I18n.tr("Resolution")
                        desc: I18n.tr("Set the webcam picture size.")
                        block: true
                        enabled: pg.featureReady && !pg.busy
                        source: "shell.json"
                        Seg {
                            anchors.left: parent.left
                            anchors.right: parent.right
                            options: ["720p", "1080p"]
                            current: pg.resolutionMode
                            onChose: function(next) { pg.setResolution(next); }
                        }
                    }
                    SettingRow {
                        width: parent.width
                        divider: true
                        label: I18n.tr("Frame rate")
                        desc: I18n.tr("Choose smoothness for video calls.")
                        controlWidth: 150
                        enabled: pg.featureReady && !pg.busy
                        source: "shell.json"
                        Seg {
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            options: ["30", "60"]
                            labels: ({ "30": "30", "60": "60" })
                            current: String(pg.value("phone.webcam.fps", 30))
                            onChose: function(next) { pg.setValue("phone.webcam.fps", Number(next)); }
                        }
                    }
                }

                PhoneSectionCard {
                    width: cards.colWidth
                    visible: pg.phoneActive
                    title: I18n.tr("MICROPHONE")
                    collapsible: false
                    Repeater {
                        model: pg.microphoneRows
                        delegate: PhoneSettingEntry {
                            required property var modelData
                            required property int index
                            spec: modelData
                            rowIndex: index
                        }
                    }
                }

                PhoneSectionCard {
                    width: cards.colWidth
                    visible: pg.phoneActive
                    title: I18n.tr("PAIRED DEVICES")
                    collapsible: false
                    Text {
                        visible: pg.pairedDevices.length === 0
                        width: parent.width
                        leftPadding: Tokens.s4
                        rightPadding: Tokens.s4
                        topPadding: Tokens.s3
                        bottomPadding: Tokens.s3
                        text: I18n.tr("No phone is paired. Open Phone to set one up.")
                        color: Tokens.inkMuted
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                        wrapMode: Text.WordWrap
                    }
                    Repeater {
                        model: pg.pairedDevices
                        delegate: Item {
                            id: deviceEntry
                            required property var modelData
                            required property int index
                            property bool entered: Tokens.reduceMotion
                            width: parent.width
                            implicitHeight: deviceRow.implicitHeight
                            opacity: entered ? 1 : 0
                            x: entered ? 0 : Tokens.s2
                            Behavior on opacity {
                                enabled: !Tokens.reduceMotion
                                NumberAnimation { duration: Tokens.durSmall; easing.type: Easing.OutCubic }
                            }
                            Behavior on x {
                                enabled: !Tokens.reduceMotion
                                NumberAnimation { duration: Tokens.durSmall; easing.type: Easing.OutCubic }
                            }
                            Timer {
                                running: !Tokens.reduceMotion
                                interval: 40 + Math.min(deviceEntry.index, 6) * 35
                                onTriggered: deviceEntry.entered = true
                            }
                            SettingRow {
                                id: deviceRow
                                anchors.left: parent.left
                                anchors.right: parent.right
                                divider: deviceEntry.index > 0
                                label: deviceEntry.modelData.name || deviceEntry.modelData.id
                                desc: deviceEntry.modelData.reachable ? I18n.tr("Connected") : I18n.tr("Not connected")
                                controlWidth: 100
                                Btn {
                                    anchors.right: parent.right
                                    anchors.verticalCenter: parent.verticalCenter
                                    compact: true
                                    armed: pg.actionDevice === ""
                                    text: pg.actionDevice === deviceEntry.modelData.id ? I18n.tr("WORKING") : I18n.tr("UNPAIR")
                                    onAct: pg.unpair(deviceEntry.modelData.id)
                                }
                            }
                        }
                    }
                }

                PhoneSectionCard {
                    property bool fullWidth: true
                    width: cards.width
                    visible: pg.phoneActive
                    title: I18n.tr("ADVANCED")
                    expanded: false
                    summary: I18n.tr("WIRELESS, APP WINDOWS, ALARMS")

                    PhoneGroupLabel { title: "WIRELESS DEBUGGING" }
                    Repeater {
                        model: pg.advancedWirelessRows
                        delegate: PhoneSettingEntry {
                            required property var modelData
                            required property int index
                            spec: modelData
                            rowIndex: index
                        }
                    }
                    PhoneGroupLabel { title: "SESSIONS" }
                    Repeater {
                        model: pg.advancedSessionRows
                        delegate: PhoneSettingEntry {
                            required property var modelData
                            required property int index
                            spec: modelData
                            rowIndex: index
                        }
                    }
                    PhoneGroupLabel { title: "APP WINDOWS" }
                    Repeater {
                        model: pg.advancedAppRows
                        delegate: PhoneSettingEntry {
                            required property var modelData
                            required property int index
                            spec: modelData
                            rowIndex: index
                        }
                    }
                    PhoneGroupLabel { title: "DEVICES" }
                    Repeater {
                        model: pg.advancedDeviceRows
                        delegate: PhoneSettingEntry {
                            required property var modelData
                            required property int index
                            spec: modelData
                            rowIndex: index
                        }
                    }
                    PhoneGroupLabel { title: "ALARMS" }
                    Repeater {
                        model: pg.advancedAlarmRows
                        delegate: PhoneSettingEntry {
                            required property var modelData
                            required property int index
                            spec: modelData
                            rowIndex: index
                        }
                    }
                }
            }
        }
    }
}

pragma ComponentBehavior: Bound

import QtQuick
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "world-land.js" as World
import "tzmap.js" as Tz

// The world-map time-zone picker (an Ubuntu-style chooser, Ryoku-toned): a live
// equirectangular map with a day/night terminator, a dot per zone, click-to-pick
// the nearest zone, a dropped pin, a fuzzy search, and the selected zone's live
// local clock. Pure UI: it emits `applied(zone)` and lets the host run the
// privileged `timedatectl`. Full-page overlay, mirroring PickFile.
Item {
    id: tzp

    property bool active: false
    property string currentZone: ""     // the system zone, shown as the accent dot
    property var zones: []              // [{tz,lat,lon,cc,note}]
    property var selected: null
    property var hovered: null
    property string query: ""
    property string localTime: ""

    signal applied(string zone)
    signal canceled()

    function open() {
        tzp.query = "";
        tzp.active = true;
        zoneLoad.running = true;
        card.forceActiveFocus();
    }
    function close() { tzp.active = false; }

    function selectByName(name) {
        if (!name || !tzp.zones || tzp.zones.length === 0)
            return;
        for (var i = 0; i < tzp.zones.length; i++) {
            if (tzp.zones[i].tz === name) { tzp.selected = tzp.zones[i]; break; }
        }
        tzp.refreshClock();
    }
    function pick(z) {
        if (!z) return;
        tzp.selected = z;
        tzp.refreshClock();
        pin.bounce();
    }
    function refreshClock() {
        if (!tzp.selected) { tzp.localTime = ""; return; }
        clockProc.command = ["env", "TZ=" + tzp.selected.tz, "date", TimeFormat.is24h ? "+%H:%M  %Z  UTC%z" : "+%-I:%M %p  %Z  UTC%z"];
        clockProc.running = false;
        clockProc.running = true;
    }

    Connections {
        target: TimeFormat
        function onIs24hChanged() { if (tzp.active) tzp.refreshClock(); }
    }

    readonly property var results: Tz.search(tzp.zones, tzp.query)
    readonly property bool changed: tzp.selected && tzp.selected.tz !== tzp.currentZone

    // canvas palette, read as plain props so a theme change triggers a repaint
    readonly property color ocean: Tokens.paper
    readonly property color landCol: Qt.rgba(Tokens.ink.r, Tokens.ink.g, Tokens.ink.b, 0.16)
    readonly property color landEdge: Tokens.line
    readonly property color gridCol: Tokens.lineSoft
    readonly property color nightCol: Qt.rgba(0.03, 0.05, 0.13, 0.42)
    readonly property color termCol: Qt.rgba(Tokens.sun.r, Tokens.sun.g, Tokens.sun.b, 0.55)
    onOceanChanged: { land.requestPaint(); sky.requestPaint(); }

    onZonesChanged: selectByName(tzp.currentZone)
    onCurrentZoneChanged: selectByName(tzp.currentZone)

    anchors.fill: parent
    visible: tzp.active
    z: 220

    Process {
        id: zoneLoad
        command: ["sh", "-c", "cat /usr/share/zoneinfo/zone1970.tab 2>/dev/null || cat /usr/share/zoneinfo/zone.tab 2>/dev/null"]
        stdout: StdioCollector {
            onStreamFinished: {
                tzp.zones = Tz.parseZoneTab(text);
                tzp.selectByName(tzp.currentZone);
            }
        }
    }
    Process {
        id: clockProc
        stdout: StdioCollector { onStreamFinished: tzp.localTime = ("" + text).trim() }
    }
    Timer { interval: 20000; running: tzp.active && tzp.selected !== null; repeat: true; onTriggered: tzp.refreshClock() }
    Timer { interval: 60000; running: tzp.active; repeat: true; onTriggered: sky.requestPaint() }

    // scrim: mute the page behind, and dismiss on an outside click
    Rectangle { anchors.fill: parent; color: Qt.rgba(Tokens.paper.r, Tokens.paper.g, Tokens.paper.b, 0.72) }
    MouseArea { anchors.fill: parent; onClicked: tzp.canceled() }

    Rectangle {
        id: card
        anchors.centerIn: parent
        width: Math.min(parent.width - Tokens.s7 * 2, 1040)
        height: Math.min(parent.height - Tokens.s6 * 2, 760)
        radius: Tokens.radius
        color: Tokens.paperLift
        border.width: Tokens.border
        border.color: Tokens.lineStrong
        focus: tzp.active
        Keys.onEscapePressed: tzp.canceled()
        MouseArea { anchors.fill: parent; onClicked: {} }

        Text {
            id: cTitle
            anchors { left: parent.left; top: parent.top; leftMargin: Tokens.s5; topMargin: Tokens.s4 }
            text: I18n.tr("TIME ZONE")
            color: Tokens.ink
            font.family: Tokens.ui
            font.pixelSize: Tokens.fMicro
            font.weight: Font.Medium
            font.letterSpacing: Tokens.trackLabel
        }
        Text {
            anchors { left: cTitle.left; top: cTitle.bottom; topMargin: Tokens.s1 }
            text: I18n.tr("Click the map or search to choose your zone")
            color: Tokens.inkFaint
            font.family: Tokens.ui
            font.pixelSize: Tokens.fTiny
        }
        IconBtn {
            id: cClose
            anchors { right: parent.right; top: parent.top; rightMargin: Tokens.s4; topMargin: Tokens.s4 }
            glyph: "\u00d7"
            onAct: tzp.canceled()
        }

        Field {
            id: search
            anchors { right: cClose.left; top: parent.top; rightMargin: Tokens.s3; topMargin: Tokens.s4 }
            width: 320
            placeholder: I18n.tr("Search a city or zone")
            onEdited: (v) => tzp.query = v
            onAccepted: { if (tzp.results.length > 0) { tzp.pick(tzp.results[0]); search.clear(); tzp.query = ""; } }
        }

        // the map: an equirectangular 2:1 canvas stack, centred in the free area
        Item {
            id: mapArea
            anchors {
                left: parent.left; right: parent.right
                top: search.bottom; bottom: foot.top
                leftMargin: Tokens.s5; rightMargin: Tokens.s5
                topMargin: Tokens.s4; bottomMargin: Tokens.s3
            }

            Item {
                id: mapWrap
                anchors.centerIn: parent
                width: Math.min(mapArea.width, mapArea.height * 2)
                height: width / 2
                clip: true

                Canvas {
                    id: land
                    anchors.fill: parent
                    onWidthChanged: requestPaint()
                    onHeightChanged: requestPaint()
                    onPaint: {
                        var ctx = getContext("2d");
                        var w = width, h = height;
                        if (w <= 0 || h <= 0) return;
                        ctx.reset();
                        ctx.fillStyle = tzp.ocean;
                        ctx.fillRect(0, 0, w, h);
                        ctx.strokeStyle = tzp.gridCol;
                        ctx.lineWidth = 1;
                        for (var lon = -150; lon < 180; lon += 30) {
                            var gx = Tz.px(lon, w);
                            ctx.beginPath(); ctx.moveTo(gx, 0); ctx.lineTo(gx, h); ctx.stroke();
                        }
                        for (var lat = -60; lat < 90; lat += 30) {
                            var gy = Tz.py(lat, h);
                            ctx.beginPath(); ctx.moveTo(0, gy); ctx.lineTo(w, gy); ctx.stroke();
                        }
                        ctx.fillStyle = tzp.landCol;
                        ctx.strokeStyle = tzp.landEdge;
                        ctx.lineWidth = 0.6;
                        var rings = World.rings;
                        for (var i = 0; i < rings.length; i++) {
                            var r = rings[i];
                            if (r.length < 3) continue;
                            ctx.beginPath();
                            for (var j = 0; j < r.length; j++) {
                                var X = Tz.px(r[j][0], w), Y = Tz.py(r[j][1], h);
                                if (j === 0) ctx.moveTo(X, Y); else ctx.lineTo(X, Y);
                            }
                            ctx.closePath();
                            ctx.fill();
                            ctx.stroke();
                        }
                    }
                }

                Canvas {
                    id: sky
                    anchors.fill: parent
                    onWidthChanged: requestPaint()
                    onHeightChanged: requestPaint()
                    onPaint: {
                        var ctx = getContext("2d");
                        var w = width, h = height;
                        if (w <= 0 || h <= 0) return;
                        ctx.reset();
                        var sub = Tz.subsolar(new Date());
                        ctx.fillStyle = tzp.nightCol;
                        var step = 3;
                        for (var x = 0; x < w; x += step) {
                            var lon = Tz.lonAt(x + step / 2, w);
                            var tb = Tz.terminatorLat(lon, sub);
                            if (tb === null) {
                                if (!Tz.isDay(0, lon, sub)) ctx.fillRect(x, 0, step, h);
                            } else {
                                var yb = Tz.py(tb, h);
                                if (sub.lat > 0) ctx.fillRect(x, yb, step, h - yb);
                                else ctx.fillRect(x, 0, step, yb);
                            }
                        }
                        ctx.strokeStyle = tzp.termCol;
                        ctx.lineWidth = 1.4;
                        ctx.beginPath();
                        var started = false;
                        for (var x2 = 0; x2 <= w; x2 += step) {
                            var lon2 = Tz.lonAt(x2, w);
                            var tb2 = Tz.terminatorLat(lon2, sub);
                            if (tb2 === null) { started = false; continue; }
                            var yb2 = Tz.py(tb2, h);
                            if (!started) { ctx.moveTo(x2, yb2); started = true; } else ctx.lineTo(x2, yb2);
                        }
                        ctx.stroke();
                        var sx = Tz.px(sub.lon, w), sy = Tz.py(sub.lat, h);
                        var g = ctx.createRadialGradient(sx, sy, 0, sx, sy, 28);
                        g.addColorStop(0, Qt.rgba(Tokens.sun.r, Tokens.sun.g, Tokens.sun.b, 0.55));
                        g.addColorStop(1, Qt.rgba(Tokens.sun.r, Tokens.sun.g, Tokens.sun.b, 0));
                        ctx.fillStyle = g;
                        ctx.beginPath(); ctx.arc(sx, sy, 28, 0, 2 * Math.PI); ctx.fill();
                        ctx.fillStyle = Tokens.sun;
                        ctx.beginPath(); ctx.arc(sx, sy, 4, 0, 2 * Math.PI); ctx.fill();
                    }
                }

                Repeater {
                    model: tzp.zones
                    delegate: Rectangle {
                        id: dot
                        required property var modelData
                        readonly property bool isCur: tzp.currentZone === modelData.tz
                        width: isCur ? 7 : 4
                        height: width
                        radius: width / 2
                        x: Tz.px(modelData.lon, mapWrap.width) - width / 2
                        y: Tz.py(modelData.lat, mapWrap.height) - height / 2
                        color: isCur ? Tokens.sun : Qt.rgba(Tokens.ink.r, Tokens.ink.g, Tokens.ink.b, 0.5)
                        border.width: isCur ? 1 : 0
                        border.color: Tokens.paper
                    }
                }

                Rectangle {
                    id: hoverRing
                    visible: tzp.hovered !== null && mapMouse.containsMouse
                    width: 16; height: 16; radius: 8
                    color: "transparent"
                    border.width: 1.5
                    border.color: Tokens.ink
                    x: tzp.hovered ? Tz.px(tzp.hovered.lon, mapWrap.width) - 8 : 0
                    y: tzp.hovered ? Tz.py(tzp.hovered.lat, mapWrap.height) - 8 : 0
                    Behavior on x { NumberAnimation { duration: 90 } }
                    Behavior on y { NumberAnimation { duration: 90 } }
                }

                Item {
                    id: pin
                    visible: tzp.selected !== null
                    property real tx: tzp.selected ? Tz.px(tzp.selected.lon, mapWrap.width) : 0
                    property real ty: tzp.selected ? Tz.py(tzp.selected.lat, mapWrap.height) : 0
                    property real drop: 0
                    x: tx
                    y: ty
                    Behavior on tx { NumberAnimation { duration: 180; easing.type: Easing.OutCubic } }
                    Behavior on ty { NumberAnimation { duration: 180; easing.type: Easing.OutCubic } }
                    transform: Translate { y: pin.drop }
                    function bounce() { dropAnim.restart(); }
                    NumberAnimation { id: dropAnim; target: pin; property: "drop"; from: -30; to: 0; duration: 460; easing.type: Easing.OutBounce }
                    Rectangle {
                        width: 6; height: 6; x: -3; y: -8; rotation: 45; color: Tokens.sun
                    }
                    Rectangle {
                        width: 15; height: 15; radius: 8; x: -7.5; y: -22
                        color: Tokens.sun; border.width: 2; border.color: Tokens.paper
                        Rectangle { anchors.centerIn: parent; width: 5; height: 5; radius: 3; color: Tokens.paper }
                    }
                }

                MouseArea {
                    id: mapMouse
                    anchors.fill: parent
                    hoverEnabled: true
                    cursorShape: Qt.CrossCursor
                    onPositionChanged: (m) => {
                        tip.mx = m.x; tip.my = m.y;
                        if (tzp.zones.length === 0) return;
                        var lat = Tz.latAt(m.y, mapWrap.height), lon = Tz.lonAt(m.x, mapWrap.width);
                        tzp.hovered = Tz.nearest(tzp.zones, lat, lon);
                    }
                    onExited: tzp.hovered = null
                    onClicked: (m) => {
                        if (tzp.zones.length === 0) return;
                        var lat = Tz.latAt(m.y, mapWrap.height), lon = Tz.lonAt(m.x, mapWrap.width);
                        tzp.pick(Tz.nearest(tzp.zones, lat, lon));
                    }
                }

                Rectangle {
                    id: tip
                    property real mx: 0
                    property real my: 0
                    visible: tzp.hovered !== null && mapMouse.containsMouse
                    x: Math.max(2, Math.min(mx + 12, mapWrap.width - width - 2))
                    y: Math.max(2, my - height - 8)
                    width: tipT.implicitWidth + 12
                    height: tipT.implicitHeight + 8
                    radius: Tokens.radius
                    color: Tokens.bone
                    Text {
                        id: tipT
                        anchors.centerIn: parent
                        text: tzp.hovered ? Tz.pretty(tzp.hovered.tz) : ""
                        color: Tokens.inkOnBone
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fTiny
                    }
                }
            }
        }

        // search results, floating over the map under the field
        Rectangle {
            visible: tzp.query.length > 0 && tzp.results.length > 0
            anchors { right: search.right; top: search.bottom; topMargin: 3 }
            width: search.width
            height: Math.min(tzp.results.length, 8) * 26 + 6
            radius: Tokens.radius
            color: Tokens.paperLift
            border.width: Tokens.border
            border.color: Tokens.lineStrong
            z: 60
            Column {
                anchors { fill: parent; margins: 3 }
                Repeater {
                    model: tzp.results.slice(0, 8)
                    delegate: Rectangle {
                        id: res
                        required property var modelData
                        width: parent.width
                        height: 26
                        radius: Tokens.radius
                        color: resHov.hovered ? Tokens.tint10 : "transparent"
                        Text {
                            anchors { left: parent.left; right: parent.right; verticalCenter: parent.verticalCenter; leftMargin: Tokens.s2; rightMargin: Tokens.s2 }
                            elide: Text.ElideRight
                            text: res.modelData.tz.replace(/_/g, " ") + (res.modelData.note ? "  \u00b7  " + res.modelData.note : "")
                            color: Tokens.ink
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fTiny
                        }
                        HoverHandler { id: resHov; cursorShape: Qt.PointingHandCursor }
                        TapHandler { onTapped: { tzp.pick(res.modelData); search.clear(); tzp.query = ""; } }
                    }
                }
            }
        }

        Item {
            id: foot
            anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
            height: 60
            Rectangle { height: 1; color: Tokens.lineSoft; anchors { left: parent.left; right: parent.right; top: parent.top } }

            Column {
                anchors { left: parent.left; leftMargin: Tokens.s5; verticalCenter: parent.verticalCenter }
                spacing: 2
                Text {
                    text: tzp.selected ? tzp.selected.tz.replace(/_/g, " ") : I18n.tr("No zone selected")
                    color: Tokens.ink
                    font.family: Tokens.display
                    font.pixelSize: Tokens.fValue
                }
                Text {
                    visible: tzp.localTime !== "" || (tzp.selected && tzp.selected.note)
                    text: {
                        var parts = [];
                        if (tzp.localTime !== "") parts.push(tzp.localTime);
                        if (tzp.selected && tzp.selected.note) parts.push(tzp.selected.note);
                        return parts.join("      ");
                    }
                    color: Tokens.inkMuted
                    font.family: Tokens.mono
                    font.pixelSize: Tokens.fSmall
                }
            }

            Row {
                anchors { right: parent.right; rightMargin: Tokens.s5; verticalCenter: parent.verticalCenter }
                spacing: Tokens.s3
                Btn {
                    text: I18n.tr("CANCEL")
                    onAct: tzp.canceled()
                }
                Btn {
                    text: I18n.tr("SET TIME ZONE")
                    primary: true
                    armed: tzp.changed
                    onAct: { if (tzp.changed) tzp.applied(tzp.selected.tz); }
                }
            }
        }
    }
}

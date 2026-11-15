import QtQuick
import Quickshell
import Quickshell.Io
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"

// the room under the sidebar's tree: the machine now (processor, memory, heat) and an
// activity map of the last weeks, one square a day as a contribution graph draws it, lit by
// how long the machine was on that day (`ryoku-hub activity`, read from the journal). Resting
// on a day reads it out; the map prints in from the left when it first arrives
Item {
    id: root

    readonly property real cell: 9
    readonly property real gap: 2
    readonly property real step: root.cell + root.gap
    readonly property int weeks: Math.max(4, Math.floor((root.width - 16 + root.gap) / root.step))
    readonly property int firstDay: Qt.locale(I18n.lang).firstDayOfWeek % 7

    // { source, days: [{ date, hours }], since }, oldest day first and today last
    property var report: null
    readonly property var days: root.report && root.report.days ? root.report.days : []
    property double loadedAt: 0
    // the day under the pointer, or null
    property var hoveredDay: null

    implicitHeight: figures.height + 8 + months.height + 4 + 7 * root.step + 8 + summary.height

    // the live figures sample only while Settings is focused and this is on show
    readonly property bool sampling: root.visible && Ambient.on
    Binding {
        target: LiveStats
        property: "active"
        value: true
        when: root.sampling
    }

    function load() {
        if (!reader.running)
            reader.running = true;
    }
    Component.onCompleted: root.load()
    // read again when Settings comes back to the front after a while, so today keeps up
    Connections {
        target: Ambient
        function onOnChanged() {
            if (Ambient.on && Date.now() - root.loadedAt > 10 * 60 * 1000)
                root.load();
        }
    }
    Process {
        id: reader
        command: ["ryoku-hub", "activity"]
        stdout: StdioCollector {
            onStreamFinished: {
                var first = root.report === null;
                try {
                    root.report = JSON.parse(text);
                } catch (e) {
                    root.report = { source: "none", days: [] };
                }
                root.loadedAt = Date.now();
                if (first && root.days.length > 0) {
                    if (Lithic.reduceMotion)
                        root.reveal = 1;
                    else
                        printIn.restart();
                }
            }
        }
    }

    property real reveal: 0
    NumberAnimation {
        id: printIn
        target: root
        property: "reveal"
        from: 0
        to: 1
        duration: 900
        easing.type: Easing.OutCubic
    }

    // 0 none, then by hours on: under 3, under 6, under 12, 12 and more
    function level(hours) {
        return hours <= 0 ? 0 : hours < 3 ? 1 : hours < 6 ? 2 : hours < 12 ? 3 : 4;
    }
    function shade(lv) {
        return lv === 0 ? Lithic.alpha(Lithic.fg, Lithic.dark ? 0.07 : 0.09)
            : Lithic.alpha(Lithic.sel, [0, 0.3, 0.5, 0.75, 1][lv]);
    }
    // the day a square stands for: { date, hours, known, future }
    readonly property int todayRow: (new Date().getDay() - root.firstDay + 7) % 7
    function dayAt(col, row) {
        var back = (root.weeks - 1 - col) * 7 + (root.todayRow - row);
        if (back < 0)
            return { future: true };
        var i = root.days.length - 1 - back;
        var d = new Date();
        d.setHours(12, 0, 0, 0);
        d.setDate(d.getDate() - back);
        var key = root.iso(d);
        if (i < 0 || !root.report || (root.report.since && key < root.report.since))
            return { key: key, date: d, hours: 0, known: false, today: back === 0 };
        return { key: key, date: d, hours: root.days[i].hours, known: true, today: back === 0 };
    }
    function iso(d) {
        var two = n => (n < 10 ? "0" : "") + n;
        return d.getFullYear() + "-" + two(d.getMonth() + 1) + "-" + two(d.getDate());
    }

    // where a month begins along the top, kept at least four weeks apart so names never touch
    readonly property var monthMarks: {
        var out = [], last = -9;
        for (var c = 0; c < root.weeks; c++) {
            var d = root.dayAt(c, 0).date;
            var prev = c > 0 ? root.dayAt(c - 1, 0).date : null;
            if (!d || (prev && prev.getMonth() === d.getMonth()) || c - last < 4 || c > root.weeks - 3)
                continue;
            if (c === 0 && root.dayAt(3, 0).date && root.dayAt(3, 0).date.getMonth() !== d.getMonth())
                continue;
            out.push({ col: c, name: Qt.locale(I18n.lang).monthName(d.getMonth(), Locale.ShortFormat).toUpperCase() });
            last = c;
        }
        return out;
    }

    readonly property var totals: {
        var shown = root.days.slice(-root.weeks * 7);
        var on = shown.filter(d => d.hours > 0);
        var streak = 0;
        for (var i = root.days.length - 1; i >= 0 && root.days[i].hours > 0; i--)
            streak++;
        var sum = on.reduce((a, d) => a + d.hours, 0);
        return { on: on.length, avg: on.length ? sum / on.length : 0, streak: streak };
    }

    Label {
        visible: UiPreferences.showJapaneseGloss
        anchors.right: parent.right
        anchors.rightMargin: 8
        anchors.verticalCenter: figures.verticalCenter
        text: "稼働"
        typeRole: "caption"
        font.pixelSize: 11
        font.letterSpacing: 2
        inkRole: "fg3"
    }

    Row {
        id: figures
        x: 8
        spacing: 14

        Repeater {
            model: [
                { name: "CPU", value: LiveStats.cpu + "%" },
                { name: "MEM", value: LiveStats.ram + "%" },
                { name: "TMP", value: LiveStats.cpuTemp > 0 ? LiveStats.cpuTemp + "°" : "--" }
            ]
            delegate: Row {
                required property var modelData
                spacing: 5
                Label {
                    text: modelData.name
                    typeRole: "monoSm"
                    inkRole: "fg3"
                    font.pixelSize: 9
                    font.letterSpacing: 1.2
                }
                Label {
                    text: LiveStats.tick > 0 ? modelData.value : "--"
                    typeRole: "monoSm"
                    inkRole: "fg2"
                    font.pixelSize: 9
                }
            }
        }
    }

    // a month's initial over the week it starts in
    Item {
        id: months
        x: 8
        y: figures.height + 8
        width: root.weeks * root.step
        height: 11
        visible: root.days.length > 0

        Repeater {
            model: root.monthMarks
            delegate: Label {
                required property var modelData
                x: modelData.col * root.step
                text: modelData.name
                typeRole: "monoSm"
                inkRole: "fg3"
                font.pixelSize: 8
                font.letterSpacing: 0.8
            }
        }
    }

    Item {
        id: grid
        x: 8
        y: months.y + months.height + 4
        width: root.weeks * root.step - root.gap
        height: 7 * root.step - root.gap
        visible: root.days.length > 0

        Repeater {
            model: root.days.length > 0 ? root.weeks * 7 : 0
            delegate: Rectangle {
                id: square
                required property int index
                readonly property int col: Math.floor(index / 7)
                readonly property int row: index % 7
                readonly property var day: root.dayAt(col, row)
                readonly property int lv: day.known ? root.level(day.hours) : 0
                // printed in from the left, a column at a time
                readonly property real shown: Math.max(0, Math.min(1, (root.reveal * (root.weeks + 4) - (col + row * 0.3)) / 3))
                visible: !day.future
                x: col * root.step
                y: row * root.step
                width: root.cell
                height: root.cell
                radius: Tone.flat ? 0 : Lithic.cornerRadius(2, root.cell, root.cell)
                color: day.known ? root.shade(lv) : "transparent"
                readonly property bool under: !!root.hoveredDay && root.hoveredDay.key === day.key
                border.width: !day.known || day.today || under ? 1 : 0
                border.color: under ? Lithic.fg : day.today ? Lithic.fg2 : Lithic.alpha(Lithic.fg, 0.08)
                opacity: square.shown
                scale: 0.4 + 0.6 * square.shown
            }
        }

        MouseArea {
            anchors.fill: parent
            hoverEnabled: true
            onPositionChanged: mouse => {
                var c = Math.floor(mouse.x / root.step), r = Math.floor(mouse.y / root.step);
                var d = c >= 0 && c < root.weeks && r >= 0 && r < 7 ? root.dayAt(c, r) : null;
                root.hoveredDay = d && !d.future ? d : null;
            }
            onExited: root.hoveredDay = null
        }
    }
    // the totals, or the day under the pointer
    Label {
        id: summary
        x: 8
        y: grid.y + grid.height + 8
        width: root.width - 16
        visible: root.days.length > 0
        elide: Text.ElideRight
        text: {
            var two = n => (n < 10 ? "0" : "") + n;
            var hd = root.hoveredDay;
            if (hd) {
                var when = Qt.locale(I18n.lang).toString(hd.date, "ddd d MMM");
                if (!hd.known)
                    return when + " · " + I18n.tr("No record");
                return when + " · " + (hd.hours > 0 ? I18n.tr("%1 h on").replace("%1", hd.hours.toFixed(1)) : I18n.tr("Off"));
            }
            var t = root.totals;
            return I18n.tr("%1 days").replace("%1", two(t.on)) + " · "
                + I18n.tr("%1 h/day").replace("%1", t.avg.toFixed(1)) + " · "
                + I18n.tr("Streak %1").replace("%1", two(t.streak));
        }
        typeRole: "monoSm"
        ink: root.hoveredDay ? Lithic.fg : Lithic.fg3
        font.pixelSize: 9
        font.letterSpacing: 0.6
        font.capitalization: Font.AllUppercase
    }
}

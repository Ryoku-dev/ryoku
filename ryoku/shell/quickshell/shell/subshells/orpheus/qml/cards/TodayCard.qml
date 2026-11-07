pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import shell.services as Ryoku
import shell.barkit as Pill
import "../kit" as K
import "../settings" as S

Item {
    id: root

    required property var scene
    property bool shown: false
    property var args: ({})

    readonly property var locale: Qt.locale()
    readonly property int firstDay: locale.firstDayOfWeek % 7
    readonly property date today: clock.date
    readonly property date selectedDate: dateFromKey(selectedKey)
    readonly property var selectedHolidays: selectedKey.length > 0
        ? Ryoku.Holidays.forDate(selectedKey) : []
    readonly property var selectedEvents: selectedKey.length > 0
        ? Ryoku.Events.forDate(selectedKey) : []
    readonly property var weatherHours: Ryoku.Weather.hourly
        ? Ryoku.Weather.hourly.slice(0, 6) : []
    readonly property var weatherDay: Ryoku.Weather.daily && Ryoku.Weather.daily.length > 0
        ? Ryoku.Weather.daily[0] : null
    readonly property bool weatherVisible: Ryoku.Weather.available
        && S.Prefs.voiceWeather === true
    readonly property bool viewingTodayMonth: viewMonth.getFullYear() === today.getFullYear()
        && viewMonth.getMonth() === today.getMonth()
    readonly property real gridWidth: 44 * 7

    property date viewMonth: new Date()
    property string selectedKey: ""
    property string lastTodayKey: ""
    property var monthDays: []
    property bool addingEvent: false

    implicitWidth: sheet.implicitWidth
    implicitHeight: sheet.implicitHeight
    focus: shown

    function monthStart(value) {
        return new Date(value.getFullYear(), value.getMonth(), 1, 12);
    }

    function dateKey(value) {
        return Ryoku.Calendar.dateKey(value.getFullYear(), value.getMonth(), value.getDate());
    }

    function dateFromKey(key) {
        const parts = String(key || "").split("-");
        if (parts.length !== 3)
            return root.today;
        return new Date(Number(parts[0]), Number(parts[1]) - 1, Number(parts[2]), 12);
    }

    function weekdayInitial(index) {
        const day = ((root.firstDay + index + 6) % 7) + 1;
        return root.locale.standaloneDayName(day, Locale.NarrowFormat);
    }

    function rebuildMonth() {
        root.monthDays = Ryoku.Calendar.buildRange(root.today,
            root.viewMonth.getFullYear(), root.viewMonth.getMonth(), 6, root.firstDay);
    }

    function configureHolidays() {
        if (!root.shown || root.monthDays.length === 0)
            return;
        const years = [];
        for (const day of root.monthDays) {
            if (years.indexOf(day.year) < 0)
                years.push(day.year);
        }
        Ryoku.Holidays.configure("", years);
    }

    function initialize() {
        root.viewMonth = root.monthStart(root.today);
        root.selectedKey = root.dateKey(root.today);
        root.lastTodayKey = root.selectedKey;
        root.rebuildMonth();
    }

    function syncToday() {
        const key = root.dateKey(root.today);
        if (key !== root.lastTodayKey) {
            if (root.selectedKey === root.lastTodayKey)
                root.selectedKey = key;
            root.lastTodayKey = key;
            root.rebuildMonth();
        }
        root.configureHolidays();
    }

    function shiftMonth(delta) {
        if (delta === 0)
            return;
        root.viewMonth = new Date(root.viewMonth.getFullYear(),
            root.viewMonth.getMonth() + delta, 1, 12);
        root.rebuildMonth();
        root.configureHolidays();
    }

    function returnToToday() {
        root.selectedKey = root.dateKey(root.today);
        root.viewMonth = root.monthStart(root.today);
        root.rebuildMonth();
        root.configureHolidays();
    }

    function eventTime(event) {
        if (!event || !event.time)
            return "All day";
        return event.endTime ? event.time + " to " + event.endTime : event.time;
    }

    function temperature(value, fallback) {
        if (value !== undefined && value !== null && String(value).length > 0)
            return String(value);
        if (fallback !== undefined && fallback !== null && isFinite(Number(fallback)))
            return String(fallback) + "°";
        return "";
    }

    function symbolName(icon) {
        const name = String(icon || "wx-unknown");
        return name.indexOf("wx-") === 0 ? "weather-" + name.slice(3) : name;
    }

    Keys.onPressed: event => {
        if (event.key === Qt.Key_PageUp) {
            root.shiftMonth(-1);
            event.accepted = true;
        } else if (event.key === Qt.Key_PageDown) {
            root.shiftMonth(1);
            event.accepted = true;
        }
    }

    onShownChanged: {
        if (shown) {
            syncToday();
            Qt.callLater(root.forceActiveFocus);
        } else {
            addingEvent = false;
            eventInput.text = "";
        }
    }
    Component.onCompleted: initialize()

    K.Sheet {
        id: sheet
        pageWidth: 360

        K.Scroller {
            width: parent.width
            height: Math.min(todayContent.implicitHeight,
                root.scene ? Math.max(0, root.scene.cardMaxHeight - 40) : todayContent.implicitHeight)

            Column {
                id: todayContent
                width: parent.width
                spacing: K.Style.s4

                Item {
                    width: root.gridWidth
                    height: Math.max(monthTitle.implicitHeight, monthActions.implicitHeight)
                    anchors.horizontalCenter: parent.horizontalCenter

                    Row {
                        anchors.left: parent.left
                        anchors.right: monthActions.left
                        anchors.rightMargin: K.Style.s2
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: K.Style.s2

                        K.Label {
                            id: monthTitle
                            text: root.locale.toString(root.viewMonth, "MMMM")
                            role: "title"
                        }
                        K.Label {
                            anchors.baseline: monthTitle.baseline
                            text: root.locale.toString(root.viewMonth, "yyyy")
                            role: "title"
                            tone: "muted"
                            numeric: true
                        }
                    }

                    Row {
                        id: monthActions
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: K.Style.s1

                        K.IconButton {
                            icon: "chevron_left"
                            onClicked: root.shiftMonth(-1)
                        }
                        K.Button {
                            visible: !root.viewingTodayMonth
                            text: "Today"
                            onClicked: root.returnToToday()
                        }
                        K.IconButton {
                            icon: "chevron_right"
                            onClicked: root.shiftMonth(1)
                        }
                    }
                }

                Column {
                    width: root.gridWidth
                    anchors.horizontalCenter: parent.horizontalCenter
                    spacing: 0

                    Row {
                        width: parent.width
                        height: 24

                        Repeater {
                            model: 7

                            delegate: Item {
                                required property int index
                                width: 44
                                height: 24

                                K.Label {
                                    anchors.centerIn: parent
                                    text: root.weekdayInitial(parent.index)
                                    role: "micro"
                                    horizontalAlignment: Text.AlignHCenter
                                }
                            }
                        }
                    }

                    Grid {
                        width: parent.width
                        height: {
                            let last = -1;
                            for (let i = 0; i < root.monthDays.length; ++i) {
                                if (root.monthDays[i].inMonth)
                                    last = i;
                            }
                            return (Math.floor(Math.max(0, last) / 7) + 1) * 40;
                        }
                        columns: 7
                        columnSpacing: 0
                        rowSpacing: 0

                        Repeater {
                            model: root.monthDays

                            delegate: Item {
                                id: dayCell

                                required property var modelData
                                readonly property bool selected: root.selectedKey === modelData.key
                                readonly property bool marked: modelData.inMonth
                                    && (Ryoku.Events.hasEvents(modelData.key) || Ryoku.Holidays.hasDate(modelData.key))

                                width: 44
                                height: 40
                                scale: dayPress.pressScale

                                Rectangle {
                                    anchors.horizontalCenter: parent.horizontalCenter
                                    y: 2
                                    width: 32
                                    height: 32
                                    radius: K.Style.radius
                                    visible: dayCell.modelData.inMonth
                                        && (dayCell.modelData.today || dayCell.selected)
                                    color: dayCell.modelData.today ? K.Style.bone : "transparent"
                                    border.width: dayCell.selected ? K.Style.border : 0
                                    border.color: K.Style.lineStrong
                                }

                                K.Label {
                                    visible: dayCell.modelData.inMonth
                                    anchors.horizontalCenter: parent.horizontalCenter
                                    y: 9
                                    width: 32
                                    text: String(dayCell.modelData.day)
                                    role: "body"
                                    tone: dayCell.modelData.today ? "onBone" : "ink"
                                    numeric: true
                                    strong: dayCell.modelData.today
                                    horizontalAlignment: Text.AlignHCenter
                                }

                                Rectangle {
                                    visible: dayCell.marked
                                    anchors.horizontalCenter: parent.horizontalCenter
                                    y: 34
                                    width: 3
                                    height: 3
                                    radius: 1.5
                                    color: K.Style.ink
                                }

                                K.Press {
                                    id: dayPress
                                    enabled: dayCell.modelData.inMonth
                                    radius: K.Style.radius
                                    onClicked: root.selectedKey = dayCell.modelData.key
                                }
                            }
                        }

                        WheelHandler {
                            acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
                            onWheel: event => {
                                const delta = event.angleDelta.y !== 0
                                    ? event.angleDelta.y : event.pixelDelta.y;
                                if (delta !== 0)
                                    root.shiftMonth(delta > 0 ? -1 : 1);
                                event.accepted = true;
                            }
                        }
                    }
                }

                Column {
                    id: agenda
                    width: parent.width
                    spacing: K.Style.s2

                    Repeater {
                        model: root.selectedHolidays

                        delegate: Item {
                            id: holidayRow
                            required property var modelData
                            width: agenda.width
                            height: K.Style.rowH

                            Rectangle {
                                anchors.left: parent.left
                                anchors.right: parent.right
                                anchors.top: parent.top
                                height: K.Style.border
                                color: K.Style.lineSoft
                            }

                            K.Label {
                                anchors.left: parent.left
                                anchors.verticalCenter: parent.verticalCenter
                                width: K.Style.s7 + K.Style.s5
                                text: "All day"
                                role: "caption"
                                tone: "muted"
                                numeric: true
                            }
                            K.Label {
                                anchors.left: parent.left
                                anchors.leftMargin: K.Style.s7 + K.Style.s5
                                anchors.right: parent.right
                                anchors.verticalCenter: parent.verticalCenter
                                text: String(holidayRow.modelData.name || "Holiday")
                                role: "body"
                                elide: Text.ElideRight
                            }
                        }
                    }

                    Repeater {
                        model: root.selectedEvents

                        delegate: Item {
                            id: eventRow
                            required property var modelData
                            width: agenda.width
                            height: K.Style.rowH

                            Rectangle {
                                anchors.left: parent.left
                                anchors.right: parent.right
                                anchors.top: parent.top
                                height: K.Style.border
                                color: K.Style.lineSoft
                            }

                            K.Label {
                                anchors.left: parent.left
                                anchors.verticalCenter: parent.verticalCenter
                                width: K.Style.s7 + K.Style.s5
                                text: root.eventTime(eventRow.modelData)
                                role: "caption"
                                tone: "muted"
                                numeric: true
                            }
                            K.Label {
                                anchors.left: parent.left
                                anchors.leftMargin: K.Style.s7 + K.Style.s5
                                anchors.right: removeEvent.left
                                anchors.rightMargin: K.Style.s2
                                anchors.verticalCenter: parent.verticalCenter
                                text: String(eventRow.modelData.text || "")
                                role: "body"
                                elide: Text.ElideRight
                            }

                            K.IconButton {
                                id: removeEvent
                                anchors.right: parent.right
                                anchors.verticalCenter: parent.verticalCenter
                                size: 28
                                icon: "close"
                                opacity: eventHover.hovered ? 1 : 0
                                enabled: eventHover.hovered
                                onClicked: Ryoku.Events.remove(eventRow.modelData.id)

                                Behavior on opacity {
                                    enabled: !K.Style.still
                                    NumberAnimation { duration: K.Style.effects }
                                }
                            }

                            HoverHandler { id: eventHover }
                        }
                    }

                    Item {
                        visible: !root.addingEvent
                        width: parent.width
                        height: 36

                        K.Label {
                            visible: root.selectedHolidays.length === 0
                                && root.selectedEvents.length === 0
                            anchors.left: parent.left
                            anchors.verticalCenter: parent.verticalCenter
                            text: "Nothing on this day"
                            role: "body"
                            tone: "variant"
                        }

                        K.Button {
                            anchors.right: parent.right
                            text: "Add event"
                            variant: "outline"
                            onClicked: root.addingEvent = true
                        }
                    }

                    K.TextField {
                        id: eventInput
                        visible: root.addingEvent
                        width: parent.width
                        placeholder: "Time and event"
                        onAccepted: raw => {
                            if (Ryoku.Events.addEntry(root.selectedKey, raw)) {
                                eventInput.text = "";
                                root.addingEvent = false;
                            }
                        }
                        onCancelled: {
                            eventInput.text = "";
                            root.addingEvent = false;
                        }
                    }
                }

                Rectangle {
                    id: weatherStrip
                    visible: root.weatherVisible
                    width: parent.width
                    height: 120
                    radius: K.Style.plateRadius
                    color: K.Style.plate
                    border.width: K.Style.border
                    border.color: K.Style.lineSoft
                    scale: weatherPress.pressScale

                    Item {
                        id: weatherSummary
                        anchors.left: parent.left
                        anchors.leftMargin: K.Style.s3
                        anchors.right: parent.right
                        anchors.rightMargin: K.Style.s3
                        anchors.top: parent.top
                        anchors.topMargin: K.Style.s3
                        height: 36

                        Pill.SymbolIcon {
                            id: weatherIcon
                            anchors.left: parent.left
                            anchors.verticalCenter: parent.verticalCenter
                            size: 28
                            name: root.symbolName(Ryoku.Weather.current ? Ryoku.Weather.current.icon : "")
                            color: K.Style.ink
                        }
                        K.Label {
                            id: weatherTemp
                            anchors.left: weatherIcon.right
                            anchors.leftMargin: K.Style.s2
                            anchors.verticalCenter: parent.verticalCenter
                            text: Ryoku.Weather.temp
                            role: "title"
                            numeric: true
                        }
                        K.Label {
                            anchors.left: weatherTemp.right
                            anchors.leftMargin: K.Style.s2
                            anchors.right: weatherHighLow.left
                            anchors.rightMargin: K.Style.s2
                            anchors.verticalCenter: parent.verticalCenter
                            text: Ryoku.Weather.condition
                            role: "body"
                            tone: "variant"
                        }
                        K.Label {
                            id: weatherHighLow
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            text: root.weatherDay
                                ? root.temperature(root.weatherDay.high, root.weatherDay.hi)
                                    + " / " + root.temperature(root.weatherDay.low, root.weatherDay.lo)
                                : ""
                            role: "body"
                            tone: "variant"
                            numeric: true
                        }
                    }

                    Row {
                        id: hourlyRow
                        anchors.left: parent.left
                        anchors.leftMargin: K.Style.s3
                        anchors.right: parent.right
                        anchors.rightMargin: K.Style.s3
                        anchors.bottom: parent.bottom
                        anchors.bottomMargin: K.Style.s3
                        height: 52

                        Repeater {
                            model: root.weatherHours

                            delegate: Item {
                                id: hourCell
                                required property var modelData
                                width: hourlyRow.width / 6
                                height: hourlyRow.height

                                Column {
                                    anchors.centerIn: parent
                                    spacing: K.Style.s1

                                    K.Label {
                                        anchors.horizontalCenter: parent.horizontalCenter
                                        text: String(hourCell.modelData.time
                                            || hourCell.modelData.hour || "").replace(/^0(\d)/, "$1")
                                        role: "micro"
                                        tone: "muted"
                                        numeric: true
                                    }
                                    Pill.SymbolIcon {
                                        anchors.horizontalCenter: parent.horizontalCenter
                                        size: 20
                                        name: root.symbolName(hourCell.modelData.icon)
                                        color: K.Style.inkDim
                                    }
                                    K.Label {
                                        anchors.horizontalCenter: parent.horizontalCenter
                                        text: root.temperature(hourCell.modelData.temperature,
                                            hourCell.modelData.temp)
                                        role: "caption"
                                        numeric: true
                                    }
                                }
                            }
                        }
                    }

                    K.Press {
                        id: weatherPress
                        radius: K.Style.plateRadius
                        onClicked: if (root.scene) root.scene.showCard("weather", {})
                    }
                }
            }
        }
    }

    SystemClock {
        id: clock
        enabled: root.shown
        precision: SystemClock.Minutes
        onDateChanged: root.syncToday()
    }
}

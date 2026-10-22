pragma Singleton

import QtQuick
import Quickshell
import Caelestia.Config
import caelestia.services

Singleton {
    readonly property bool secondsWanted: GlobalConfig.bar.clock.showSeconds || GlobalConfig.dashboard.showClockSeconds

    property alias enabled: clock.enabled
    readonly property date date: clock.date
    readonly property int hours: clock.hours
    readonly property int minutes: clock.minutes
    readonly property int seconds: clock.seconds

    readonly property list<string> timeComponents: format(Units.twelveHourClock ? "hh:mm:A" : "hh:mm").split(":")
    readonly property string hourStr: timeComponents[0] ?? ""
    readonly property string minuteStr: timeComponents[1] ?? ""
    readonly property string amPmStr: timeComponents[2] ?? ""

    function format(fmt: string): string {
        return clock.date.toLocaleString(Qt.locale(), fmt);
    }

    SystemClock {
        id: clock

        precision: secondsWanted ? SystemClock.Seconds : SystemClock.Minutes
    }
}

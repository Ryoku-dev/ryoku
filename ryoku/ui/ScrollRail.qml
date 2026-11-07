import QtQuick
import QtQuick.Controls
import Ryoku.Ui.Singletons

// A square thumb. Qt's pill is the only rounded thing left on a radius-2
// surface, so ryovm drew its own; this is that, shared. Every Flickable,
// ListView and GridView in the apps uses it.
ScrollBar {
    id: rail
    property bool motionEnabled: !Tokens.reduceMotion
    contentItem: Rectangle {
        implicitWidth: 4
        radius: 0
        antialiasing: false
        color: rail.pressed ? Tokens.ink : Tokens.inkFaint
        opacity: rail.policy === ScrollBar.AlwaysOff ? 0 : (rail.active ? 1 : 0.5)
        Behavior on opacity { enabled: rail.motionEnabled; NumberAnimation { duration: Tokens.snap } }
        Behavior on color { enabled: rail.motionEnabled; ColorAnimation { duration: Tokens.snap } }
    }
    background: null
}

import "../" as Services
import QtQuick
import Caelestia as Backend

QtObject {
    readonly property var audio: Services.Audio
    readonly property var players: Services.Players
    readonly property var lyrics: Backend.Lyrics
    readonly property var recorder: Services.Recorder
}

pragma Singleton
import QtQuick
import "../modules/visualizer/Singletons" as VisualizerState

QtObject {
    readonly property bool hasCustomColor: VisualizerState.Config.hasCustomColor
    readonly property color customColor: VisualizerState.Config.customColor
    readonly property real gain: VisualizerState.Config.gain
}

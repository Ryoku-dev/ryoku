pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import inir
import inir.modules.common
import inir.modules.common.widgets.widgetCanvas
import inir.modules.iris.frame
import inir.modules.iris.style
import inir.services

Item {
    id: root
    focus: true
    scale: GlobalStates.overlayOpen ? 1 : 1.03
    Behavior on scale { NumberAnimation { duration: IrisStyle.emergeDuration; easing.type: IrisStyle.feedbackEasing } }

    Rectangle {
        anchors.fill: parent
        color: IrisStyle.scrim
        opacity: GlobalStates.overlayOpen ? 0.72 : 0
        Behavior on opacity { NumberAnimation { duration: IrisStyle.emergeDuration; easing.type: IrisStyle.feedbackEasing } }
    }

    WidgetCanvas {
        anchors.fill: parent
        onClicked: GlobalStates.overlayOpen = false

        OverlayTaskbar {
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.top: parent.top
            anchors.topMargin: IrisFrame.inset("top") + Math.round(14 * IrisStyle.density)
        }

        Repeater {
            model: ScriptModel {
                values: Array.from(Persistent.states.overlay.open ?? []).map(identifier =>
                    OverlayContext.availableWidgets.find(widget => widget.identifier === identifier))
                    .filter(widget => widget !== undefined)
                objectProp: "identifier"
            }
            delegate: OverlayWidgetDelegateChooser {}
        }
    }
}

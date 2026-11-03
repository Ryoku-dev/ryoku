import qs.modules.common
import qs.modules.common.widgets
import qs.modules.waffle.looks
import QtQuick
import QtQuick.Controls

ListView {
    id: root

    ScrollBar.vertical: WScrollBar {}
    
    // Smooth transitions for list changes
    add: Transition {
        NumberAnimation { property: "opacity"; from: 0; to: 1; duration: Looks.transition.enabled ? Looks.transition.duration.normal : 0; easing.type: Easing.OutQuad }
    }
    
    remove: Transition {
        NumberAnimation { property: "opacity"; from: 1; to: 0; duration: Looks.transition.enabled ? Looks.transition.duration.fast : 0; easing.type: Easing.InQuad }
    }
    
    displaced: Transition {
        NumberAnimation { properties: "x,y"; duration: Looks.transition.enabled ? Looks.transition.duration.normal : 0; easing.type: Easing.OutQuad }
    }
    
    // Prevent flicker on model changes
    cacheBuffer: 200
}

import QtQuick
import QtQuick.Controls
import qs.modules.waffle.looks

StackView {
    id: root

    clip: true
    background: null

    // Instant transitions - no animation for better performance and cleaner aesthetic
    pushEnter: Transition {
        NumberAnimation { duration: 0 }
    }

    pushExit: Transition {
        NumberAnimation { duration: 0 }
    }

    popEnter: Transition {
        NumberAnimation { duration: 0 }
    }

    popExit: Transition {
        NumberAnimation { duration: 0 }
    }
}

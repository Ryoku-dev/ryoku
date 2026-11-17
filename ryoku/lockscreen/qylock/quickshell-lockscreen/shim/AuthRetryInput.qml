import QtQuick

Item {
    id: root

    required property var retryTarget
    property var passwordScope: null
    property bool retryAllowed: true
    property bool motionSeen: false
    property point lastMotionPoint: Qt.point(-1, -1)

    function hasNonemptyPassword(item) {
        if (!item)
            return false
        if ((item.echoMode === TextInput.Password ||
                item.echoMode === TextInput.NoEcho ||
                item.echoMode === TextInput.PasswordEchoOnEdit) && item.length > 0)
            return true
        const childItems = item.children || []
        for (let i = 0; i < childItems.length; ++i) {
            if (hasNonemptyPassword(childItems[i]))
                return true
        }
        return false
    }

    function requestRetry() {
        if (!retryAllowed || hasNonemptyPassword(passwordScope))
            return false
        return retryTarget && retryTarget.requestAuthRetry() === true
    }

    function noteMotion(x, y) {
        if (!retryAllowed) {
            cancelPending()
            return
        }
        const point = Qt.point(x, y)
        if (!motionSeen) {
            motionSeen = true
            lastMotionPoint = point
            return
        }
        if (lastMotionPoint.x === point.x && lastMotionPoint.y === point.y)
            return
        lastMotionPoint = point
        motionSettle.restart()
    }

    function cancelPending() {
        motionSettle.stop()
    }

    function isPointOverControl(item, x, y) {
        if (!item || item.visible === false || item.enabled === false)
            return false
        const childItems = item.children || []
        for (let i = childItems.length - 1; i >= 0; --i) {
            if (isPointOverControl(childItems[i], x, y))
                return true
        }
        if (item === passwordScope || typeof item.mapFromItem !== "function" ||
                typeof item.contains !== "function")
            return false
        const localPoint = item.mapFromItem(root, x, y)
        if (!item.contains(localPoint))
            return false
        if (typeof item.echoMode !== "undefined")
            return true
        if (typeof item.acceptedButtons === "undefined" || item.acceptedButtons === Qt.NoButton)
            return false
        return !(item.z < 0 && passwordScope &&
            item.width >= passwordScope.width && item.height >= passwordScope.height)
    }

    onRetryAllowedChanged: { if (!retryAllowed) cancelPending() }

    Keys.priority: Keys.AfterItem
    Keys.onReturnPressed: (event) => { event.accepted = requestRetry() }
    Keys.onEnterPressed: (event) => { event.accepted = requestRetry() }

    Timer {
        id: motionSettle
        interval: 400
        repeat: false
        onTriggered: root.requestRetry()
    }

    HoverHandler {
        onPointChanged: {
            if (!root.retryAllowed) {
                root.cancelPending()
                return
            }
            const position = point.position
            if (root.isPointOverControl(root.passwordScope, position.x, position.y))
                root.cancelPending()
            else
                root.noteMotion(position.x, position.y)
        }
    }

    MouseArea {
        anchors.fill: parent
        z: 2147483646
        acceptedButtons: Qt.LeftButton
        propagateComposedEvents: true
        onClicked: (mouse) => {
            if (!root.isPointOverControl(root.passwordScope, mouse.x, mouse.y))
                root.requestRetry()
            mouse.accepted = false
        }
    }
}

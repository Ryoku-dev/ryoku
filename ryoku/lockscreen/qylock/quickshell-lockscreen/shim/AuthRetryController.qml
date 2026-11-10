import QtQuick

Item {
    id: root
    visible: false
    width: 0
    height: 0

    required property var conversation
    required property var restartConversation
    property bool armed: false
    property bool foreground: false
    property bool unlocking: false
    property bool passwordSubmitted: false
    property bool keyInputOwned: false
    property alias coolingDown: cooldown.running

    function cancel() {
        cooldown.stop()
    }

    function request() {
        if (!enabled || !armed || !foreground || unlocking || passwordSubmitted ||
                keyInputOwned || !conversation.active ||
                !conversation.responseRequired ||
                conversation.pendingPassword !== "" || cooldown.running)
            return false

        conversation.abort()
        if (!enabled || !armed || !foreground || unlocking || keyInputOwned)
            return false

        if (restartConversation() !== true)
            return false
        cooldown.restart()
        return true
    }

    onEnabledChanged: { if (!enabled) cancel() }
    onArmedChanged: { if (!armed) cancel() }
    onForegroundChanged: { if (!foreground) cancel() }
    onUnlockingChanged: { if (unlocking) cancel() }
    onKeyInputOwnedChanged: { if (keyInputOwned) cancel() }

    Timer {
        id: cooldown
        interval: 1500
        repeat: false
    }
}

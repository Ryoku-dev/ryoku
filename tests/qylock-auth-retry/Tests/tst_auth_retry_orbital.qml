import QtQuick
import QtTest
import "../../../ryoku/lockscreen/qylock/quickshell-lockscreen/shim" as Lock

TestCase {
    id: testCase
    name: "QylockAuthRetryOrbital"
    when: windowShown
    visible: true
    width: 1280
    height: 800

    property int retries: 0
    property int logins: 0
    property var config: ({ themeMode: "dark", enableWindup: "true", clock24h: "true" })
    property var keyboard: QtObject { property bool numLock: false; property bool capsLock: false }
    property var userModel: ListModel {
        property string lastUser: "user"
        property int lastIndex: 0
        function rowCount() { return count }
        Component.onCompleted: append({ name: "user", realName: "User", icon: "", homeDir: "/home/user" })
    }
    property var sessionModel: ListModel {
        property int lastIndex: 0
        property string desktopName: ""
        function rowCount() { return count }
        Component.onCompleted: append({ name: "session", file: "session.desktop" })
    }
    property var sddm: QtObject {
        property string hostName: "box"
        property bool keyPrompts: false
        readonly property bool fingerprintHint: true
        property bool fingerprintReady: false
        property string fingerprintState: "idle"
        property bool fingerprintUnlock: false
        readonly property bool keyReady: false
        readonly property string keyState: "off"
        readonly property bool keyPasswordMode: false
        signal loginFailed()
        signal loginSucceeded()
        signal keyTimedOut()
        signal surfaceRevealed()
        function login(user, password, session) { testCase.logins += 1 }
        function loginKeyPin(user, pin) {}
        function usePassword() {}
        function useKey() {}
    }
    QtObject {
        id: target
        function requestAuthRetry() { testCase.retries += 1; return true }
    }

    Lock.AuthRetryInput {
        id: input
        parent: testCase
        width: 1280
        height: 800
        retryTarget: target
        passwordScope: theme.item
        Loader {
            id: theme
            anchors.fill: parent
            focus: true
            source: Qt.resolvedUrl("../../../ryoku/lockscreen/qylock/themes/clockwork/orbital/Main.qml")
        }
    }

    function init() {
        retries = 0
        logins = 0
    }

    function findPassword(item) {
        if (item.echoMode === TextInput.Password)
            return item
        for (let i = 0; i < item.children.length; ++i) {
            const found = findPassword(item.children[i])
            if (found)
                return found
        }
        return null
    }

    function test_shipped_skin_empty_return_retries_and_typed_return_does_not() {
        tryVerify(() => theme.status === Loader.Ready)
        const password = findPassword(theme.item)
        verify(password)
        password.forceActiveFocus()
        keyClick(Qt.Key_Return)
        compare(retries, 1)
        compare(logins, 0)
        keyClick(Qt.Key_S)
        keyClick(Qt.Key_Return)
        compare(retries, 1)
    }

    function test_shipped_skin_background_click_retries_and_field_click_does_not() {
        tryVerify(() => theme.status === Loader.Ready)
        const password = findPassword(theme.item)
        verify(password)
        const point = password.mapToItem(input, password.width / 2, password.height / 2)
        mouseClick(input, point.x, point.y)
        compare(retries, 0)
        verify(password.activeFocus)
        mouseClick(input, 40, 400)
        compare(retries, 1)
    }
}

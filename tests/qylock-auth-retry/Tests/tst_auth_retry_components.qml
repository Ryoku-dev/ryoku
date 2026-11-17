import QtQuick
import QtTest
import "../../../ryoku/lockscreen/qylock/quickshell-lockscreen/shim" as Lock

TestCase {
    id: testCase
    name: "QylockAuthRetryComponents"
    when: windowShown
    visible: true
    width: 640
    height: 480

    property var input
    property var controller
    property var target
    property var conversation
    property int controlClicks: 0
    property string lifecycle: ""

    component RetryTarget: QtObject {
        property int calls: 0
        function requestAuthRetry() { calls += 1; return true }
    }
    component Conversation: QtObject {
        property bool active: true
        property bool responseRequired: true
        property string pendingPassword: ""
        property var onAbort: null
        function abort() {
            lifecycle += "abort;"
            active = false
            if (onAbort)
                onAbort()
        }
    }
    component InputFixture: Lock.AuthRetryInput {
        width: 640
        height: 480
        retryTarget: target
        passwordScope: content
        property bool keyInputOwned: false
        retryAllowed: !keyInputOwned
        Item {
            id: content
            anchors.fill: parent
            FocusScope {
                id: controlFocus
                x: 480; y: 20; width: 140; height: 80
                MouseArea {
                    objectName: "control"
                    anchors.fill: parent
                    onClicked: { controlClicks += 1; controlFocus.forceActiveFocus() }
                }
            }
            TextInput {
                id: password
                objectName: "password"
                x: 200; y: 300; width: 240; height: 40
                echoMode: TextInput.NoEcho
                MouseArea { anchors.fill: parent; onClicked: password.forceActiveFocus() }
            }
        }
    }
    component ControllerFixture: Lock.AuthRetryController {
        conversation: testCase.conversation
        enabled: true
        armed: true
        foreground: true
        restartConversation: function() {
            verify(!conversation.active)
            lifecycle += "start;"
            conversation.active = true
            conversation.responseRequired = true
            return true
        }
    }

    Component { id: targetComponent; RetryTarget {} }
    Component { id: conversationComponent; Conversation {} }
    Component { id: inputComponent; InputFixture {} }
    Component { id: controllerComponent; ControllerFixture {} }
    Component { id: configComponent; Lock.AuthRetryConfig { path: "/unused" } }

    function findObject(item, name) {
        if (item.objectName === name) return item
        for (let i = 0; i < item.children.length; ++i) {
            const found = findObject(item.children[i], name)
            if (found) return found
        }
        return null
    }
    function init() {
        target = createTemporaryObject(targetComponent, testCase)
        conversation = createTemporaryObject(conversationComponent, testCase)
        input = createTemporaryObject(inputComponent, testCase)
        controller = createTemporaryObject(controllerComponent, testCase)
        verify(target && conversation && input && controller)
        input.visible = true
        input.forceActiveFocus()
        wait(0)
    }
    function cleanup() {
        controller.destroy(); input.destroy(); conversation.destroy(); target.destroy()
        controller = null; input = null; conversation = null; target = null
        controlClicks = 0; lifecycle = ""
    }

    function setRetryEligibility(value) {
        if ("retryAllowed" in input)
            input.retryAllowed = value
        else
            input.enabled = value
        wait(0)
    }

    function test_default_off_keeps_descendant_password_and_control_enabled() {
        setRetryEligibility(false)
        const password = findObject(input, "password")
        const control = findObject(input, "control")
        verify(input.enabled)
        verify(password.enabled)
        mouseClick(password, 20, 20, Qt.LeftButton)
        keyClick(Qt.Key_S)
        compare(password.length, 1)
        mouseClick(control, control.width / 2, control.height / 2, Qt.LeftButton)
        compare(controlClicks, 1)
        compare(target.calls, 0)
    }

    function test_key_ownership_cancels_pending_motion_without_disabling_controls() {
        mouseMove(input, 100, 200)
        mouseMove(input, 180, 200)
        wait(100)
        input.keyInputOwned = true
        wait(0)
        const control = findObject(input, "control")
        verify(input.enabled)
        mouseClick(control, control.width / 2, control.height / 2, Qt.LeftButton)
        wait(400)
        compare(controlClicks, 1)
        compare(target.calls, 0)
    }

    function test_return_and_keypad_enter_request_retry() {
        keyClick(Qt.Key_Return)
        keyClick(Qt.Key_Enter)
        compare(target.calls, 2)
    }
    function test_click_does_not_steal_control_or_right_click() {
        mouseClick(input, 80, 80, Qt.LeftButton)
        mouseClick(input, 90, 80, Qt.RightButton)
        const control = findObject(input, "control")
        mouseClick(control, control.width / 2, control.height / 2, Qt.LeftButton)
        compare(target.calls, 1)
        compare(controlClicks, 1)
    }
    function test_noecho_password_wins_after_focus_transfer() {
        const password = findObject(input, "password")
        const control = findObject(input, "control")
        mouseClick(password, 20, 20, Qt.LeftButton)
        keyClick(Qt.Key_S)
        keyClick(Qt.Key_E)
        keyClick(Qt.Key_C)
        keyClick(Qt.Key_R)
        keyClick(Qt.Key_E)
        keyClick(Qt.Key_T)
        compare(password.length, 6)
        mouseClick(control, 20, 20, Qt.LeftButton)
        verify(!password.activeFocus)
        mouseClick(input, 80, 80, Qt.LeftButton)
        keyClick(Qt.Key_Return)
        compare(target.calls, 0)
        compare(password.length, 6)
    }
    function test_motion_ignores_entry_and_settles_once() {
        mouseMove(input, 100, 200)
        wait(450)
        compare(target.calls, 0)
        mouseMove(input, 140, 200)
        wait(100)
        mouseMove(input, 220, 200)
        wait(450)
        compare(target.calls, 1)
    }
    function test_controller_aborts_before_one_restart_and_cools_down() {
        compare(controller.request(), true)
        compare(lifecycle, "abort;start;")
        compare(controller.request(), false)
        compare(lifecycle, "abort;start;")
    }
    function test_controller_guards_password_recognition_proof_foreground_and_key() {
        conversation.pendingPassword = "occupied"
        compare(controller.request(), false)
        conversation.pendingPassword = ""
        conversation.responseRequired = false
        compare(controller.request(), false)
        conversation.responseRequired = true
        controller.passwordSubmitted = true
        compare(controller.request(), false)
        controller.passwordSubmitted = false
        controller.foreground = false
        compare(controller.request(), false)
        controller.foreground = true
        controller.armed = false
        compare(controller.request(), false)
        controller.armed = true
        controller.keyInputOwned = true
        compare(controller.request(), false)
        controller.keyInputOwned = false
        controller.unlocking = true
        compare(controller.request(), false)
        compare(lifecycle, "")
    }
    function test_key_activation_cancels_pending_cooldown() {
        compare(controller.request(), true)
        verify(controller.coolingDown)
        controller.keyInputOwned = true
        verify(!controller.coolingDown)
        compare(controller.request(), false)
    }
    function test_key_activation_during_abort_prevents_restart() {
        conversation.onAbort = function() { controller.keyInputOwned = true }
        compare(controller.request(), false)
        compare(lifecycle, "abort;")
        verify(!conversation.active)
    }
    function test_flag_accepts_only_readable_exact_on() {
        const config = createTemporaryObject(configComponent, testCase)
        verify(config !== null)
        config.apply("on\n", true)
        verify(config.enabled)
        config.apply("true\n", true)
        verify(!config.enabled)
        config.apply("on\n", false)
        verify(!config.enabled)
        config.apply("", false)
        verify(!config.enabled)
    }
}

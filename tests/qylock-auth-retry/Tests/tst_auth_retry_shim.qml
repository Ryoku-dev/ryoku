import QtQuick
import QtTest
import "../../../ryoku/lockscreen/qylock/quickshell-lockscreen/shim" as Lock
import Quickshell.Services.Pam
import Quickshell

TestCase {
    name: "QylockAuthRetryShim"
    when: windowShown
    visible: true
    width: 320
    height: 240

    function cleanup() {
        Quickshell.faceLock = ""
    }

    function test_facelock_uses_fixed_pam_services_data() {
        return [
            {tag: "default", value: "", service: "ryoku-lock-pw"},
            {tag: "opt-in", value: "on", service: "ryoku-lock-pw-facelock"},
            {tag: "arbitrary-service", value: "other", service: "ryoku-lock-pw"}
        ]
    }

    function test_facelock_uses_fixed_pam_services(data) {
        Quickshell.faceLock = data.value
        const shim = createTemporaryObject(shimComponent, this)
        verify(shim !== null)
        const password = PamRegistry.find(data.service)
        verify(password !== null)
        shim.armWhenReady = true
        shim.loginSessionActive = true
        password.active = true
        password.responseRequired = true
        shim.sddm.login("test-user", "test-password", 0)
        compare(password.lastResponse, "test-password")
    }

    function waitingPassword(shim) {
        shim.armWhenReady = true
        shim.loginSessionActive = true
        shim.fpHasFingers = true
        const password = PamRegistry.find("ryoku-lock-pw")
        verify(password !== null)
        password.active = true
        password.responseRequired = true
        return password
    }

    function test_default_off_preserves_empty_submission() {
        const shim = createTemporaryObject(shimComponent, this)
        verify(shim !== null)
        shim.authRetryEnabled = false
        const password = waitingPassword(shim)

        shim.sddm.login("test-user", "", 0)

        compare(password.abortCount, 0)
        compare(password.respondCount, 1)
        compare(password.lastResponse, "")
    }


    function test_opt_in_empty_submit_restarts_waiting_password_conversation() {
        const shim = createTemporaryObject(shimComponent, this)
        verify(shim !== null)
        shim.authRetryEnabled = true
        const password = waitingPassword(shim)

        shim.sddm.login("test-user", "", 0)

        compare(password.abortCount, 1)
        compare(password.startCount, 1)
        compare(password.respondCount, 0)
    }

    function test_nonempty_password_wins_when_retry_enabled() {
        const shim = createTemporaryObject(shimComponent, this)
        verify(shim !== null)
        shim.authRetryEnabled = true
        const password = waitingPassword(shim)

        shim.sddm.login("test-user", "secret", 0)

        compare(password.abortCount, 0)
        compare(password.respondCount, 1)
        compare(password.lastResponse, "secret")
    }

    function test_security_key_ownership_inhibits_retry_without_consuming_pin() {
        const shim = createTemporaryObject(shimComponent, this)
        verify(shim !== null)
        shim.authRetryEnabled = true
        shim.keyEnabled = true
        shim.keyState = "pin"
        const password = waitingPassword(shim)
        const key = PamRegistry.find("ryoku-lock-key")
        verify(key !== null)
        key.active = true
        key.responseRequired = true

        shim.sddm.login("test-user", "", 0)

        compare(password.abortCount, 0)
        compare(password.respondCount, 0)
        compare(key.respondCount, 0)
        compare(key.abortCount, 0)
    }

    Component { id: shimComponent; Lock.SddmShim {} }
}

pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import Quickshell.Services.UPower
import inir.modules.common
import inir.modules.common.functions
import inir.services

Singleton {
    id: root

    function _log(...args): void {
        if (Quickshell.env("QS_DEBUG") === "1") console.log(...args);
    }

    // `inir battery simulate "14 charging"` stands in for UPower until `off` or a restart, so every family's battery
    // surface can be seen on a machine without one. It never suspends the real machine.
    property real simulatedPercentage: -1 // 0..1 while simulating
    property string simulatedState: "discharging" // charging, discharging, full
    readonly property bool simulating: simulatedPercentage >= 0

    property bool available: simulating || UPower.displayDevice.isLaptopBattery
    property var chargeState: !simulating ? UPower.displayDevice.state
        : simulatedState === "charging" ? UPowerDeviceState.Charging
        : simulatedState === "full" ? UPowerDeviceState.FullyCharged
        : UPowerDeviceState.Discharging
    property bool isCharging: chargeState == UPowerDeviceState.Charging
    property bool isPluggedIn: isCharging || chargeState == UPowerDeviceState.PendingCharge
    // Discharging-based, not !isPluggedIn: FullyCharged on AC must not count as "on battery"
    readonly property bool onBattery: available && (chargeState == UPowerDeviceState.Discharging || chargeState == UPowerDeviceState.PendingDischarge)
    property real percentage: simulating ? simulatedPercentage : (UPower.displayDevice?.percentage ?? 1)
    readonly property bool allowAutomaticSuspend: Config.options?.battery?.automaticSuspend ?? false
    readonly property bool soundEnabled: Config.options?.sounds?.battery ?? true

    property bool isLow: available && percentage <= 0.20
    property bool isCritical: available && percentage <= 0.05
    property bool isSuspending: available && (percentage <= ((Config.options?.battery?.suspend ?? 5) / 100))
    property bool isFull: available && (percentage >= ((Config.options?.battery?.full ?? 95) / 100))

    property bool isLowAndNotCharging: isLow && !isCharging
    property bool isCriticalAndNotCharging: isCritical && !isCharging
    property bool isSuspendingAndNotCharging: !simulating && allowAutomaticSuspend && isSuspending && !isCharging
    property bool isFullAndCharging: isFull && isCharging

    // A simulated pack: 12 W over five hours, refilled at 30 W in two.
    property real energyRate: !simulating ? UPower.displayDevice.changeRate : isCharging ? 30 : onBattery ? 12 : 0
    property real timeToEmpty: !simulating ? UPower.displayDevice.timeToEmpty : onBattery ? Math.round(percentage * 18000) : 0
    property real timeToFull: !simulating ? UPower.displayDevice.timeToFull : isCharging ? Math.round((1 - percentage) * 7200) : 0

    IpcHandler {
        target: "battery"

        function status(): string {
            if (!root.available)
                return "no battery"
            const state = root.isCharging ? "charging" : root.onBattery ? "discharging" : "plugged in"
            return `${Math.round(root.percentage * 100)}% ${state}${root.simulating ? " (simulated)" : ""}`
        }

        // "14", "14 charging", "full" or "off"; a level alone is discharging.
        function simulate(spec: string): string {
            const words = String(spec ?? "").trim().toLowerCase().split(/[\s:]+/).filter(w => w.length > 0)
            if (words.length === 0 || words[0] === "off") {
                root.simulatedPercentage = -1
                return status()
            }
            const level = words.find(w => /^\d+(\.\d+)?%?$/.test(w))
            const state = words.find(w => w === "charging" || w === "discharging" || w === "full") ?? "discharging"
            // The level first: a notice raised by the state change reads the new level.
            root.simulatedPercentage = state === "full" && level === undefined ? 1
                : Math.max(0, Math.min(100, parseFloat(level ?? "50"))) / 100
            root.simulatedState = state
            return status()
        }
    }

    // ─── Charge limit ───
    readonly property bool chargeLimitEnabled: Config.options?.battery?.chargeLimit?.enable ?? false
    readonly property int chargeLimitThreshold: Config.options?.battery?.chargeLimit?.threshold ?? 80
    property var _chargeLimitCapability: null
    property int _pendingChargeLimitAction: -1 // -1 none, 0 disable, 1 enable
    property bool _initialChargeLimitRead: true
    readonly property bool chargeLimitSupported: _chargeLimitCapability !== null
    readonly property bool chargeLimitAdjustable: chargeLimitSupported
        && (_chargeLimitCapability.options ?? []).length === 0
    readonly property int currentChargeLimit: chargeLimitSupported
        ? Number(_chargeLimitCapability.current ?? 100) : -1
    readonly property bool chargeLimitActive: currentChargeLimit > 0 && currentChargeLimit < 100

    Component.onCompleted: {
        if (root.available)
            root._readChargeLimit()
    }

    onAvailableChanged: {
        if (available) {
            root._readChargeLimit()
        } else {
            root._chargeLimitCapability = null
        }
    }

    function _readChargeLimit(): void {
        if (!chargeLimitReader.running)
            chargeLimitReader.running = true
    }

    Process {
        id: chargeLimitReader
        command: ["ryoku-host", "power", "capabilities", "--json"]
        stdout: StdioCollector { id: chargeLimitOutput }
        onExited: (exitCode) => {
            if (exitCode !== 0)
                return
            try {
                const report = JSON.parse(chargeLimitOutput.text)
                root._chargeLimitCapability = report?.battery?.chargeLimit ?? null
                const initialRead = root._initialChargeLimitRead
                root._initialChargeLimitRead = false
                const pendingAction = root._pendingChargeLimitAction
                root._pendingChargeLimitAction = -1
                if (pendingAction === 1 || (initialRead && root.chargeLimitEnabled))
                    Qt.callLater(root._applyChargeLimit)
                else if (pendingAction === 0)
                    Qt.callLater(root._resetChargeLimit)
            } catch (e) {
                console.warn("[Battery] Failed to read charge-limit capability: " + e)
            }
        }
    }

    Timer {
        interval: 30000
        repeat: true
        running: root.chargeLimitSupported
        onTriggered: root._readChargeLimit()
    }

    function _applyChargeLimit(): void {
        if (!chargeLimitSupported)
            return
        if (chargeLimitWriter.running) {
            _pendingChargeLimitAction = 1
            return
        }
        chargeLimitWriter.command = [
            "ryoku-host", "power", "charge-limit", "set", String(chargeLimitThreshold)
        ]
        chargeLimitWriter.running = true
    }

    function _resetChargeLimit(): void {
        if (!chargeLimitSupported)
            return
        if (chargeLimitWriter.running) {
            _pendingChargeLimitAction = 0
            return
        }
        chargeLimitWriter.command = ["ryoku-host", "power", "charge-limit", "clear"]
        chargeLimitWriter.running = true
    }

    Process {
        id: chargeLimitWriter
        onExited: (exitCode) => {
            if (exitCode === 0) {
                root._readChargeLimit()
            } else {
                root._pendingChargeLimitAction = -1
                console.warn("[Battery] Failed to update charge limit (exit code " + exitCode + ")")
            }
        }
    }

    onChargeLimitEnabledChanged: {
        if (!chargeLimitSupported)
            return
        if (chargeLimitEnabled)
            _applyChargeLimit()
        else
            _resetChargeLimit()
    }

    onChargeLimitThresholdChanged: {
        if (chargeLimitSupported && chargeLimitEnabled && chargeLimitAdjustable)
            _applyChargeLimit()
    }

    // ─── Battery warnings ───
    onIsLowAndNotChargingChanged: {
        if (!root.available || !isLowAndNotCharging) return;

        if (root.soundEnabled) Audio.playEvent("batteryLow");
    }

    onIsCriticalAndNotChargingChanged: {
        if (!root.available || !isCriticalAndNotCharging) return;

        if (root.soundEnabled) Audio.playEvent("batteryCritical");
    }

    onIsSuspendingAndNotChargingChanged: {
        if (root.available && isSuspendingAndNotCharging) {
            Session.suspend()
        }
    }

    onIsFullAndChargingChanged: {
        if (!root.available || !isFullAndCharging) return;
        Quickshell.execDetached([
            "/usr/bin/notify-send",
            Translation.tr("Battery full"),
            Translation.tr("Please unplug the charger"),
            "-a", "Shell",
            "--hint=int:transient:1",
        ]);

        if (root.soundEnabled) Audio.playEvent("batteryFull");
    }

    onIsPluggedInChanged: {
        if (!root.available || !root.soundEnabled) return;
        if (isPluggedIn) {
            Audio.playEvent("powerPlug")
        } else {
            Audio.playEvent("powerUnplug")
        }
    }
}

pragma Singleton

import stage.services
import stage.modules.common
import Quickshell
import Quickshell.Services.UPower
import QtQuick
import Quickshell.Io

Singleton {
    id: root

    readonly property var displayDevice: UPower.displayDevice

    // Presence, health and the charge-limit sysfs path stay on the physical
    // battery pick (first present pack), which keeps its laptop-battery
    // metadata where the synthetic display device does not.
    readonly property var physicalDevice: {
        const devices = UPower.devices?.values ?? [];
        for (let i = 0; i < devices.length; ++i) {
            const candidate = devices[i];
            if (candidate?.isLaptopBattery && candidate?.isPresent)
                return candidate;
        }
        return root.displayDevice;
    }

    // The display device is synthetic on some UPower versions and can lose
    // laptop-battery metadata while the real pack remains present.
    // Level and charge state follow the aggregate, not one pack: on a
    // two-battery machine the first pack can sit parked by its controller
    // ("Not charging" at 99 %) for hours while the other carries the load,
    // and binding to it froze the readout and the low-battery warnings.
    // An unusable aggregate falls back to the active pack, then to the
    // physical pick.
    readonly property var device: {
        const agg = root.displayDevice;
        if (agg && agg.ready !== false
                && agg.state !== UPowerDeviceState.Unknown)
            return agg;
        let fallback = null;
        const devices = UPower.devices?.values ?? [];
        for (let i = 0; i < devices.length; ++i) {
            const candidate = devices[i];
            if (!candidate?.isLaptopBattery || !candidate?.isPresent) continue;
            if (candidate.state === UPowerDeviceState.Discharging
                    || candidate.state === UPowerDeviceState.Charging)
                return candidate;
            if (fallback === null) fallback = candidate;
        }
        return fallback ?? root.physicalDevice;
    }

    // A battery that has not reported yet lies: 0 %, state Unknown. Anything that acts on the level
    // (warnings, automatic suspend) waits for this instead of trusting cold-start values.
    readonly property bool deviceReady: (physicalDevice?.ready ?? false)
        && (physicalDevice?.isPresent ?? false) && (device?.ready ?? false)

    property bool available: (physicalDevice?.isLaptopBattery ?? false)
        && (physicalDevice?.isPresent ?? false)
    property var chargeState: device?.state ?? UPowerDeviceState.Unknown
    property bool isCharging: chargeState == UPowerDeviceState.Charging
    readonly property bool isFullyCharged: chargeState == UPowerDeviceState.FullyCharged

    // The AC line, not the battery state, is what says whether a charger is attached: a pack held at
    // its charge limit reports Discharging or PendingCharge at ~0 W with the charger plugged in.
    readonly property bool onAc: !UPower.onBattery
    // Charge states describe the battery, not the AC line: a parked or stale
    // pack can read FullyCharged or PendingCharge for hours while the machine
    // runs off another battery, and both would read as "plugged in". Only the
    // AC line answers "is a charger attached".
    property bool isPluggedIn: onAc

    // UPower exposes a floating estimate that jitters below one displayed
    // percent. Quantize at the shared service so battery faces do not restart
    // animations or invalidate the desktop for an invisible fractional change.
    readonly property real percentage: Math.round(Math.max(0, Math.min(1,
        device?.percentage ?? 1)) * 100) / 100
    readonly property int percent: Math.round(percentage * 100)

    property real energyRate: device?.changeRate ?? 0
    property real timeToEmpty: device?.timeToEmpty ?? 0
    property real timeToFull: device?.timeToFull ?? 0

    readonly property bool allowAutomaticSuspend: Config.options.battery.automaticSuspend
    // How far the level must recover before the same warning is allowed to fire again
    readonly property int rearmMargin: 3

    property bool isLow: available && (percentage <= Config.options.battery.low / 100)
    property bool isCritical: available && (percentage <= Config.options.battery.critical / 100)
    property bool isSuspending: available && (percentage <= Config.options.battery.suspend / 100)
    property bool isFull: available && (percentage >= Config.options.battery.full / 100)

    property bool isLowAndUnplugged: isLow && !isPluggedIn
    property bool isCriticalAndUnplugged: isCritical && !isPluggedIn

    // Plugged in and still losing charge: the supply cannot keep up with what the machine is drawing.
    readonly property bool drainingOnAc: available && onAc && !isCharging && !chargeLimitReached
        && chargeState == UPowerDeviceState.Discharging && energyRate > 0.5

    // As charged as this pack is going to get right now, whether that is 100 % or a firmware limit.
    readonly property bool atChargeCeiling: available
        && (chargeLimitReached || (isPluggedIn && (isFullyCharged || percent >= 100)))

    property real health: {
        if (!root.available || !(physicalDevice?.healthSupported ?? false))
            return 0;
        const value = Number(physicalDevice.healthPercentage) || 0;
        if (value === 0)
            return 0.01;
        return value < 1 ? value * 100 : value;
    }

    property string batteryNativePath: root.available ? String(physicalDevice?.nativePath ?? "") : ""

    property int cycles: -1

    // ── Charge limit ────────────────────────────────────────────────────────────────────────────
    // Standard kernel ABI first, then known vendor-specific locations (TLP-style). UPower exposes
    // charge thresholds of its own but caches them past the point of being wrong, so sysfs only.
    readonly property var chargeLimitCandidates: {
        const paths = [];
        if (!available) return paths;
        if (batteryNativePath) {
            const base = `/sys/class/power_supply/${batteryNativePath}`;
            paths.push({ path: `${base}/charge_control_end_threshold`, type: "plain",
                start: `${base}/charge_control_start_threshold` });
            paths.push({ path: `${base}/charge_stop_threshold`, type: "plain",
                start: `${base}/charge_start_threshold` });
            paths.push({ path: `/sys/devices/platform/smapi/${batteryNativePath}/stop_charge_thresh`,
                type: "plain",
                start: `/sys/devices/platform/smapi/${batteryNativePath}/start_charge_thresh` });
        }
        paths.push({ path: "/sys/devices/platform/huawei-wmi/charge_control_thresholds", type: "last",
            startInline: true });
        paths.push({ path: "/sys/devices/platform/lg-laptop/battery_care_limit", type: "plain" });
        paths.push({ path: "/sys/devices/platform/sony-laptop/battery_care_limiter", type: "plain" });
        paths.push({ path: "/sys/devices/platform/samsung/battery_life_extender", type: "bool80" });
        return paths;
    }
    property int chargeLimitCandidateIndex: 0
    property bool chargeLimitResolved: false // a candidate answered; stop walking the list on refresh
    property int chargeLimit: 100 // 0 or 100 = no limit
    property int chargeLimitStart: -1 // firmware recharge threshold, -1 when it exposes none
    property bool chargeLimitStartMissing: false // stop re-reading a node that is not there
    readonly property bool chargeLimitActive: chargeLimitIsActive()

    // Imperative code reads these instead of the properties above: a handler can run before the
    // bindings that derive from the value it was woken by, and a stale read here silently unlatches.
    function chargeLimitIsActive(): bool {
        return root.available && root.chargeLimit > 0 && root.chargeLimit < 100;
    }
    function chargeLimitIsHeld(): bool {
        return root.chargeLimitIsActive() && root.chargeLimitLatched;
    }

    // Firmware does not top the pack back up the moment it dips below the limit; it waits for a
    // recharge threshold a few points lower. Assume the usual 5-point band when none is exposed.
    function chargeLimitFloor(): int {
        if (root.chargeLimitStart > 0 && root.chargeLimitStart < root.chargeLimit)
            return root.chargeLimitStart;
        return Math.max(0, root.chargeLimit - 6);
    }
    readonly property int chargeLimitHoldFloor: chargeLimitFloor()

    // Latched rather than derived: between the limit and the recharge threshold the pack sits on AC
    // losing a few points with no charge current, and a plain comparison flips the whole shell back
    // to "discharging" for hours at a time.
    property bool chargeLimitLatched: false
    readonly property bool chargeLimitReached: chargeLimitActive && chargeLimitLatched

    // Pure decision, so the band can be reasoned about (and tested) without a machine in the right
    // state: given where the latch is now, say where it should be.
    function chargeLimitHoldNext(latched, limit, floor, level, charging, holdable): bool {
        if (!holdable) return false;
        if (!charging && level >= limit - 1) return true;
        if (!latched) return false;
        if (charging && level < limit - 1) return false;
        return level >= floor - 1;
    }

    function refreshChargeLimitHold() {
        const holdable = root.available && root.deviceReady && root.chargeLimitIsActive() && root.onAc;
        root.chargeLimitLatched = root.chargeLimitHoldNext(root.chargeLimitLatched, root.chargeLimit,
            root.chargeLimitFloor(), root.percent, root.isCharging, holdable);
    }

    // Time until the effective full point (charge limit if active, otherwise UPower's estimate)
    readonly property real timeToFullEffective: {
        if (!chargeLimitActive) return timeToFull;
        if (chargeLimitReached || percent >= chargeLimit) return 0;
        const dev = root.device;
        if (!dev) return 0;
        const rate = Math.abs(dev.changeRate);
        if (dev.energyCapacity > 0 && rate > 0.01) {
            const remaining = dev.energyCapacity * (chargeLimit / 100) - dev.energy;
            return Math.max(0, remaining / rate * 3600);
        }
        if (percentage < 1 && timeToFull > 0) {
            return timeToFull * Math.max(0, chargeLimit / 100 - percentage) / (1 - percentage);
        }
        return 0;
    }

    function parseChargeLimit(content, type): int {
        const trimmed = String(content).trim();
        if (type === "bool80") return trimmed === "1" ? 80 : 100;
        const parts = trimmed.split(/\s+/);
        const val = parseInt(type === "last" ? parts[parts.length - 1] : parts[0], 10);
        // 0 means "off" on several drivers, and anything out of range is not a limit we can trust
        if (isNaN(val) || val <= 0 || val > 100) return 100;
        return val;
    }

    function parseChargeStart(content): int {
        const val = parseInt(String(content).trim().split(/\s+/)[0], 10);
        if (isNaN(val) || val <= 0 || val >= 100) return -1;
        return val;
    }

    // Refresh only what is known to exist. A machine with no threshold node at all would otherwise
    // walk the whole candidate list on every heartbeat and log a failed read for each one.
    function reloadChargeLimit() {
        if (!root.chargeLimitResolved) return;
        chargeLimitFile.reload();
        if (!root.chargeLimitStartMissing) chargeLimitStartFile.reload();
    }

    function rediscoverChargeLimit() {
        root.chargeLimitResolved = false;
        root.chargeLimitStartMissing = false;
        root.chargeLimitCandidateIndex = 0;
        chargeLimitFile.reload();
        chargeLimitStartFile.reload();
    }

    FileView {
        id: chargeLimitFile
        path: root.chargeLimitCandidates[root.chargeLimitCandidateIndex]?.path ?? ""
        printErrors: false // walking the candidate list: a missing node is the expected answer
        onLoaded: {
            const candidate = root.chargeLimitCandidates[root.chargeLimitCandidateIndex];
            if (!candidate) return;
            root.chargeLimitResolved = true;
            root.chargeLimit = root.parseChargeLimit(text(), candidate.type);
            if (candidate.startInline) {
                root.chargeLimitStart = root.parseChargeStart(text());
            } else if (!candidate.start) {
                root.chargeLimitStart = -1;
            }
            root.refreshChargeLimitHold();
        }
        onLoadFailed: {
            if (root.chargeLimitCandidateIndex < root.chargeLimitCandidates.length - 1) {
                root.chargeLimitStartMissing = false;
                root.chargeLimitCandidateIndex++;
                return;
            }
            root.chargeLimit = 100;
            root.chargeLimitStart = -1;
            root.refreshChargeLimitHold();
        }
    }

    FileView {
        id: chargeLimitStartFile
        path: root.chargeLimitStartMissing ? ""
            : (root.chargeLimitCandidates[root.chargeLimitCandidateIndex]?.start ?? "")
        printErrors: false // most firmware exposes no start threshold; onLoadFailed records it
        onLoaded: {
            root.chargeLimitStartMissing = false;
            root.chargeLimitStart = root.parseChargeStart(text());
        }
        onLoadFailed: {
            // The huawei node carries both thresholds in the limit file; don't undo what it parsed
            if (root.chargeLimitCandidates[root.chargeLimitCandidateIndex]?.startInline) return;
            root.chargeLimitStartMissing = true;
            root.chargeLimitStart = -1;
        }
    }

    FileView {
        id: cycleCountFile
        path: root.batteryNativePath ? `/sys/class/power_supply/${root.batteryNativePath}/cycle_count` : ""
        onLoaded: {
            const value = parseInt(text().trim(), 10);
            // Several drivers expose the file but report zero when the counter
            // is unsupported; do not present that as a brand-new battery.
            root.cycles = value > 0 ? value : -1;
        }
        onLoadFailed: {
            root.cycles = -1;
        }
    }

    // ── Warnings ────────────────────────────────────────────────────────────────────────────────
    // Armed/disarmed rather than edge-triggered: a level sitting on the threshold crosses it in both
    // directions over and over, and every crossing used to be a fresh notification and sound.
    property bool lowArmed: true
    property bool criticalArmed: true
    property bool chargedArmed: true

    function notify(title, body, urgency, extraHints) {
        const args = ["notify-send", title, body, "-a", "Shell",
            "--hint=int:transient:1", "--hint=boolean:suppress-sound:true"];
        if (urgency) args.push("-u", urgency);
        for (let i = 0; i < (extraHints?.length ?? 0); ++i) args.push(extraHints[i]);
        Quickshell.execDetached(args);
    }

    // Pure halves of the arm/fire decision, kept separate so the hysteresis can be checked without
    // waiting for a flat battery.
    function warningArmed(armed, threshold, level, unplugged): bool {
        if (!unplugged) return true;
        if (level >= threshold + root.rearmMargin) return true;
        return armed;
    }
    function warningFires(armed, threshold, level, unplugged): bool {
        return unplugged && armed && level <= threshold;
    }

    function evaluateBatteryState() {
        if (!root.available || !root.deviceReady) {
            root.cancelAutomaticSuspend(false);
            return;
        }

        const opts = Config.options.battery;
        const level = root.percent;
        const unplugged = !root.isPluggedIn;

        // A charger re-arms everything; so does climbing back clear of the threshold
        root.criticalArmed = root.warningArmed(root.criticalArmed, opts.critical, level, unplugged);
        root.lowArmed = root.warningArmed(root.lowArmed, opts.low, level, unplugged);

        if (root.warningFires(root.criticalArmed, opts.critical, level, unplugged)) {
            root.criticalArmed = false;
            root.lowArmed = false; // the low warning is noise once this one has fired
            root.warnCritical(opts);
        } else if (root.warningFires(root.lowArmed, opts.low, level, unplugged)) {
            root.lowArmed = false;
            root.warnLow();
        }

        root.evaluateChargedNotification(unplugged, level, opts);
        root.evaluateAutomaticSuspend(unplugged, level, opts);
    }

    function warnLow() {
        root.notify(Translation.tr("Low battery"), Translation.tr("Consider plugging in your device"),
            "critical");
        SoundService.playEvent("battery", ["battery-low", "dialog-warning"]);
    }

    function warnCritical(opts) {
        const body = opts.automaticSuspend
            ? Translation.tr("Please charge!\nAutomatic suspend triggers at %1%").arg(opts.suspend)
            : Translation.tr("Please charge!");
        root.notify(Translation.tr("Critically low battery"), body, "critical");
        SoundService.playEvent("battery", ["battery-caution", "suspend-error", "dialog-error"]);
    }

    function evaluateChargedNotification(unplugged, level, opts) {
        if (unplugged || !root.chargedArmed) return;
        if (!(Config.options.battery.notifyCharged ?? true)) return;

        // A firmware limit holding the pack is the finished state on this machine: saying "unplug"
        // there would be advice against the whole point of the limit.
        if (root.chargeLimitIsHeld()) {
            root.chargedArmed = false;
            root.announceCharged(Translation.tr("Charged to %1%").arg(root.chargeLimit),
                Translation.tr("The charge limit is holding it here — the charger can stay in."));
            return;
        }
        if (root.isFullyCharged || level >= 100) {
            root.chargedArmed = false;
            root.announceCharged(Translation.tr("Battery full"),
                Translation.tr("You can unplug the charger."));
            return;
        }
        if (opts.full <= 100 && level >= opts.full) {
            root.chargedArmed = false;
            root.announceCharged(Translation.tr("Battery at %1%").arg(level),
                Translation.tr("Consider unplugging the charger."));
        }
    }

    function announceCharged(title, body) {
        root.notify(title, body);
        SoundService.playEvent("battery", ["battery-full", "complete", "dialog-information"]);
    }

    // ── Automatic suspend ───────────────────────────────────────────────────────────────────────
    readonly property int suspendWarningSeconds: Math.max(0,
        Config.options.battery.suspendWarningSeconds ?? 30)
    property bool suspendPending: false
    property int suspendCountdown: 0
    // One automatic suspend per discharge; without this the machine wakes at 3 % and suspends again,
    // and a cancelled countdown restarts the moment the next reading arrives.
    property bool suspendInhibitedUntilRecovery: false

    function evaluateAutomaticSuspend(unplugged, level, opts) {
        if (!unplugged || level >= opts.suspend + root.rearmMargin)
            root.suspendInhibitedUntilRecovery = false;

        const shouldSuspend = root.allowAutomaticSuspend && unplugged && level <= opts.suspend
            && !root.suspendInhibitedUntilRecovery;
        if (!shouldSuspend) {
            root.cancelAutomaticSuspend(false);
            return;
        }
        if (root.suspendPending || suspendConfirmTimer.running) return;
        // One reading is not enough: a single bad sample must not put the machine to sleep
        suspendConfirmTimer.restart();
    }

    Timer {
        id: suspendConfirmTimer
        interval: 8000
        onTriggered: {
            if (!root.available || !root.deviceReady) return;
            if (!root.allowAutomaticSuspend || root.isPluggedIn) return;
            if (root.suspendInhibitedUntilRecovery) return;
            if (root.percent > Config.options.battery.suspend) return;
            root.beginAutomaticSuspend();
        }
    }

    function beginAutomaticSuspend() {
        if (root.suspendPending) return;
        if (root.suspendWarningSeconds <= 0) {
            root.suspendNow();
            return;
        }
        root.suspendPending = true;
        root.suspendCountdown = root.suspendWarningSeconds;
        suspendCountdownTimer.restart();
        root.notify(Translation.tr("Battery critically low"),
            Translation.tr("Suspending in %1 s. Plug in to cancel.").arg(root.suspendWarningSeconds),
            "critical", ["-t", String(root.suspendWarningSeconds * 1000),
                "--hint=string:x-qs-notif:battery-suspend-warn"]);
        SoundService.playEvent("battery", ["battery-caution", "dialog-warning"]);
    }

    Timer {
        id: suspendCountdownTimer
        interval: 1000
        repeat: true
        onTriggered: {
            if (root.isPluggedIn || !root.allowAutomaticSuspend) {
                root.cancelAutomaticSuspend(false);
                return;
            }
            root.suspendCountdown--;
            if (root.suspendCountdown <= 0) root.suspendNow();
        }
    }

    function cancelAutomaticSuspend(byUser) {
        suspendConfirmTimer.stop();
        if (!root.suspendPending) return;
        suspendCountdownTimer.stop();
        root.suspendPending = false;
        root.suspendCountdown = 0;
        root.dismissSuspendWarning();
        // A deliberate cancel has to survive the next reading, which still sees a flat battery
        if (byUser) root.suspendInhibitedUntilRecovery = true;
    }

    function suspendNow() {
        suspendConfirmTimer.stop();
        suspendCountdownTimer.stop();
        root.suspendPending = false;
        root.suspendCountdown = 0;
        root.suspendInhibitedUntilRecovery = true;
        root.dismissSuspendWarning();
        Quickshell.execDetached(["bash", "-c", `systemctl suspend || loginctl suspend`]);
    }

    function dismissSuspendWarning() {
        const list = Notifications.list ?? [];
        for (let i = list.length - 1; i >= 0; --i) {
            if (list[i]?.notification?.hints?.["x-qs-notif"] === "battery-suspend-warn")
                Notifications.discardNotification(list[i].notificationId);
        }
    }

    // ── Wiring ──────────────────────────────────────────────────────────────────────────────────
    onBatteryNativePathChanged: {
        cycleCountFile.reload();
        root.rediscoverChargeLimit();
    }

    onChargeStateChanged: {
        cycleCountFile.reload();
        root.reloadChargeLimit();
        root.refreshChargeLimitHold();
        root.evaluateBatteryState();
    }

    onPercentChanged: {
        root.refreshChargeLimitHold();
        root.evaluateBatteryState();
    }

    onDeviceReadyChanged: {
        root.refreshChargeLimitHold();
        root.evaluateBatteryState();
    }

    onAvailableChanged: root.evaluateBatteryState()

    onOnAcChanged: root.refreshChargeLimitHold()

    onIsPluggedInChanged: {
        if (!root.available) return;
        if (root.isPluggedIn) {
            root.refreshChargeLimitHold();
            // Plugging in a pack that is already at its ceiling is not news worth a notification
            root.chargedArmed = !(root.isFullyCharged || root.percent >= 100 || root.chargeLimitIsHeld());
            SoundService.playEvent("battery", "power-plug");
        } else {
            root.chargedArmed = true;
            SoundService.playEvent("battery", "power-unplug");
        }
        root.evaluateBatteryState();
    }

    // Sysfs has no usable change notification and UPower stops emitting when nothing moves, so the
    // limit (which tlp, vendor tools and firmware menus rewrite behind our back) is re-read on a
    // heartbeat, which also picks up threshold edits made in settings.
    Timer {
        interval: 30000
        running: root.available
        repeat: true
        onTriggered: {
            root.reloadChargeLimit();
            root.refreshChargeLimitHold();
            root.evaluateBatteryState();
        }
    }
}

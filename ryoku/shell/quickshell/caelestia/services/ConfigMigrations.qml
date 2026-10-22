pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io
import Caelestia.Config
import caelestia.utils

Singleton {
    id: root


    readonly property var retiredStatusKeys: [
        { id: "lockStatus", key: "showLockStatus" },
        { id: "microphone", key: "showMicrophone" },
        { id: "kbLayout", key: "showKbLayout" },
        { id: "network", key: "showNetwork" },
        { id: "ethernet", key: "showNetwork" },
        { id: "bluetooth", key: "showBluetooth" },
        { id: "audio", key: "showAudio" },
        { id: "battery", key: "showBattery" },
        { id: "peripheralBattery", key: "showPeripheralBattery" },
        { id: "nightlight", key: "showNightLight" },
        { id: "notifications", key: "showNotifications" }
    ]
    readonly property var retiredIconNames: ({
            lockstatus: "lockStatus",
            kblayout: "kbLayout"
        })
    readonly property string orderFilePath: `${Paths.config}/status_icons_order.txt`

    // The order the file asked for, as the list spells the ids, without anything the
    // list has no entry for.
    function retiredOrder(order: string): var {
        const ids = [];
        for (const name of order.split(",")) {
            const id = root.retiredIconNames[name] ?? name;
            if (id && root.retiredStatusKeys.some(entry => entry.id === id) && !ids.includes(id))
                ids.push(id);
        }
        return ids;
    }

    function migrateStatusIcons(order: string): void {
        const status = GlobalConfig.bar.status;
        const icons = GlobalConfig.bar.statusIcons;

        if (icons.isOverride("values")) {
            if (order)
                Quickshell.execDetached(["rm", "-f", root.orderFilePath]);
            return;
        }

        if (!order && !root.retiredStatusKeys.some(entry => status.isOverride(entry.key)))
            return;

        const enabled = {};
        for (const entry of root.retiredStatusKeys)
            enabled[entry.id] = status[entry.key];

        const shipped = icons.values.map(entry => entry.id);
        const wanted = root.retiredOrder(order).filter(id => shipped.includes(id));
        for (const id of shipped)
            if (!wanted.includes(id))
                wanted.push(id);

        for (let i = 0; i < wanted.length; i++) {
            const from = icons.values.findIndex(entry => entry.id === wanted[i]);
            if (from >= 0 && from !== i)
                icons.move(from, i);
            icons.at(i).enabled = enabled[wanted[i]] ?? false;
        }

        for (const entry of root.retiredStatusKeys)
            status.resetOption(entry.key);
    }

    function migrateWorkspaceDisplay(): void {
        const workspaces = GlobalConfig.bar.workspaces;
        if (!workspaces.isOverride("useIcon"))
            return;

        workspaces.displayType = workspaces.useIcon ? BarWorkspaceDisplay.Shapes : BarWorkspaceDisplay.Text;
        workspaces.resetOption("useIcon");
    }

    function migrateQuickToggles(): void {
        const utilities = GlobalConfig.utilities;
        if (!utilities.isOverride("quickToggles"))
            return;

        const toggles = utilities.quickToggles || [];
        if (toggles.some(toggle => toggle.id === "gameMode"))
            return;

        const next = toggles.map(toggle => ({ id: toggle.id, enabled: toggle.enabled !== false }));
        const settings = next.findIndex(toggle => toggle.id === "settings");
        next.splice(settings < 0 ? next.length : settings + 1, 0, { id: "gameMode", enabled: true });
        utilities.quickToggles = next;
    }

    function migrateTemperatureUnits(): void {
        const services = GlobalConfig.services;

        if (services.isOverride("useFahrenheit")) {
            services.weatherUnits = services.useFahrenheit ? TemperatureUnit.Fahrenheit : TemperatureUnit.Celsius;
            services.resetOption("useFahrenheit");
        }

        if (services.isOverride("useFahrenheitPerformance")) {
            services.sensorUnits = services.useFahrenheitPerformance ? TemperatureUnit.Fahrenheit : TemperatureUnit.Celsius;
            services.resetOption("useFahrenheitPerformance");
        }
    }

    function migrateClockFormat(): void {
        const services = GlobalConfig.services;

        if (services.isOverride("useTwelveHourClock")) {
            services.clockFormat = services.useTwelveHourClock ? ClockFormat.TwelveHour : ClockFormat.TwentyFourHour;
            services.resetOption("useTwelveHourClock");
        }
    }

    function migratePerMonitor(): void {
        const workspaces = GlobalConfig.bar.workspaces;

        if (!workspaces.isOverride("perMonitorWorkspaces"))
            return;

        workspaces.perMonitor = workspaces.perMonitorWorkspaces;
        workspaces.resetOption("perMonitorWorkspaces");
    }

    function migrateDockPinned(): void {
        const dock = GlobalConfig.bar.dock;
        const launcher = GlobalConfig.launcher;
        if (!dock.isOverride("pinnedApps") && launcher.isOverride("favouriteApps")) {
            dock.pinnedApps = [...launcher.favouriteApps];
        }
    }

    Component.onCompleted: {
        root.migrateWorkspaceDisplay();
        root.migrateQuickToggles();
        root.migrateTemperatureUnits();
        root.migrateClockFormat();
        root.migratePerMonitor();
        root.migrateDockPinned();
        orderReader.running = true;
    }

    Process {
        id: orderReader

        // `cat` rather than a declarative read so that the answer arrives either way:
        // what the status icons need to know is not only what the file says but that it
        // is not there at all.
        command: ["cat", root.orderFilePath]

        stdout: StdioCollector {
            id: orderOutput
        }

        onExited: root.migrateStatusIcons(orderOutput.text.trim())
    }
}

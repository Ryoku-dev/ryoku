pragma Singleton
import QtQuick
import Quickshell

Singleton {
    id: root

    // Default and Expressive style options shared by most components
    readonly property var defaultStyleOptions: [
        {
            displayName: qsTr("Default"),
            icon: "style",
            value: "default"
        },
        {
            displayName: qsTr("Expressive"),
            icon: "fluid_med",
            value: "expressive"
        }
    ]

    readonly property var allComponents: [
        {
            id: "policies_panel_button",
            icon: "star",
            title: "Policies panel button",
            styleConfigKey: "policies",
            styleOptions: [
                {
                    displayName: qsTr("Default"),
                    icon: "style",
                    value: "default"
                },
                {
                    displayName: qsTr("Expressive"),
                    icon: "fluid_med",
                    value: "expressive"
                },
                {
                    displayName: qsTr("Outline"),
                    icon: "circle",
                    value: "outline"
                }
            ],
            pageId: "privacy"
        },
        {
            id: "active_window",
            icon: "label",
            title: "Active window",
            styleConfigKey: "activeWindow",
            styleOptions: defaultStyleOptions,
            configPage: "ActiveWindowConfig.qml"
        },
        {
            id: "search",
            icon: "search",
            title: "Search",
            styleConfigKey: "search",
            styleOptions: [
                {
                    displayName: qsTr("Default"),
                    icon: "style",
                    value: "default"
                },
                {
                    displayName: qsTr("Expressive"),
                    icon: "fluid_med",
                    value: "expressive"
                },
                {
                    displayName: qsTr("Neural"),
                    icon: "neurology",
                    value: "neural"
                }
            ],
            configPage: "SearchBarWidgetConfig.qml"
        },
        {
            id: "music_player",
            icon: "music_note",
            title: "Music player",
            styleConfigKey: "media",
            styleOptions: [
                {
                    displayName: qsTr("Default"),
                    icon: "style",
                    value: "default"
                },
                {
                    displayName: qsTr("Expressive"),
                    icon: "fluid_med",
                    value: "expressive"
                },
                {
                    displayName: qsTr("Neural"),
                    icon: "graphic_eq",
                    value: "neural"
                },
                {
                    displayName: qsTr("Ring"),
                    icon: "motion_photos_on",
                    value: "ring"
                },
                {
                    displayName: qsTr("Tonal"),
                    icon: "gradient",
                    value: "tonal"
                }
            ],
            configPage: "MediaPlayerConfig.qml"
        },
        {
            id: "workspaces",
            icon: "workspaces",
            title: "Workspaces",
            styleConfigKey: "workspaces",
            configPage: "../WorkspacesConfig.qml",
            styleOptions: [
                {
                    displayName: qsTr("Default"),
                    icon: "workspaces",
                    value: "default"
                },
                {
                    displayName: qsTr("Minimal"),
                    icon: "navigation",
                    value: "minimal"
                },
                {
                    displayName: qsTr("Expressive"),
                    icon: "fluid_med",
                    value: "expressive"
                },
                {
                    displayName: qsTr("Dock"),
                    icon: "dock_to_left",
                    value: "dock"
                },
                {
                    displayName: qsTr("Index"),
                    icon: "format_list_numbered",
                    value: "index"
                }
            ]
        },
        {
            id: "system_monitor",
            icon: "monitor_heart",
            title: "System monitor",
            styleConfigKey: "resources",
            styleOptions: defaultStyleOptions,
            configPage: "SystemMonitorConfig.qml"
        },
        {
            id: "ai_plan_usage",
            icon: "neurology",
            title: "AI Plan Usage",
            styleConfigKey: "aiPlanUsage",
            styleOptions: defaultStyleOptions,
            configPage: "AiPlanUsageConfig.qml"
        },
        {
            id: "privacy_pill",
            icon: "shield_lock",
            title: "Privacy pill",
            configPage: "PrivacyPillConfig.qml"
        },
        {
            id: "port_watcher",
            icon: "lan",
            title: "Port Watcher",
            styleConfigKey: "portWatcher",
            styleOptions: defaultStyleOptions,
            configPage: "PortWatcherConfig.qml"
        },
        {
            id: "clock",
            icon: "nest_clock_farsight_analog",
            title: "Clock",
            styleConfigKey: "clock",
            configPage: "ClockBarWidgetConfig.qml",
            styleOptions: [
                {
                    displayName: qsTr("Default"),
                    icon: "style",
                    value: "default"
                },
                {
                    displayName: qsTr("Material"),
                    icon: "interests",
                    value: "material"
                },
                {
                    displayName: qsTr("Expressive"),
                    icon: "fluid_med",
                    value: "expressive"
                },
                {
                    displayName: qsTr("Neural"),
                    icon: "neurology",
                    value: "neural"
                },
                {
                    displayName: qsTr("Relief"),
                    icon: "content_cut",
                    value: "relief"
                }
            ]
        },
        {
            id: "system_tray",
            icon: "system_update_alt",
            title: "System tray",
            styleConfigKey: "systray",
            styleOptions: defaultStyleOptions,
            configPage: "SystemTrayConfig.qml"
        },
        {
            id: "dashboard_panel_button",
            icon: "notifications",
            title: "Dashboard panel button",
            styleConfigKey: "dashboard",
            styleOptions: [
                {
                    displayName: qsTr("Default"),
                    icon: "style",
                    value: "default"
                },
                {
                    displayName: qsTr("Expressive"),
                    icon: "fluid_med",
                    value: "expressive"
                },
                {
                    displayName: qsTr("Orbs"),
                    icon: "workspaces",
                    value: "orbs"
                }
            ],
            configPage: "DashboardButtonConfig.qml"
        },
        {
            id: "record_indicator",
            icon: "screen_record",
            title: "Record indicator",
            styleConfigKey: "recordIndicator",
            styleOptions: [
                {
                    displayName: qsTr("Default"),
                    icon: "style",
                    value: "default"
                },
                {
                    displayName: qsTr("Expressive"),
                    icon: "fluid_med",
                    value: "expressive"
                },
                {
                    displayName: qsTr("Neural"),
                    icon: "neurology",
                    value: "neural"
                }
            ],
            configPage: "RecordIndicatorConfig.qml"
        },
        {
            id: "screen_share_indicator",
            icon: "screen_share",
            title: "Screen share indicator",
            configPage: "IndicatorsConfig.qml"
        },
        {
            id: "phone_scrcpy_indicator",
            icon: "smart_display",
            title: "Phone scrcpy indicator",
            pageId: "devicesPhone"
        },
        {
            id: "shell_update_indicator",
            icon: "deployed_code_update",
            title: "Shell update indicator",
            pageId: "about"
        },
        {
            id: "mode_indicator",
            icon: "tune",
            title: "Mode indicator",
            configPage: "IndicatorsConfig.qml"
        },
        {
            id: "easyeffects_indicator",
            icon: "graphic_eq",
            title: "EasyEffects preset",
            configPage: "IndicatorsConfig.qml"
        },
        {
            id: "date",
            icon: "date_range",
            title: "Date",
            styleConfigKey: "date",
            configPage: "DateBarWidgetConfig.qml",
            styleOptions: [
                {
                    displayName: qsTr("Default"),
                    icon: "style",
                    value: "default"
                },
                {
                    displayName: qsTr("Expressive"),
                    icon: "fluid_med",
                    value: "expressive"
                },
                {
                    displayName: qsTr("Neural"),
                    icon: "neurology",
                    value: "neural"
                }
            ]
        },
        {
            id: "battery",
            icon: "battery_android_6",
            title: "Battery",
            styleConfigKey: "battery",
            configPage: "BatteryConfig.qml",
            styleOptions: [
                {
                    displayName: qsTr("Default"),
                    icon: "style",
                    value: "default"
                },
                {
                    displayName: qsTr("Material"),
                    icon: "interests",
                    value: "material"
                },
                {
                    displayName: qsTr("Expressive"),
                    icon: "fluid_med",
                    value: "expressive"
                }
            ]
        },
        {
            id: "timer",
            icon: "timer",
            title: "Timer & Pomodoro",
            styleConfigKey: "timer",
            styleOptions: defaultStyleOptions,
            configPage: "IndicatorsConfig.qml"
        },
        {
            id: "weather",
            icon: "weather_mix",
            title: "Weather",
            styleConfigKey: "weather",
            styleOptions: [
                {
                    displayName: qsTr("Default"),
                    icon: "style",
                    value: "default"
                },
                {
                    displayName: qsTr("Expressive"),
                    icon: "fluid_med",
                    value: "expressive"
                },
                {
                    displayName: qsTr("Horizon"),
                    icon: "landscape",
                    value: "horizon"
                },
                {
                    displayName: qsTr("Tessera"),
                    icon: "view_module",
                    value: "tessera"
                }
            ],
            configPage: "WeatherBarWidgetConfig.qml"
        },
        {
            id: "utility_buttons",
            icon: "build",
            title: "Utility buttons",
            styleConfigKey: "utilButtons",
            styleOptions: [
                {
                    displayName: qsTr("Default"),
                    icon: "style",
                    value: "default"
                },
                {
                    displayName: qsTr("Expressive"),
                    icon: "fluid_med",
                    value: "expressive"
                },
                {
                    displayName: qsTr("Segments"),
                    icon: "view_week",
                    value: "segments"
                }
            ],
            configPage: "UtilButtonsConfig.qml"
        },
        {
            id: "bluetooth_devices",
            icon: "bluetooth_connected",
            title: "Bluetooth Devices",
            styleConfigKey: "bluetooth",
            styleOptions: defaultStyleOptions,
            configPage: "BluetoothConfig.qml"
        },
        {
            id: "keyboard_layout",
            icon: "keyboard",
            title: "Keyboard Layout",
            styleConfigKey: "keyboard",
            configPage: "KeyboardLayoutConfig.qml",
            styleOptions: [
                {
                    displayName: qsTr("Default"),
                    icon: "style",
                    value: "default"
                },
                {
                    displayName: qsTr("Material"),
                    icon: "interests",
                    value: "material"
                },
                {
                    displayName: qsTr("Expressive"),
                    icon: "fluid_med",
                    value: "expressive"
                }
            ]
        },
        {
            id: "sports",
            icon: "sports_soccer",
            title: "Sports",
            styleConfigKey: "sports",
            styleOptions: defaultStyleOptions,
            configPage: "SportsConfig.qml"
        },
        {
            id: "power",
            icon: "power_settings_new",
            title: "Power button",
            styleConfigKey: "power",
            styleOptions: [
                {
                    displayName: qsTr("Default"),
                    icon: "style",
                    value: "default"
                },
                {
                    displayName: qsTr("Expressive"),
                    icon: "fluid_med",
                    value: "expressive"
                },
                {
                    displayName: qsTr("Solid"),
                    icon: "radio_button_checked",
                    value: "solid"
                },
                {
                    displayName: qsTr("Dot"),
                    icon: "fiber_manual_record",
                    value: "dot"
                }
            ],
            pageId: "power"
        },
        {
            id: "dock_to_panel",
            icon: "apps",
            title: "Dock to Panel",
            configPage: "DockToPanelConfig.qml"
        }
    ]

    // The Settings Bar page uses this list for its widget cards. Keep the
    // filter result stable so page construction does not repeat the same
    // registry scan and allocate a new model expression.
    readonly property var configurableComponents: allComponents.filter(c => c.configPage || c.pageId)

    readonly property var componentById: {
        const map = {};
        for (const component of allComponents) {
            map[component.id] = component;
        }
        return map;
    }

    function getComponent(id) {
        return componentById[id] ?? null;
    }

    function getAvailableComponents(usedIds) {
        return allComponents.filter(c => !usedIds.includes(c.id));
    }
}

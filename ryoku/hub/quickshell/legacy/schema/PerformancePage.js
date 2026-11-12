.pragma library

// PerformancePage as data, for the search index: the same rows the page draws,
// grouped the way it groups them.

var rows = [
    {
        "tab": "", "group": "POWER", "key": "powerProfileEffects", "ctl": "sw", "src": "performance.json",
        "label": "Follow the power profile",
        "desc": "Power Saver strips motion, blur and shadows"
    },
    {
        "tab": "", "group": "POWER", "key": "autoPowerSaverOnBattery", "ctl": "sw", "src": "performance.json",
        "label": "Auto power saver on battery",
        "desc": "Switches to Power Saver when you unplug"
    },
    {
        "tab": "", "group": "EFFECTS", "key": "lowPowerMode", "ctl": "sw", "src": "performance.json", "caps": "liveConfigEval",
        "label": "Low power mode",
        "desc": "Turns every effect switch here on at once"
    },
    {
        "tab": "", "group": "EFFECTS", "key": "reduceMotion", "ctl": "sw", "src": "performance.json",
        "label": "Reduce motion",
        "desc": "Shell transitions land instantly"
    },
    {
        "tab": "", "group": "EFFECTS", "key": "disableBlur", "ctl": "sw", "src": "performance.json", "caps": "liveConfigEval",
        "label": "Disable blur",
        "desc": "Drops the frosted-glass look everywhere"
    },
    {
        "tab": "", "group": "EFFECTS", "key": "disableShadows", "ctl": "sw", "src": "performance.json", "caps": "liveConfigEval",
        "label": "Disable shadows",
        "desc": "Surfaces draw without a shadow pass"
    },
    {
        "tab": "", "group": "MOTION", "key": "liveWallpaper60", "ctl": "sw", "src": "performance.json",
        "label": "60fps live wallpaper",
        "desc": "Smoother video wallpaper, at a decode cost"
    },
    {
        "tab": "", "group": "MOTION", "key": "pauseLiveWallpaperWhenFullscreen", "ctl": "sw", "src": "performance.json",
        "label": "Pause video wallpaper",
        "desc": "Stops the video behind a fullscreen window"
    },
    {
        "tab": "", "group": "MOTION", "key": "ambientBarMotion", "ctl": "sw", "src": "performance.json",
        "label": "Bar drifts when silent",
        "desc": "The bar keeps drifting while nothing plays"
    },
    {
        "tab": "", "group": "MEMORY", "key": "unloadWidgetsWhenCovered", "ctl": "sw", "src": "performance.json",
        "label": "Hide covered widgets",
        "desc": "Parks widgets while every monitor is covered"
    },
    {
        "tab": "", "group": "MEMORY", "key": "unloadVisualizerWhenSilent", "ctl": "sw", "src": "performance.json",
        "label": "Unload the visualiser",
        "desc": "Frees ~250 MB after 30s of silence"
    },
    {
        "tab": "", "group": "MEMORY", "key": "unloadLauncherWhenIdle", "ctl": "sw", "src": "performance.json",
        "label": "Unload the launcher",
        "desc": "Frees ~250 MB a minute after closing"
    },
    {
        "tab": "", "group": "MEMORY", "key": "unloadOverviewWhenIdle", "ctl": "sw", "src": "performance.json",
        "label": "Unload the overview",
        "desc": "Frees ~250 MB a minute after Super+Tab closes"
    }
];

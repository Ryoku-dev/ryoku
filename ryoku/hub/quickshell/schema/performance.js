.pragma library

var title = "Performance";
var lead = "Turn down the desktop's effects and let idle parts unload, for a lighter load on the processor, graphics and memory.";

var sections = [
    { id: "low-power", title: "Low power mode" },
    { id: "wallpaper", title: "Live wallpaper" },
    { id: "bar", title: "Bar motion" },
    { id: "unload", title: "Unload idle parts" }
];

var related = ["power", "animations", "window-style", "graphics"];

function perf(key) { return { store: "performance", key: key }; }

var rows = [
    { id: "low-power", section: "low-power", label: "Low power mode",
      detail: "Turns off the desktop's motion, blur and shadows and stills the music visualizer, for a weak graphics card or a long day on battery",
      keywords: ["potato", "weak gpu", "lag", "slow", "eye candy", "effects off", "lightweight", "battery life"],
      control: "toggle", bind: perf("lowPowerMode"), "default": false },
    { id: "reduce-motion", section: "low-power", label: "Reduce motion",
      detail: "Its home is Animations; low power mode keeps motion reduced while it is on",
      keywords: ["animations off", "instant", "accessibility", "motion sickness"],
      control: "link", link: "animations/reduce-motion", actionText: "Open" },
    { id: "blur-shadows", section: "low-power", label: "Blur and shadows",
      detail: "Their home is Window style; low power mode turns them off while it is on",
      keywords: ["disable blur", "disable shadows", "frosted glass", "transparency"],
      control: "link", link: "window-style/blur", actionText: "Open" },

    { id: "wallpaper-efficiency", section: "wallpaper", label: "Wallpaper efficiency",
      detail: "Performance mode, video decoding and the battery saver, in the wallpaper app",
      keywords: ["live wallpaper", "video wallpaper", "decode", "performance mode", "battery", "ryogami", "frame rate", "60 fps"],
      control: "handoff", command: ["ryogami", "wallpaper", "settings", "system", "performance", "paper.performanceMode"],
      actionText: "Open" },
    { id: "wallpaper-pause", section: "wallpaper", label: "When video wallpapers pause",
      detail: "Pause behind full-screen apps and while chosen apps run, in the wallpaper app",
      keywords: ["pause video wallpaper", "fullscreen", "full screen", "games", "live wallpaper", "ryogami", "playback"],
      control: "handoff", command: ["ryogami", "wallpaper", "settings", "wallpaper", "playback", "playback.processEnabled"],
      actionText: "Open" },

    { id: "bar-drift", section: "bar", label: "Keep the bar moving in silence",
      detail: "The bar drifts gently while nothing plays. Off, it rests and saves a little power.",
      keywords: ["ambient bar motion", "drift", "idle animation", "bar animation", "silent"],
      control: "toggle", bind: perf("ambientBarMotion"), "default": false },

    { id: "unload-widgets", section: "unload", label: "Unload covered widgets",
      detail: "Desktop widgets leave memory while windows cover every display",
      keywords: ["memory", "ram", "widgets", "covered", "park"],
      control: "toggle", bind: perf("unloadWidgetsWhenCovered"), "default": true },
    { id: "unload-visualizer", section: "unload", label: "Unload the music visualizer",
      detail: "Frees about 250 MB after 30 seconds of silence",
      keywords: ["memory", "ram", "visualiser", "visualizer", "cava", "audio"],
      control: "toggle", bind: perf("unloadVisualizerWhenSilent"), "default": true },
    { id: "unload-launcher", section: "unload", label: "Unload the app launcher",
      detail: "Frees about 250 MB a minute after it closes; the next open takes a moment longer",
      keywords: ["memory", "ram", "launcher", "app search"],
      control: "toggle", bind: perf("unloadLauncherWhenIdle"), "default": true },
    { id: "unload-overview", section: "unload", label: "Unload the overview",
      detail: "Frees about 250 MB a minute after the overview closes",
      keywords: ["memory", "ram", "overview", "expose", "super tab"],
      control: "toggle", bind: perf("unloadOverviewWhenIdle"), "default": true }
];

.pragma library

var title = "Animations";
var lead = "How fast and how much the desktop moves: panels and menus, windows opening and closing, and the effects in between.";

var sections = [
    { id: "speed", title: "Motion speed" },
    { id: "reduce", title: "Reduce motion" },
    { id: "style", title: "Animation style" },
    { id: "open-close", title: "Window open and close", missing: "This window manager opens and closes windows without a choice of effect." },
    { id: "wobbly", title: "Wobbly windows", missing: "Wobbly windows need a window manager that can bend them." },
    { id: "focus-flash", title: "Focus flash", missing: "Focus flash needs a window manager plugin that draws it." },
    // provider rows land here; the page draws them inside Animation style
    { id: "curves", title: "Animation curves" },
    { id: "per-animation", title: "Each animation" }
];

function s(key, fallback) {
    return { store: "settings", key: key, fallback: fallback };
}

function d(key) {
    return { store: "desktop", key: key };
}

// template rows feed search only; the page draws what they stand for
var rows = [
    { id: "motion-speed", section: "speed", label: "Speed",
      detail: "How fast panels, menus and transitions move; 100% is the usual pace",
      keywords: ["animation speed", "tempo", "faster", "slower", "motion scale", "duration"],
      control: "slider", min: 0.25, max: 3, step: 0.05, pct: true,
      bind: s("theme.motion.scale", 1), "default": 1 },
    { id: "window-animations", section: "speed", label: "Window animations",
      detail: "Windows and workspaces move as they open, close and switch; off makes them instant",
      keywords: ["animations off", "disable animations", "turn off animations", "master switch", "instant"],
      control: "toggle", bind: d("appearance.animations") },

    { id: "reduce-motion", section: "reduce", label: "Reduce motion",
      detail: "Panels, menus and transitions land in place instead of moving",
      keywords: ["accessibility", "motion sickness", "vestibular", "less motion", "no animations", "instant", "prefers reduced motion"],
      control: "toggle", bind: s("theme.motion.reduce", false), "default": false },

    { id: "anim-preset", section: "style", label: "Animation style",
      detail: "How windows, workspaces and panels move; changing it reloads the window manager",
      keywords: ["preset", "personality", "feel", "bounce", "fade", "fast", "minimal", "dusky"],
      control: "picker", bind: { store: "animations", key: "preset" }, "default": "ryoku",
      options: [
          { value: "ryoku", label: "Ryoku" },
          { value: "dusky", label: "Dusky" },
          { value: "bounce", label: "Bounce" },
          { value: "fade", label: "Fade" },
          { value: "fast", label: "Fast" },
          { value: "mechanical", label: "Mechanical" },
          { value: "minimal", label: "Minimal" },
          { value: "air", label: "Air" },
          { value: "slowmotion", label: "Slow motion" },
          { value: "exaggerated", label: "Exaggerated" },
          { value: "hallucination", label: "Hallucination" },
          { value: "rage", label: "Rage" },
          { value: "disable", label: "Off" }
      ] },
    { id: "curves", section: "style", label: "Animation curves", template: true, more: true,
      detail: "The easing shapes animations follow; drag the handles to make your own",
      keywords: ["bezier", "easing", "curve editor", "handles", "custom curve", "overshoot"] },
    { id: "each-animation", section: "style", label: "Each animation", template: true, more: true,
      detail: "Turn single animations off, or change how long they take, their curve and style",
      keywords: ["per animation", "workspace switch", "window move", "window resize", "fade", "layers", "spring", "duration"] },
    { id: "reset-animations", section: "style", label: "Reset every animation", template: true, more: true,
      detail: "Clear your changes to single animations and your own curves",
      keywords: ["defaults", "restore", "undo animations"] },

    { id: "window-open-style", section: "open-close", label: "Open and close effect",
      detail: "How a window appears and leaves",
      keywords: ["pop in", "slide", "gnomed", "from the middle", "window open", "window close", "popin"],
      control: "segmented", bind: d("appearance.windowStyle"),
      options: [
          { value: "pop", label: "Pop" },
          { value: "slide", label: "Slide" },
          { value: "gnomed", label: "Unfold" }
      ],
      when: { bind: d("appearance.animations"), is: [true] } },

    { id: "wobbly", section: "wobbly", label: "Wobbly windows",
      detail: "A dragged window bends and springs back into shape",
      keywords: ["wobble", "jelly", "elastic", "spring", "compiz"],
      control: "toggle", bind: d("appearance.wobblyWindows") }
];

var related = ["window-style", "performance", "mouse-touchpad", "window-manager"];

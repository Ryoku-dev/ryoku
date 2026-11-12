.pragma library

var title = "Window behaviour";
var lead = "How windows tile, take focus, open, move and gather into workspaces.";

var sections = [
    { id: "tiling", title: "Tiling", missing: "This window manager has one way of tiling, so there is nothing to choose." },
    { id: "gaps", title: "Gaps" },
    { id: "new-windows", title: "New windows" },
    { id: "focus", title: "Focus" },
    { id: "moving", title: "Moving and resizing" },
    { id: "tabbed", title: "Tabbed columns" },
    { id: "room", title: "Room for bars and docks" },
    { id: "workspaces", title: "Workspaces" },
    { id: "overview", title: "Overview and window switcher" },
    { id: "urgent", title: "Urgent windows" },
    { id: "zoom", title: "Desktop zoom" },
    { id: "more", title: "Compatibility" }
];

function d(key) { return { store: "desktop", key: key }; }

// the focus choices every window manager understands
var focusOptions = [
    { value: 1, label: "Follows the pointer" },
    { value: 0, label: "Click to focus" }
];

// finer modes some window managers keep; the page adds the one in use so it stays visible and choosable
var extraFocus = [
    { value: 2, label: "Click, but scrolling follows the pointer" },
    { value: 3, label: "Keyboard only, clicks leave focus alone" }
];

var rows = [
    { id: "tiling-layout", section: "tiling", label: "Tiling layout",
      detail: "How new windows share the screen",
      keywords: ["dwindle", "master", "stack", "scrolling", "columns", "split", "layout"],
      control: "segmented",
      options: [
          { value: "dwindle", label: "Split" },
          { value: "master", label: "Main and stack" },
          { value: "scrolling", label: "Scrolling" }
      ],
      bind: d("appearance.layout"), needs: { cap: "tiledLayout" } },

    { id: "gaps-inner", section: "gaps", label: "Between windows",
      detail: "Space between neighbouring tiled windows",
      keywords: ["gaps in", "inner gaps", "spacing", "padding"], control: "stepper", min: 0, max: 40, unit: "px",
      bind: d("appearance.gapsIn") },
    { id: "gaps-outer", section: "gaps", label: "At the screen edge",
      detail: "Space between windows and the edges of the screen",
      keywords: ["gaps out", "outer gaps", "margin", "spacing"], control: "stepper", min: 0, max: 60, unit: "px",
      bind: d("appearance.gapsOut") },
    { id: "gaps-workspaces", section: "gaps", label: "Between workspaces",
      detail: "Extra space between workspaces while you swipe from one to the next",
      keywords: ["workspace gaps", "swipe"], control: "stepper", min: 0, max: 80, unit: "px",
      bind: d("appearance.gapsWorkspaces") },

    { id: "tame-maximised", section: "new-windows", label: "Keep maximised apps in a tile",
      detail: "Apps that open maximised start as a tiled window inside your gaps",
      keywords: ["maximize", "maximise", "tame", "fullscreen on open"], control: "toggle",
      bind: d("windows.tameMaximizeOnOpen") },

    { id: "focus-follows", section: "focus", label: "Which window gets focus",
      detail: "Focus follows the pointer as it moves, or waits for a click",
      keywords: ["focus follows mouse", "follow mouse", "hover", "click to focus", "sloppy focus"],
      control: "segmented", options: focusOptions,
      bind: d("input.followMouse") },
    { id: "no-focus-fallback", section: "focus", label: "Leave focus where it was on close",
      detail: "When a window closes, the window under the pointer does not take focus",
      keywords: ["no focus fallback", "refocus"], control: "toggle",
      bind: d("appearance.noFocusFallback"), more: true },

    { id: "resize-on-border", section: "moving", label: "Drag edges to resize",
      detail: "Grab a window's edge with the pointer to resize it",
      keywords: ["resize on border", "drag", "edge"], control: "toggle",
      bind: d("appearance.resizeOnBorder") },
    { id: "border-grab", section: "moving", label: "Grab area past the edge",
      detail: "How far outside the border a drag still resizes",
      keywords: ["extend border grab area", "resize area"], control: "stepper", min: 0, max: 40, unit: "px",
      bind: d("appearance.extendBorderGrab"),
      when: { bind: d("appearance.resizeOnBorder"), is: [true] } },
    { id: "resize-cursor", section: "moving", label: "Show the resize pointer on edges",
      detail: "The pointer changes shape over a window's edge",
      keywords: ["hover icon on border", "cursor"], control: "toggle",
      bind: d("appearance.hoverIconOnBorder") },
    { id: "snap", section: "moving", label: "Snap floating windows",
      detail: "Floating windows stick to screen edges and to each other",
      keywords: ["snap", "magnet", "stick"], control: "toggle",
      bind: d("appearance.snapEnabled") },
    { id: "resize-corner", section: "moving", label: "Resize floating windows from",
      detail: "The corner a floating window grows from while you resize it",
      keywords: ["resize corner", "anchor"], control: "picker",
      options: [
          { value: 0, label: "The nearest corner" },
          { value: 1, label: "Top left" },
          { value: 2, label: "Top right" },
          { value: 3, label: "Bottom right" },
          { value: 4, label: "Bottom left" }
      ],
      bind: d("appearance.resizeCorner"), more: true }
];

var related = ["shortcuts", "app-rules", "window-style", "mouse-touchpad"];

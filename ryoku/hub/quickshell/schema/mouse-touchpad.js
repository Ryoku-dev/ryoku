.pragma library

var title = "Mouse & touchpad";
var lead = "How the pointer moves and scrolls, what the touchpad does, and what extra mouse buttons send.";

var sections = [
    { id: "mouse", title: "Mouse" },
    { id: "touchpad", title: "Touchpad", missing: "This window manager has no touchpad settings." },
    { id: "gestures", title: "Gestures", missing: "Gestures need a window manager that supports them." },
    { id: "pointer", title: "Pointer" },
    { id: "buttons", title: "Extra mouse buttons" }
];

function d(key) {
    return { store: "desktop", key: key };
}

var swiping = { bind: d("input.workspaceSwipe"), is: [true] };

// template rows feed search only; the page draws the extra buttons itself
var rows = [
    { id: "pointer-speed", section: "mouse", label: "Pointer speed",
      detail: "Slower or faster than the device's own speed, which sits in the middle. The touchpad follows it too.",
      keywords: ["sensitivity", "speed", "mouse speed", "dpi", "accel speed", "cursor speed"],
      control: "slider", min: -1, max: 1, step: 0.05, bind: d("input.sensitivity") },
    { id: "acceleration", section: "mouse", label: "Acceleration",
      detail: "Adaptive goes further on a quick flick; flat keeps the pointer in step with your hand",
      keywords: ["accel", "accel_profile", "mouse acceleration", "precision", "raw input", "gaming"],
      control: "segmented", options: [
          { label: "Default", value: "" },
          { label: "Flat", value: "flat" },
          { label: "Adaptive", value: "adaptive" }
      ], bind: d("input.accelProfile") },
    { id: "left-handed", section: "mouse", label: "Left-handed buttons",
      detail: "Swap the left and right buttons",
      keywords: ["left hand", "swap buttons", "primary button"],
      control: "toggle", bind: d("input.leftHanded") },
    { id: "mouse-natural-scroll", section: "mouse", label: "Natural scrolling",
      detail: "The page moves the way the wheel turns, as on a touchscreen",
      keywords: ["natural scroll", "reverse scroll", "invert scroll", "scroll direction", "wheel"],
      control: "toggle", bind: d("input.mouseNaturalScroll") },
    { id: "mouse-scroll-speed", section: "mouse", label: "Scroll speed",
      detail: "How far each notch of the wheel scrolls",
      keywords: ["wheel speed", "scroll factor", "scroll distance"],
      control: "slider", min: 0.2, max: 3, step: 0.1, unit: "×", bind: d("input.mouseScrollFactor") },
    { id: "middle-click-paste", section: "mouse", label: "Middle-click pastes",
      detail: "Pressing the wheel pastes the text you last selected",
      keywords: ["middle click", "primary selection", "paste", "wheel click"],
      control: "toggle", bind: d("input.middleClickPaste") },

    { id: "touchpad-on", section: "touchpad", label: "Touchpad",
      detail: "Turn the touchpad off, as the Fn touchpad key does",
      keywords: ["disable touchpad", "trackpad off", "touchpad off", "fn key"],
      control: "toggle", bind: { store: "mouse-touchpad", key: "touchpad" }, needs: { cap: "touchpadToggle" } },
    { id: "touch-natural-scroll", section: "touchpad", label: "Natural scrolling",
      detail: "Content follows your two fingers, as on a phone",
      keywords: ["natural scroll", "reverse scroll", "invert scroll", "two finger scroll", "trackpad"],
      control: "toggle", bind: d("input.naturalScroll") },
    { id: "tap-to-click", section: "touchpad", label: "Tap to click",
      detail: "A tap clicks; a two-finger tap right-clicks and three fingers middle-click",
      keywords: ["tap", "tapping", "trackpad", "right click"],
      control: "toggle", bind: d("input.tapToClick") },
    { id: "tap-and-drag", section: "touchpad", label: "Tap and drag",
      detail: "Tap, then keep the finger down to drag what you tapped",
      keywords: ["drag", "tap drag", "drag lock", "trackpad"],
      control: "toggle", bind: d("input.tapAndDrag") },
    { id: "disable-while-typing", section: "touchpad", label: "Ignore while typing",
      detail: "So a palm cannot move the pointer while you type",
      keywords: ["palm rejection", "disable while typing", "dwt", "trackpad"],
      control: "toggle", bind: d("input.disableWhileTyping") },
    { id: "click-by-fingers", section: "touchpad", label: "Click by finger count",
      detail: "Pressing the pad with one finger clicks, two right-click and three middle-click",
      keywords: ["clickfinger", "click method", "button areas", "right click", "trackpad"],
      control: "toggle", bind: d("input.clickfinger") },
    { id: "middle-emulation", section: "touchpad", label: "Middle click with both buttons",
      detail: "Pressing left and right together counts as a middle click",
      keywords: ["middle emulation", "middle click", "three button"],
      control: "toggle", bind: d("input.middleEmulation") },
    { id: "touch-scroll-speed", section: "touchpad", label: "Scroll speed",
      detail: "How far a two-finger scroll moves",
      keywords: ["scroll factor", "scroll distance", "trackpad"],
      control: "slider", min: 0.2, max: 3, step: 0.1, unit: "×", bind: d("input.touchScrollFactor") },

    { id: "workspace-swipe", section: "gestures", label: "Swipe between workspaces",
      detail: "Swipe sideways with several fingers to go to the next workspace",
      keywords: ["gesture", "workspace swipe", "three finger", "four finger", "trackpad"],
      control: "toggle", bind: d("input.workspaceSwipe") },
    { id: "swipe-fingers", section: "gestures", label: "Fingers",
      detail: "How many fingers make a workspace swipe",
      keywords: ["swipe fingers", "three finger", "four finger"],
      control: "segmented", options: [{ label: "3", value: 3 }, { label: "4", value: 4 }],
      bind: d("input.swipeFingers"), when: swiping },
    { id: "swipe-direction", section: "gestures", label: "Natural swipe direction",
      detail: "The workspaces move with your fingers",
      keywords: ["invert swipe", "swipe direction", "reverse"],
      control: "toggle", bind: d("input.swipeInvert"), when: swiping },
    { id: "swipe-new", section: "gestures", label: "Swipe past the last workspace to add one",
      detail: "Swiping past the end opens a new, empty workspace",
      keywords: ["create workspace", "new workspace"],
      control: "toggle", bind: d("input.swipeCreateNew"), when: swiping },
    { id: "swipe-distance", section: "gestures", label: "Swipe distance",
      detail: "How far to swipe for a whole switch; less switches sooner",
      keywords: ["swipe length", "gesture distance", "sensitivity"],
      control: "slider", min: 100, max: 600, step: 10, unit: "px", bind: d("input.swipeDistance"), when: swiping },

    { id: "hide-while-typing", section: "pointer", label: "Hide while typing",
      detail: "The pointer goes away when you type and comes back when you move it",
      keywords: ["hide cursor", "hide pointer", "keypress", "cursor"],
      control: "toggle", bind: d("cursor.hideOnKeyPress") },
    { id: "hide-when-still", section: "pointer", label: "Hide when still",
      detail: "Seconds without movement before the pointer hides; 0 keeps it shown",
      keywords: ["hide cursor", "inactive timeout", "idle", "cursor"],
      control: "stepper", min: 0, max: 30, unit: "s", bind: d("cursor.inactiveTimeout") },

    { id: "extra-buttons", section: "buttons", label: "Extra mouse buttons", template: true,
      detail: "Make a side or thumb button send keys, or run a sequence of keys",
      keywords: ["remap", "rebind", "mouse buttons", "side buttons", "thumb buttons", "back", "forward", "gaming mouse", "mmo", "g keys", "input-remapper"] },
    { id: "find-button", section: "buttons", label: "Find a button", template: true,
      detail: "Press a button on your mouse to choose it",
      keywords: ["detect", "which button", "press to bind"] },
    { id: "key-sequences", section: "buttons", label: "Key sequences", template: true,
      detail: "A button can run several keys in order, with pauses between them",
      keywords: ["macro", "sequence", "record", "repeat", "combo"] }
];

var related = ["fonts-icons", "window-behaviour", "keyboard", "displays"];

.pragma library

var title = "Game bar";
var lead = "The overlay its shortcut opens over a game: how it looks, where its widgets snap and settle, and the Discord voice widget.";

var sections = [
    { id: "game-bar", title: "Game bar" },
    { id: "look", title: "Look" },
    { id: "snapping", title: "Snapping" },
    { id: "guides", title: "Guide grid" },
    { id: "motion", title: "Settle motion" },
    { id: "voice", title: "Discord voice" },
    { id: "pinned", title: "Pinned voice overlay" }
];

// every key lives under gamebar in shell.json; the daemon validates it (ryoku/shell/ipc/settings.go)
// and the shell retunes the open bar at once. Fallbacks follow the shell's own defaults.
function g(key, fallback) {
    return { store: "settings", key: "gamebar." + key, fallback: fallback };
}

var on = { bind: g("enabled", true), is: [true] };
var snapOn = { bind: g("snap", true), is: [true] };
var guidesOn = { bind: g("guideGrid", true), is: [true] };

// a row that needs several switches on lists them outermost first in `whenAll`;
// its `when` is the innermost, and the page shows the first one that is off (adapt)
var rows = [
    { id: "enabled", section: "game-bar", label: "Use the game bar",
      detail: "Its shortcut opens capture, performance, music, task and voice widgets over the game you are playing; off, the shortcut does nothing",
      keywords: ["gamebar", "game overlay", "in-game", "overlay", "enable", "disable", "gaming"],
      control: "toggle", bind: g("enabled", true), "default": true },
    { id: "shortcut", section: "game-bar", label: "Game bar shortcut",
      detail: "Change the keys that open the game bar",
      keywords: ["hotkey", "keybind", "shortcut", "super shift g", "keys"],
      control: "link", link: "shortcuts/shell.gamebar", actionText: "Open" },

    { id: "dim", section: "look", label: "Background dim",
      detail: "How dark the game gets behind the open bar",
      keywords: ["darken", "scrim", "shade", "background", "dim"],
      control: "slider", min: 0, max: 1, step: 0.01, pct: true, changeLabel: "Game bar background dim",
      bind: g("dim", 0.62), "default": 0.62, when: on },
    { id: "scale", section: "look", label: "Size",
      detail: "Scales the strip, the widgets and their text together",
      keywords: ["scale", "zoom", "larger", "smaller", "text size", "widgets"],
      control: "slider", min: 0.5, max: 2, step: 0.05, pct: true, changeLabel: "Game bar size",
      bind: g("scale", 1), "default": 1, when: on },
    { id: "radius", section: "look", label: "Corner rounding",
      detail: "How rounded the strip and the widget frames are; 0 keeps them square",
      keywords: ["rounding", "corners", "radius", "shape"],
      control: "stepper", min: 0, max: 64, unit: "px", changeLabel: "Game bar corner rounding",
      bind: g("radius", 14), "default": 14, when: on },
    { id: "fade", section: "look", label: "Fade time",
      detail: "How long the bar takes to appear and to leave; 0 shows it at once",
      keywords: ["fade", "duration", "opening", "closing", "animation", "speed"],
      control: "stepper", min: 0, max: 2000, step: 50, unit: "ms", changeLabel: "Game bar fade time",
      bind: g("fade", 200), "default": 200, when: on },

    { id: "snap", section: "snapping", label: "Snap widgets into place",
      detail: "A widget you let go pulls to the screen edges, other widgets and the centre lines; off, it stays where you drop it",
      keywords: ["snap", "magnet", "align", "edges", "position", "placement"],
      control: "toggle", changeLabel: "Game bar snapping",
      bind: g("snap", true), "default": true, when: on },
    { id: "snap-reach", section: "snapping", label: "Snap distance",
      detail: "How close a widget must come to a line before it pulls to it",
      keywords: ["reach", "threshold", "pull", "magnet"],
      control: "stepper", min: 0, max: 64, unit: "px", changeLabel: "Game bar snap distance",
      bind: g("snapReach", 9), "default": 9, whenAll: [on], when: snapOn },
    { id: "snap-grid", section: "snapping", label: "Fallback grid",
      detail: "The spacing a widget falls back to when no edge is near",
      keywords: ["grid", "spacing", "step"],
      control: "stepper", min: 4, max: 128, step: 4, unit: "px", changeLabel: "Game bar fallback grid",
      bind: g("snapGrid", 24), "default": 24, whenAll: [on], when: snapOn },

    { id: "guide-grid", section: "guides", label: "Show the guide grid",
      detail: "Draw the placement grid while you drag a widget",
      keywords: ["grid lines", "guides", "drag", "placement"],
      control: "toggle", changeLabel: "Game bar guide grid",
      bind: g("guideGrid", true), "default": true, whenAll: [on], when: snapOn },
    { id: "grid-step", section: "guides", label: "Grid spacing",
      detail: "The distance between the guide lines",
      keywords: ["grid", "spacing", "lines", "density"],
      control: "stepper", min: 8, max: 256, step: 8, unit: "px", changeLabel: "Game bar grid spacing",
      bind: g("gridStep", 48), "default": 48, whenAll: [on, snapOn], when: guidesOn },
    { id: "grid-alpha", section: "guides", label: "Grid strength",
      detail: "How strongly the guide lines show over the game",
      keywords: ["opacity", "visibility", "transparency", "lines"],
      control: "slider", min: 0, max: 1, step: 0.01, pct: true, changeLabel: "Game bar grid strength",
      bind: g("gridAlpha", 0.28), "default": 0.28, whenAll: [on, snapOn], when: guidesOn },

    { id: "spring", section: "motion", label: "Stiffness",
      detail: "How hard a widget you let go is pulled into its spot; higher settles faster",
      keywords: ["spring", "snappy", "settle", "fast", "motion"],
      control: "slider", min: 0.5, max: 20, step: 0.1, changeLabel: "Game bar settle stiffness",
      bind: g("spring", 3.2), "default": 3.2, when: on },
    { id: "damping", section: "motion", label: "Damping",
      detail: "How quickly the settle stops; lower overshoots and bounces, higher is firm",
      keywords: ["bounce", "overshoot", "spring", "settle", "motion"],
      control: "slider", min: 0.05, max: 1, step: 0.01, changeLabel: "Game bar settle damping",
      bind: g("damping", 0.58), "default": 0.58, when: on },
    { id: "mass", section: "motion", label: "Weight",
      detail: "How heavy the settle feels; heavier moves slower and softer",
      keywords: ["mass", "heavy", "spring", "settle", "motion"],
      control: "slider", min: 0.1, max: 10, step: 0.1, changeLabel: "Game bar settle weight",
      bind: g("mass", 1), "default": 1, when: on },

    { id: "voice-avatars", section: "voice", label: "People shown",
      detail: "How many people in the call the voice widget shows",
      keywords: ["discord", "voice", "call", "participants", "avatars", "limit"],
      control: "stepper", min: 1, max: 8, changeLabel: "Game bar people shown",
      bind: g("voiceAvatars", 8), "default": 8, when: on },
    { id: "voice-dim", section: "voice", label: "Fade quiet people",
      detail: "Dim everyone who is not speaking so the speaker stands out",
      keywords: ["discord", "voice", "speaking", "dim", "fade", "active speaker"],
      control: "toggle", changeLabel: "Game bar fade quiet people",
      bind: g("voiceDim", true), "default": true, when: on },
    { id: "voice-dim-opacity", section: "voice", label: "Quiet people's opacity",
      detail: "How visible someone who is not speaking stays; 100% does not fade them",
      keywords: ["discord", "voice", "opacity", "fade", "transparency"],
      control: "slider", min: 0.2, max: 1, step: 0.01, pct: true, changeLabel: "Game bar quiet people's opacity",
      bind: g("voiceDimOpacity", 0.5), "default": 0.5, whenAll: [on], when: { bind: g("voiceDim", true), is: [true] } },

    { id: "voice-names", section: "pinned", label: "Names under pictures",
      detail: "Show each person's name under their picture in the pinned overlay",
      keywords: ["discord", "voice", "pinned", "names", "labels"],
      control: "toggle", changeLabel: "Game bar voice names",
      bind: g("voiceNames", true), "default": true, when: on },
    { id: "voice-avatar-size", section: "pinned", label: "Picture size",
      detail: "How large each picture is while the overlay is pinned and the game bar is closed",
      keywords: ["discord", "voice", "pinned", "avatar size", "larger", "smaller"],
      control: "stepper", min: 32, max: 72, unit: "px", changeLabel: "Game bar voice picture size",
      bind: g("voiceAvatarSize", 52), "default": 52, when: on },
    { id: "voice-bg-opacity", section: "pinned", label: "Panel opacity",
      detail: "How solid the panel behind the pinned overlay is; 0% leaves only the pictures over the game",
      keywords: ["discord", "voice", "pinned", "background", "transparency", "panel"],
      control: "slider", min: 0, max: 1, step: 0.01, pct: true, changeLabel: "Game bar voice panel opacity",
      bind: g("voiceBgOpacity", 0.72), "default": 0.72, when: on },
    { id: "voice-controls", section: "pinned", label: "Show mute and deafen",
      detail: "Show your microphone and headphone state in the pinned overlay; it never takes the pointer",
      keywords: ["discord", "voice", "pinned", "mute", "deafen", "microphone", "headphones"],
      control: "toggle", changeLabel: "Game bar mute and deafen",
      bind: g("voiceControls", true), "default": true, when: on }
];

var related = ["shortcuts", "screen-capture", "performance"];

function met(cond, value) {
    var v = value(cond.bind);
    return cond.is.some(x => JSON.stringify(x) === JSON.stringify(v));
}

// rows with each `when` set to the first of its conditions that is not met (or its own when
// all are), so a row waits on the outermost switch that is off and a deep link lands there.
// value(bind) reads a bind (Changes.value).
function adapt(rows, value) {
    return rows.map(r => {
        if (!r.whenAll)
            return r;
        var first = r.whenAll.concat([r.when]).find(c => !met(c, value));
        return first && first !== r.when ? Object.assign({}, r, { when: first }) : r;
    });
}

function row(id) {
    for (var i = 0; i < rows.length; i++)
        if (rows[i].id === id)
            return rows[i];
    return null;
}

.pragma library

// widgets.json keys are <widget prefix><suffix>. Ryoku's own widgets are
// adapter properties; widgets an installed item provides are not, so their
// keys are recognised by suffix and fall back to the defaults below.

var perMonitorSuffixes = ["Enabled", "Anchor", "X", "Y", "Scale", "Locked",
    "Size", "Width", "Height", "Depth"];

var _fallback = {
    Enabled: false,
    Anchor: "free",
    X: 120,
    Y: 120,
    Scale: 1.0,
    Locked: false,
    Size: "",
    Opacity: 1.0,
    Bg: "card",
    Color: "",
    Color2: "",
    Gradient: false,
    Style: "ryoku",
    Opts: "",
    Radius: -1,
    Pad: -1,
    Border: -1,
    BorderOpacity: -1,
    BackingOpacity: -1
};

// Longer suffixes first, so BorderOpacity never reads as Border + Opacity.
var _pattern = /^([a-z][A-Za-z0-9]*?)(BackingOpacity|BorderOpacity|Gradient|Enabled|Opacity|Locked|Anchor|Border|Color2|Height|Radius|Scale|Color|Depth|Style|Width|Opts|Size|Pad|Bg|X|Y)$/;

function split(key) {
    const match = _pattern.exec(String(key || ""));
    return match ? { prefix: match[1], suffix: match[2] } : null;
}

function fallback(key) {
    const parts = split(key);
    return parts ? _fallback[parts.suffix] : undefined;
}

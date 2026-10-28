.pragma library

// The marks the Orpheus logo can wear when it is not the system brand mark.
// Same ids and code points as QS Bar's launcher glyphs (qsbar/Theme.qml,
// launcherLogoIconOptions/launcherLogoIconCodes), so a mark picked in either
// bar is the same Nerd Font glyph; "ryoku" stays the CJK 力.

var font = "JetBrainsMono Nerd Font";
var jpFont = "Noto Sans CJK JP";

var codes = {
    "ryoku": 0x529B, "hyprland": 0xF359, "arch": 0xE732, "grid": 0xEEED, "spark": 0xE6A4, "power": 0xF011,
    "dragon": 0x2EEF, "mark": 0xEE99, "nix": 0xF313, "branch": 0xE666, "rebel": 0xF1D0, "ubuntu": 0xF31B,
    "debian": 0xF306, "fedora": 0xF30A, "gentoo": 0xF30D, "void": 0xF32E, "artix": 0xF31F, "manjaro": 0xF312,
    "suse": 0xF314, "alpine": 0xF300, "endeavour": 0xF322, "garuda": 0xF337, "cachyos": 0xF385, "freebsd": 0xF30C,
    "apple": 0xF302, "raspi": 0xF315, "elementary": 0xF309, "gnome": 0xF361, "alma": 0xF31D, "centos": 0xF304,
    "devuan": 0xF307, "arco": 0xF346, "ferris": 0xF323, "codeberg": 0xF330, "gitea": 0xF339, "tux": 0xF31A,
    "android": 0xE70E, "mint": 0xF30E, "kali": 0xF327, "popos": 0xF32A, "zorin": 0xF32F, "plasma": 0xF332,
    "wayland": 0xF367, "docker": 0xF308, "github": 0xE709, "git": 0xE702, "gitlab": 0xE7EB, "python": 0xE73C,
    "rust": 0xE7A8, "go": 0xE724, "node": 0xE719, "react": 0xE7BA, "vue": 0xE8DC, "kube": 0xE81D,
    "vim": 0xE7C5, "neovim": 0xE83A, "firefox": 0xE745, "chrome": 0xE743, "java": 0xE738, "js": 0xE781,
    "ts": 0xE8CA, "cpp": 0xE7A3, "ruby": 0xE739, "php": 0xE73D, "swift": 0xE755, "kotlin": 0xE81B,
    "lua": 0xE826, "haskell": 0xE777, "blender": 0xE766, "figma": 0xE7DA, "redis": 0xE76D, "postgres": 0xE76E,
    "steam": 0xF1B6, "spotify": 0xF1BC, "discord": 0xF1FF, "telegram": 0xF2C6, "slack": 0xF198, "empire": 0xF1D1,
    "jedi": 0xEECC, "sith": 0xEDDC, "mandalorian": 0xEDD9, "firstorder": 0xF2B0, "deathstar": 0xF08D8, "spaceinvaders": 0xF0BC9,
    "ghost": 0xEEFE, "pokeball": 0xF041D, "pokemon": 0xF0A09, "gamepad": 0xF11B, "retropad": 0xF0B82, "d20": 0xEEF5,
    "playstation": 0xED18, "xbox": 0xED3E, "switch": 0xF07E1, "minecraft": 0xF0373, "wizard": 0xEF01, "dungeon": 0xEEFA,
    "sword": 0xF04E5, "shield": 0xED25, "crown": 0xEDEB, "skull": 0xEE15, "crossbones": 0xEF0E, "ninja": 0xF0774,
    "robot": 0xEE0D, "alien": 0xF089A, "knight": 0xED63, "paw": 0xF1B0, "masks": 0xF0D02, "theater": 0xEEB6,
    "film": 0xF008, "spade": 0xF08D1, "superpowers": 0xF2DD, "reddit": 0xF1A1, "twitch": 0xF1E8
};

var ids = ["ryoku", "hyprland", "arch", "grid", "spark", "power", "dragon", "mark", "nix", "branch", "rebel", "ubuntu", "debian", "fedora", "gentoo", "void", "artix", "manjaro", "suse", "alpine", "endeavour", "garuda", "cachyos", "freebsd", "apple", "raspi", "elementary", "gnome", "alma", "centos", "devuan", "arco", "ferris", "codeberg", "gitea", "tux", "android", "mint", "kali", "popos", "zorin", "plasma", "wayland", "docker", "github", "git", "gitlab", "python", "rust", "go", "node", "react", "vue", "kube", "vim", "neovim", "firefox", "chrome", "java", "js", "ts", "cpp", "ruby", "php", "swift", "kotlin", "lua", "haskell", "blender", "figma", "redis", "postgres", "steam", "spotify", "discord", "telegram", "slack", "empire", "jedi", "sith", "mandalorian", "firstorder", "deathstar", "spaceinvaders", "ghost", "pokeball", "pokemon", "gamepad", "retropad", "d20", "playstation", "xbox", "switch", "minecraft", "wizard", "dungeon", "sword", "shield", "crown", "skull", "crossbones", "ninja", "robot", "alien", "knight", "paw", "masks", "theater", "film", "spade", "superpowers", "reddit", "twitch"];

function valid(id) {
    return typeof id === "string" && codes.hasOwnProperty(id);
}

function glyph(id) {
    return String.fromCodePoint(valid(id) ? codes[id] : codes.ryoku);
}

function family(id) {
    return !valid(id) || id === "ryoku" ? jpFont : font;
}

// Relative size per mark, 1 = the brand mark's size; some glyphs sit small
// in the font and read better a touch larger.
function scale(id) {
    if (id === "arch") return 1.06;
    if (id === "ryoku") return 0.94;
    return 1;
}

function xOffset(id) {
    if (id === "mark") return 0.5;
    if (id === "arch") return 1;
    if (id === "grid") return -1;
    return 0;
}

.pragma library

// Theme scheme catalog for the Hub Appearance page picker (COLOUR SCHEME).
//
// The two dynamic variants (Default, Wallpaper) then the 57 static themes, each with a
// 7-swatch preview projection [surface, onSurface, primary, secondary, tertiary, error,
// outline] and a dark flag (surface luma < 0.5). This is picker cosmetics ONLY: the daemon
// owns the authoritative 30-role palettes (ryoku/shell/ipc/themes_gen.go) and resolves the
// selected theme into shell.json `themePalette`, which every surface consumes. The id is the
// theme.theme value written through the settings seam -- the same key the sidebar theme
// picker (pill MenuTheme) reads and writes, so the two stay one truth on the selection.
//
// Generated from themes_gen.go (cross-checked byte-for-byte against MenuTheme.qml). Regenerate
// when the daemon catalog changes; a Go `theme` topic serving this projection would retire
// both this literal and the sidebar's.
var schemes = [
    { id: "Default", label: "Default", dynamic: true, icon: "palette" },
    { id: "Wallpaper", label: "Wallpaper", dynamic: true, icon: "wallpaper" },
    { id: "Catppuccin Mocha", label: "Catppuccin Mocha", dark: true, sw: ["#1e1e2e", "#cdd6f4", "#b4befe", "#f2cdcd", "#94e2d5", "#f38ba8", "#a6adc8"] },
    { id: "Dracula", label: "Dracula", dark: true, sw: ["#282A36", "#F8F8F2", "#BD93F9", "#FF79C6", "#8BE9FD", "#FF5555", "#6272A4"] },
    { id: "Everforest Dark Medium", label: "Everforest Dark Medium", dark: true, sw: ["#232A2E", "#D3C6AA", "#A7C080", "#7FBBB3", "#83C092", "#E67E80", "#7A8478"] },
    { id: "Gruvbox Dark Medium", label: "Gruvbox Dark Medium", dark: true, sw: ["#282828", "#EBDBB2", "#83A598", "#B8BB26", "#8EC07C", "#FB4934", "#928374"] },
    { id: "Kanagawa Wave", label: "Kanagawa Wave", dark: true, sw: ["#1F1F28", "#DCD7BA", "#7E9CD8", "#957FB8", "#7AA89F", "#E82424", "#938AA9"] },
    { id: "Nord Dark", label: "Nord Dark", dark: true, sw: ["#2E3440", "#ECEFF4", "#88C0D0", "#81A1C1", "#8FBCBB", "#BF616A", "#4C566A"] },
    { id: "One Dark", label: "One Dark", dark: true, sw: ["#282C34", "#ABB2BF", "#61AFEF", "#C678DD", "#56B6C2", "#E06C75", "#636D83"] },
    { id: "Rose Pine", label: "Ros\u00e9 Pine", dark: true, sw: ["#191724", "#E0DEF4", "#C4A7E7", "#9CCFD8", "#EBBCBA", "#EB6F92", "#6E6A86"] },
    { id: "Solarized Dark", label: "Solarized Dark", dark: true, sw: ["#002b36", "#93a1a1", "#268bd2", "#2aa198", "#b58900", "#dc322f", "#839496"] },
    { id: "Tokyo Night", label: "Tokyo Night", dark: true, sw: ["#1a1b26", "#a9b1d6", "#7aa2f7", "#bb9af7", "#73daca", "#f7768e", "#9aa5ce"] },
];

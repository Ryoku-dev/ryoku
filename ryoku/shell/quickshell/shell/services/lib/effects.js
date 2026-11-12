// The shell's own glass and shadows on its panels and the dock. The user's
// switches are shell.json theme.effects.glass and .shadows, on unless set false;
// `forced` (low power, Power Saver or Game Mode) turns both off whatever they say.
// Window style's blur and shadows in desktop.json are the window manager's alone
// and play no part here.

function userOn(settings, key) {
    var effects = settings && settings.theme && settings.theme.effects;
    return !(effects && effects[key] === false);
}

function switches(settings, forced) {
    return {
        blurDisabled: !!forced || !userOn(settings, "glass"),
        shadowsDisabled: !!forced || !userOn(settings, "shadows")
    };
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { switches };

.pragma library

// LockscreenPage as data. Generated from the page it replaces.
// Descriptions are written by hand; the inventory carries engineering
// notes, which are not user copy.

var rows = [
    {
        "tab": "",
        "group": "OTHER",
        "key": "",
        "label": "Lock skin",
        "desc": "Reskins the lock and sign-in screens",
        "ctl": "chips",
        "src": "99-ryoku.conf (contents `[Theme]\\nCurrent=ryoku\\n`). Paths overridable by env: RYOKU_SDDM_THEMES_DIR, RYOKU_SDDM_CONF, RYOKU_QYLOCK_THEMES.",
        "opts": [
            "clockwork/orbital",
            "clockwork/tape",
            "<dynamic>",
            "last-of-us",
            "windows_7",
            "pixel-*",
            "R1999*",
            "<any"
        ]
    },
    {
        "tab": "",
        "group": "OTHER",
        "key": "",
        "label": "At sign-in (keyring)",
        "desc": "How the keyring unlocks your saved passwords",
        "ctl": "chips",
        "src": "~/.config/ryoku/keyring.json (mode) and /etc/pam.d/sddm (pam_gnome_keyring). Managed by `ryoku keyring set`; $RYOKU_PAM_FILE overrides the PAM path for tests.",
        "opts": [
            "unlock-on-login",
            "never-ask",
            "ask"
        ]
    },
    {
        "tab": "",
        "group": "OTHER",
        "key": "",
        "label": "Browse RyoStore",
        "desc": "Opens the RyoStore lockscreen catalogue",
        "ctl": "action",
        "src": "ryostore open lockscreens"
    }
];

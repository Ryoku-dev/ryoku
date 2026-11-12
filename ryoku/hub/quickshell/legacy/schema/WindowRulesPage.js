.pragma library

// WindowRulesPage as data, for search only: the page itself is a bespoke list
// editor, so this one row is what lets a query reach it. The action keys are the
// values the rule's action field accepts, indexed so a search for an action
// (float, fullscreen, opacity) lands on the page.

var rows = [
    {
        "tab": "",
        "group": "RULES",
        "key": "desktop.windowRules",
        "label": "Rule editor",
        "desc": "Match a window by class or title, then apply one action",
        "ctl": "list",
        "src": "desktop.json",
        "caps": "windowRules",
        "opts": [
            "float", "tile", "pin", "fullscreen", "maximize", "center",
            "size", "move", "workspace", "opacity", "noblur", "blur", "noborder",
            "noshadow", "norounding", "nodim", "noanim", "opaque", "xray",
            "nofocus", "stayfocused", "keepaspectratio", "pseudo",
            "immediate", "idleinhibit", "suppressevent",
            "columnwidth", "minsize", "maxsize", "scrollfactor",
            "tiledstate", "babaisfloat", "blockout"
        ]
    }
];

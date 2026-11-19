import QtQuick

QtObject {
    readonly property var layouts: ({
        qwerty: [
            ["q", "w", "e", "r", "t", "y", "u", "i", "o", "p", "[", "]"],
            ["a", "s", "d", "f", "g", "h", "j", "k", "l", ";", "'"],
            ["z", "x", "c", "v", "b", "n", "m", ",", ".", "/"]
        ],
        qwertz: [
            ["q", "w", "e", "r", "t", "z", "u", "i", "o", "p", "ü", "+"],
            ["a", "s", "d", "f", "g", "h", "j", "k", "l", "ö", "ä"],
            ["y", "x", "c", "v", "b", "n", "m", ",", ".", "-"]
        ],
        azerty: [
            ["a", "z", "e", "r", "t", "y", "u", "i", "o", "p", "^", "$"],
            ["q", "s", "d", "f", "g", "h", "j", "k", "l", "m", "ù"],
            ["w", "x", "c", "v", "b", "n", ",", ";", ":", "!"]
        ],
        dvorak: [
            ["'", ",", ".", "p", "y", "f", "g", "c", "r", "l", "/", "="],
            ["a", "o", "e", "u", "i", "d", "h", "t", "n", "s", "-"],
            [";", "q", "j", "k", "x", "b", "m", "w", "v", "z"]
        ],
        colemak: [
            ["q", "w", "f", "p", "g", "j", "l", "u", "y", ";", "[", "]"],
            ["a", "r", "s", "t", "d", "h", "n", "e", "i", "o", "'"],
            ["z", "x", "c", "v", "b", "k", "m", ",", ".", "/"]
        ]
    })

    function rowsFor(layoutId) {
        return layouts[layoutId] || layouts.qwerty;
    }
}

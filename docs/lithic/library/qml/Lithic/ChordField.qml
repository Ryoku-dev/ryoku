import QtQuick
import QtQuick.Templates as T

T.Control {
    id: root

    property string chord: ""
    property string placeholder: Lithic.tr("Not set")
    property string conflict: ""
    property string accessibleName: Lithic.tr("Shortcut")
    property string previewState: ""
    readonly property bool recording: _recording || previewState === "recording"
    readonly property var keys: displayKeys(chord)

    property bool _recording: false
    readonly property bool _hoverActive: enabled && (hover.hovered || previewState === "hover")
    readonly property bool _pressActive: enabled && (tap.pressed || previewState === "pressed")
    readonly property bool _focusActive: enabled && !recording && (visualFocus || previewState === "focus")
    readonly property real _conflictHeight: conflict.length > 0
                                            ? LithicTokens.size.textFieldHelpGap + conflictLine.implicitHeight : 0
    readonly property real _capHeight: Lithic.type.label.lineHeight + LithicTokens.space.s1 * 2

    signal recorded(string chord)
    signal cleared()

    implicitWidth: Math.max(LithicTokens.size.pickerWidthMin,
                            caps.implicitWidth + LithicTokens.size.textFieldInset * 2)
    implicitHeight: LithicTokens.size.textFieldHeight + _conflictHeight
    focusPolicy: Qt.StrongFocus
    opacity: enabled ? 1 : Lithic.disabledOpacity

    Accessible.role: Accessible.Button
    Accessible.name: accessibleName
    Accessible.description: (recording ? Lithic.tr("Press a shortcut")
                             : keys.length > 0 ? keys.join(" ") : placeholder)
                            + (conflict.length > 0 ? ". " + conflict : "")

    readonly property var _modifierOrder: ["Super", "Ctrl", "Alt", "Shift"]
    // the faces wm.DisplayKeys gives, in the reader's language; the chord text stays canonical
    readonly property var _faces: ({
        "Super": Lithic.tr("Super"), "Ctrl": Lithic.tr("Ctrl"), "Alt": Lithic.tr("Alt"),
        "Shift": Lithic.tr("Shift"),
        "Return": Lithic.tr("Enter"), "Escape": Lithic.tr("Esc"), "space": Lithic.tr("Space"),
        "Space": Lithic.tr("Space"), "Prior": Lithic.tr("Page Up"), "Next": Lithic.tr("Page Down"),
        "grave": "`", "comma": ",", "bracketleft": "[", "bracketright": "]",
        "XF86AudioRaiseVolume": Lithic.tr("Vol +"), "XF86AudioLowerVolume": Lithic.tr("Vol -"),
        "XF86AudioMute": Lithic.tr("Mute"), "XF86AudioMicMute": Lithic.tr("Mic mute"),
        "XF86AudioPlay": Lithic.tr("Play"), "XF86AudioNext": Lithic.tr("Next"),
        "XF86AudioPrev": Lithic.tr("Prev"), "XF86MonBrightnessUp": Lithic.tr("Brightness +"),
        "XF86MonBrightnessDown": Lithic.tr("Brightness -")
    })

    // any spelling ("SUPER + Q", "super+q") reads as the canonical chord
    function canonical(text) {
        const mods = [];
        const rest = [];
        const parts = String(text || "").split("+");
        for (let index = 0; index < parts.length; ++index) {
            const token = parts[index].trim();
            if (token.length === 0)
                continue;
            const lower = token.toLowerCase();
            const modifier = _modifierOrder.find(name => name.toLowerCase() === lower);
            if (modifier !== undefined) {
                if (mods.indexOf(modifier) < 0)
                    mods.push(modifier);
            } else {
                rest.push(token);
            }
        }
        mods.sort((a, b) => _modifierOrder.indexOf(a) - _modifierOrder.indexOf(b));
        return mods.concat(rest).join("+");
    }

    // the key faces on the caps
    function displayKeys(text) {
        const tokens = canonical(text);
        if (tokens.length === 0)
            return [];
        return tokens.split("+").map(token => {
            if (_faces[token] !== undefined)
                return _faces[token];
            const digit = /^KP_([0-9])$/.exec(token);
            return digit ? Lithic.tr("Num %1").arg(digit[1]) : token;
        });
    }

    function _modifiers(event) {
        const mods = [];
        if (event.modifiers & Qt.MetaModifier) mods.push("Super");
        if (event.modifiers & Qt.ControlModifier) mods.push("Ctrl");
        if (event.modifiers & Qt.AltModifier) mods.push("Alt");
        if (event.modifiers & Qt.ShiftModifier) mods.push("Shift");
        return mods;
    }

    // Combos.qtKeyName from the Hub, plus the media keys and the shifted symbols Qt reports for Shift+digit
    function keyName(event) {
        const k = event.key;
        if (event.modifiers & Qt.KeypadModifier) {
            if (k >= Qt.Key_0 && k <= Qt.Key_9)
                return "KP_" + String.fromCharCode(k);
            switch (k) {
            case Qt.Key_End: return "KP_1";
            case Qt.Key_Down: return "KP_2";
            case Qt.Key_PageDown: return "KP_3";
            case Qt.Key_Left: return "KP_4";
            case Qt.Key_Clear: return "KP_5";
            case Qt.Key_Right: return "KP_6";
            case Qt.Key_Home: return "KP_7";
            case Qt.Key_Up: return "KP_8";
            case Qt.Key_PageUp: return "KP_9";
            case Qt.Key_Insert: return "KP_0";
            case Qt.Key_Delete: return "KP_Decimal";
            case Qt.Key_Enter: case Qt.Key_Return: return "KP_Enter";
            case Qt.Key_Plus: return "KP_Add";
            case Qt.Key_Minus: return "KP_Subtract";
            case Qt.Key_Asterisk: return "KP_Multiply";
            case Qt.Key_Slash: return "KP_Divide";
            case Qt.Key_Period: case Qt.Key_Comma: return "KP_Decimal";
            }
        }
        if (k >= Qt.Key_A && k <= Qt.Key_Z)
            return String.fromCharCode(k);
        if (k >= Qt.Key_0 && k <= Qt.Key_9)
            return String.fromCharCode(k);
        if (k >= Qt.Key_F1 && k <= Qt.Key_F24)
            return "F" + (k - Qt.Key_F1 + 1);
        switch (k) {
        case Qt.Key_Return: case Qt.Key_Enter: return "Return";
        case Qt.Key_Space: return "Space";
        case Qt.Key_Tab: case Qt.Key_Backtab: return "Tab";
        case Qt.Key_Left: return "Left";
        case Qt.Key_Right: return "Right";
        case Qt.Key_Up: return "Up";
        case Qt.Key_Down: return "Down";
        case Qt.Key_Backspace: return "BackSpace";
        case Qt.Key_Delete: return "Delete";
        case Qt.Key_Home: return "Home";
        case Qt.Key_End: return "End";
        case Qt.Key_PageUp: return "Prior";
        case Qt.Key_PageDown: return "Next";
        case Qt.Key_Insert: return "Insert";
        case Qt.Key_Print: return "Print";
        case Qt.Key_Minus: return "minus";
        case Qt.Key_Equal: return "equal";
        case Qt.Key_Comma: return "comma";
        case Qt.Key_Period: return "period";
        case Qt.Key_Slash: return "slash";
        case Qt.Key_Backslash: return "backslash";
        case Qt.Key_Semicolon: return "semicolon";
        case Qt.Key_Apostrophe: return "apostrophe";
        case Qt.Key_BracketLeft: return "bracketleft";
        case Qt.Key_BracketRight: return "bracketright";
        case Qt.Key_QuoteLeft: return "grave";
        case Qt.Key_VolumeUp: return "XF86AudioRaiseVolume";
        case Qt.Key_VolumeDown: return "XF86AudioLowerVolume";
        case Qt.Key_VolumeMute: return "XF86AudioMute";
        case Qt.Key_MicMute: return "XF86AudioMicMute";
        case Qt.Key_MediaPlay: case Qt.Key_MediaTogglePlayPause: return "XF86AudioPlay";
        case Qt.Key_MediaNext: return "XF86AudioNext";
        case Qt.Key_MediaPrevious: return "XF86AudioPrev";
        case Qt.Key_MonBrightnessUp: return "XF86MonBrightnessUp";
        case Qt.Key_MonBrightnessDown: return "XF86MonBrightnessDown";
        }
        if (event.modifiers & Qt.ShiftModifier) {
            switch (k) {
            case Qt.Key_Exclam: return "1";
            case Qt.Key_At: return "2";
            case Qt.Key_NumberSign: return "3";
            case Qt.Key_Dollar: return "4";
            case Qt.Key_Percent: return "5";
            case Qt.Key_AsciiCircum: return "6";
            case Qt.Key_Ampersand: return "7";
            case Qt.Key_Asterisk: return "8";
            case Qt.Key_ParenLeft: return "9";
            case Qt.Key_ParenRight: return "0";
            case Qt.Key_Underscore: return "minus";
            case Qt.Key_Plus: return "equal";
            case Qt.Key_Less: return "comma";
            case Qt.Key_Greater: return "period";
            case Qt.Key_Question: return "slash";
            case Qt.Key_Bar: return "backslash";
            case Qt.Key_Colon: return "semicolon";
            case Qt.Key_QuoteDbl: return "apostrophe";
            case Qt.Key_BraceLeft: return "bracketleft";
            case Qt.Key_BraceRight: return "bracketright";
            case Qt.Key_AsciiTilde: return "grave";
            }
        }
        return "";
    }

    function _alone(name) {
        return /^F([1-9]|1[0-9]|2[0-4])$/.test(name) || name.indexOf("XF86") === 0;
    }

    function startRecording() {
        if (!enabled)
            return;
        forceActiveFocus(Qt.OtherFocusReason);
        _recording = true;
    }

    function cancelRecording() {
        _recording = false;
    }

    onActiveFocusChanged: {
        if (!activeFocus)
            _recording = false;
    }
    onEnabledChanged: {
        if (!enabled)
            _recording = false;
    }

    Keys.onPressed: function(event) {
        if (!_recording) {
            if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter || event.key === Qt.Key_Space) {
                startRecording();
                event.accepted = true;
            }
            return;
        }
        const mods = _modifiers(event);
        // bare Tab still moves focus, which ends recording
        if ((event.key === Qt.Key_Tab || event.key === Qt.Key_Backtab)
                && (mods.length === 0 || (mods.length === 1 && mods[0] === "Shift")))
            return;
        event.accepted = true;
        if (event.isAutoRepeat)
            return;
        if (event.key === Qt.Key_Escape) {
            _recording = false;
            return;
        }
        if (mods.length === 0 && (event.key === Qt.Key_Backspace || event.key === Qt.Key_Delete)) {
            _recording = false;
            chord = "";
            cleared();
            return;
        }
        const name = keyName(event);
        if (name.length === 0 || (mods.length === 0 && !_alone(name)))
            return;
        const result = mods.concat([name]).join("+");
        _recording = false;
        chord = result;
        recorded(result);
    }

    HoverHandler {
        id: hover
        enabled: root.enabled
    }

    TapHandler {
        id: tap
        enabled: root.enabled
        onTapped: {
            if (root._recording)
                root.cancelRecording();
            else
                root.startRecording();
        }
    }

    background: Item {
        FieldSurface {
            id: surface
            width: parent.width
            height: LithicTokens.size.textFieldHeight
            hovered: root._hoverActive
            pressed: root._pressActive
            focused: root.recording
            invalid: root.conflict.length > 0

            FocusRing {
                x: -outset
                y: -outset
                width: surface.width + outset * 2
                height: surface.height + outset * 2
                keyboardFocusVisible: root._focusActive
                cut: LithicTokens.cut.mid
            }
        }

        Label {
            id: conflictLine
            visible: root.conflict.length > 0
            x: 0
            y: LithicTokens.size.textFieldHeight + LithicTokens.size.textFieldHelpGap
            width: parent.width
            text: root.conflict
            typeRole: "caption"
            ink: Lithic.jasper
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
    }

    contentItem: Item {
        Row {
            id: caps
            x: LithicTokens.size.textFieldInset
            y: (LithicTokens.size.textFieldHeight - root._capHeight) / 2
            visible: !root.recording && root.keys.length > 0
            spacing: LithicTokens.space.s1

            Repeater {
                model: root.keys
                delegate: CutRect {
                    id: cap
                    required property string modelData
                    width: capLabel.implicitWidth + LithicTokens.space.s2 * 2
                    height: root._capHeight
                    fill: Lithic.field
                    strokeColor: Lithic.edge
                    strokeWidth: LithicTokens.size.hairline
                    cut: LithicTokens.cut.token

                    Label {
                        id: capLabel
                        anchors.centerIn: parent
                        text: cap.modelData
                        typeRole: "label"
                        inkRole: "fg"
                    }
                }
            }
        }

        Label {
            x: LithicTokens.size.textFieldInset
            width: parent.width - LithicTokens.size.textFieldInset * 2
            height: LithicTokens.size.textFieldHeight
            verticalAlignment: Text.AlignVCenter
            visible: root.recording || root.keys.length === 0
            text: root.recording ? Lithic.tr("Press a shortcut") : root.placeholder
            typeRole: "body"
            inkRole: root.recording ? "fg2" : "fg3"
        }
    }
}

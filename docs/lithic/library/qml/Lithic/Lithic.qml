pragma Singleton
import QtQuick

QtObject {
    id: root

    property string mode: "follow"
    property string contrast: "standard"
    property string tint: "off"
    property bool reduceMotion: false
    readonly property bool dark: _resolved.dark
    readonly property string source: _resolved.source
    readonly property bool paletteReady: _ready

    readonly property color ground: _resolved.ground
    readonly property color stone1: _resolved.stone1
    readonly property color stone2: _resolved.stone2
    readonly property color stone3: _resolved.stone3
    readonly property color stone4: _resolved.stone4
    readonly property color slab: _resolved.slab
    readonly property color field: _resolved.field
    readonly property color vein: _resolved.vein
    readonly property color edge: _resolved.edge
    readonly property color fg: _resolved.fg
    readonly property color fg2: _resolved.fg2
    readonly property color fg3: _resolved.fg3
    readonly property color sel: _resolved.sel
    property color onSel
    property color onSel2
    readonly property color gold: _resolved.gold
    property color onGold
    readonly property color goldInk: _resolved.goldInk
    readonly property color jade: _resolved.jade
    readonly property color lapis: _resolved.lapis
    readonly property color amber: _resolved.amber
    readonly property color jasper: _resolved.jasper
    readonly property color scrim: _resolved.scrim

    readonly property var font: ({
        sans: _fontFamily(LithicTokens.font.sans),
        mono: _fontFamily(LithicTokens.font.mono),
        icons: _fontFamily(LithicTokens.font.icons)
    })

    readonly property var type: ({
        name: _typeStyle(LithicTokens.type.name),
        sentence: _typeStyle(LithicTokens.type.sentence),
        section: _typeStyle(LithicTokens.type.section),
        title: _typeStyle(LithicTokens.type.title),
        row: _typeStyle(LithicTokens.type.row),
        body: _typeStyle(LithicTokens.type.body),
        proof: _typeStyle(LithicTokens.type.proof),
        label: _typeStyle(LithicTokens.type.label),
        caption: _typeStyle(LithicTokens.type.caption),
        detail: _typeStyle(LithicTokens.type.detail),
        group: _typeStyle(LithicTokens.type.group),
        mono: _typeStyle(LithicTokens.type.mono),
        monoSm: _typeStyle(LithicTokens.type.monoSm)
    })

    readonly property var curve: [
        LithicTokens.motion.curve[0],
        LithicTokens.motion.curve[1],
        LithicTokens.motion.curve[2],
        LithicTokens.motion.curve[3],
        1,
        1
    ]

    // Temporary names used by the phase 2 component files until they are rewritten
    readonly property var sp: LithicTokens.space
    readonly property var cut: ({
        token: LithicTokens.cut.token,
        small: LithicTokens.cut.small,
        mid: LithicTokens.cut.mid,
        large: LithicTokens.cut.large,
        notch: LithicTokens.cut.notch,
        toggle: LithicTokens.cut.toggleTrack,
        toggleTrack: LithicTokens.cut.toggleTrack,
        toggleKnob: LithicTokens.cut.toggleKnob
    })
    readonly property var motion: ({
        press: duration(LithicTokens.motion.press),
        hover: duration(LithicTokens.motion.hoverIn),
        hoverExit: duration(LithicTokens.motion.hoverOut),
        select: duration(LithicTokens.motion.select),
        field: duration(LithicTokens.motion.field),
        notch: duration(LithicTokens.motion.notch),
        overlay: duration(LithicTokens.motion.dialogIn)
    })
    readonly property int easing: Easing.BezierSpline
    readonly property var easingCurve: curve

    readonly property real disabledOpacity: LithicTokens.alpha.disabled
    readonly property real pressCutGrow: LithicTokens.size.controlPressCutGrow
    readonly property real pressSink: LithicTokens.size.controlPressSink
    readonly property color focusColor: fg2
    readonly property real focusWidth: LithicTokens.size.focusRingWidth
    readonly property real focusGap: LithicTokens.size.focusRingGap

    property var _environment: ({})
    property var _named: ({})
    property var _wallpaper: ({})
    property var _injected: ({})
    property bool _injectedSet: false
    property bool _followWallpaper: true
    property color _editionAccent: "transparent"
    property bool _hasEditionAccent: false
    property bool _ready: false
    property bool _loading: false
    property var _resolved: _signature(true)

    function duration(milliseconds) {
        return reduceMotion ? 0 : milliseconds;
    }

    function stone(tone) {
        if (tone === "alive") return jade;
        if (tone === "becoming") return lapis;
        if (tone === "attention" || tone === "paused") return amber;
        if (tone === "failed") return jasper;
        return vein;
    }

    function mineralField(tone) {
        if (tone === "alive")
            return mix(stone2, jade, LithicTokens.alpha.mineralJade);
        if (tone === "becoming" || tone === "attention" || tone === "paused" || tone === "failed")
            return mix(stone2, stone(tone), LithicTokens.alpha.mineral);
        return stone3;
    }

    function mix(a, b, amount) {
        var from = Qt.color(a);
        var to = Qt.color(b);
        var t = Math.max(0, Math.min(1, Number(amount)));
        return Qt.rgba(from.r + (to.r - from.r) * t,
                       from.g + (to.g - from.g) * t,
                       from.b + (to.b - from.b) * t,
                       from.a + (to.a - from.a) * t);
    }

    function alpha(c, amount) {
        var color = Qt.color(c);
        return Qt.rgba(color.r, color.g, color.b,
                       color.a * Math.max(0, Math.min(1, Number(amount))));
    }

    function inkOn(c) {
        var background = Qt.color(c);
        var black = Qt.rgba(0, 0, 0, 1);
        var white = Qt.rgba(1, 1, 1, 1);
        return contrastRatio(black, background) >= contrastRatio(white, background)
                ? black
                : white;
    }

    function contrastRatio(a, b) {
        var aLuma = _relativeLuma(Qt.color(a));
        var bLuma = _relativeLuma(Qt.color(b));
        return (Math.max(aLuma, bLuma) + 0.05) / (Math.min(aLuma, bLuma) + 0.05);
    }

    function contrastTone(ink, background, lighter) {
        var foreground = Qt.color(ink);
        var backdrop = Qt.color(background);
        if (contrastRatio(foreground, backdrop) >= LithicTokens.contrast.minimum)
            return foreground;
        var hsl = _rgbToHsl(foreground);
        for (var step = 1; step <= 255; ++step) {
            var at = step / 255;
            var nextLightness = lighter
                    ? hsl[2] + (1 - hsl[2]) * at
                    : hsl[2] * (1 - at);
            var candidate = _hsl(hsl[0], hsl[1], nextLightness, foreground.a);
            if (contrastRatio(candidate, backdrop) >= LithicTokens.contrast.minimum)
                return candidate;
        }
        return lighter
                ? Qt.rgba(1, 1, 1, foreground.a)
                : Qt.rgba(0, 0, 0, foreground.a);
    }

    function hoverFill(fill, progress) {
        var t = progress === undefined ? 1 : progress;
        return mix(fill, Qt.rgba(1, 1, 1, 1), LithicTokens.alpha.hoverFill * t);
    }

    function hoverInk(ink, progress) {
        var t = progress === undefined ? 1 : progress;
        return alpha(ink, LithicTokens.alpha.hoverInk * t);
    }

    function hoverPlane(plane, progress) {
        var t = progress === undefined ? 1 : Math.max(0, Math.min(1, progress));
        if (Qt.colorEqual(plane, stone4))
            return mix(plane, fg, LithicTokens.alpha.hoverPlane * t);
        return mix(plane, stone3, t);
    }

    function pressRow(fill, progress) {
        var t = progress === undefined ? 1 : progress;
        return mix(fill, fg, LithicTokens.alpha.pressRow * t);
    }

    function focusCut(controlCut) {
        return controlCut + LithicTokens.size.focusCutGrow;
    }

    function environment(name) {
        return _environment[name] === undefined ? "" : _environment[name];
    }

    function setPalette(object, selectedMode) {
        if (_validMode(selectedMode))
            mode = selectedMode;
        var candidate = object;
        if (candidate && typeof candidate.themePalette === "object" && candidate.themePalette !== null)
            candidate = candidate.themePalette;
        _injected = _cleanRoles(candidate);
        _injectedSet = _roleCount(_injected) > 0;
        _resolve();
    }

    function reload() {
        _injected = {};
        _injectedSet = false;
        _load();
    }

    onModeChanged: {
        if (!_validMode(mode)) {
            mode = "follow";
            return;
        }
        if (!_loading)
            _resolve();
    }

    onContrastChanged: {
        if (contrast !== "standard" && contrast !== "high") {
            contrast = "standard";
            return;
        }
        if (!_loading)
            _resolve();
    }

    onTintChanged: {
        if (tint !== "off" && tint !== "low" && tint !== "medium") {
            tint = "off";
            return;
        }
        if (!_loading)
            _resolve();
    }

    function _fontFamily(candidates) {
        var installed = Qt.fontFamilies();
        for (var i = 0; i < candidates.length; ++i) {
            if (installed.indexOf(candidates[i]) !== -1)
                return candidates[i];
        }
        return candidates.length > 0 ? candidates[0] : "";
    }

    function _typeStyle(token) {
        return {
            family: font[token.family] || font.sans,
            pixelSize: token.size,
            weight: token.weight,
            lineHeight: token.line
        };
    }

    function _validMode(candidate) {
        return candidate === "follow" || candidate === "dark" || candidate === "light";
    }

    function _readText(path) {
        if (!path)
            return "";
        try {
            var request = new XMLHttpRequest();
            request.open("GET", "file://" + encodeURI(path), false);
            request.send();
            return request.status === 0 || request.status === 200 ? request.responseText : "";
        } catch (error) {
            return "";
        }
    }

    function _readObject(path) {
        var text = _readText(path);
        if (!text)
            return null;
        try {
            var parsed = JSON.parse(text);
            return parsed && typeof parsed === "object" ? parsed : null;
        } catch (error) {
            return null;
        }
    }

    function _readEnvironment() {
        var text = _readText("/proc/self/environ");
        var result = {};
        var entries = text.split("\u0000");
        for (var i = 0; i < entries.length; ++i) {
            var separator = entries[i].indexOf("=");
            if (separator > 0)
                result[entries[i].slice(0, separator)] = entries[i].slice(separator + 1);
        }
        return result;
    }

    function _isColor(value) {
        return typeof value === "string" && /^#[0-9a-fA-F]{6}([0-9a-fA-F]{2})?$/.test(value);
    }

    function _cleanRoles(object) {
        var result = {};
        if (!object || typeof object !== "object")
            return result;
        for (var key in object) {
            if (_isColor(object[key]))
                result[key] = object[key];
        }
        return result;
    }

    function _roleCount(roles) {
        var count = 0;
        for (var key in roles)
            ++count;
        return count;
    }

    function _parseEdition(text) {
        var values = {};
        var lines = text.split("\n");
        for (var i = 0; i < lines.length; ++i) {
            var line = lines[i].trim();
            if (!line || line[0] === "#")
                continue;
            var separator = line.indexOf("=");
            if (separator < 1)
                continue;
            var key = line.slice(0, separator).trim();
            var value = line.slice(separator + 1).trim();
            if ((value[0] === "\"" && value[value.length - 1] === "\"")
                    || (value[0] === "'" && value[value.length - 1] === "'"))
                value = value.slice(1, -1);
            values[key] = value;
        }
        var ids = [values.ID || ""].concat((values.ID_LIKE || "").split(/\s+/));
        for (var index = 0; index < ids.length; ++index) {
            var edition = ids[index].toLowerCase();
            var colorValue = LithicTokens.edition[edition];
            if (_isColor(colorValue))
                return Qt.color(colorValue);
        }
        return Qt.rgba(0, 0, 0, 0);
    }

    function _signature(isDark) {
        var signature = isDark ? LithicTokens.signature.dark : LithicTokens.signature.light;
        var minerals = isDark ? LithicTokens.minerals.dark : LithicTokens.minerals.light;
        return {
            ground: signature.ground,
            stone1: signature.stone1,
            stone2: signature.stone2,
            stone3: signature.stone3,
            stone4: signature.stone4,
            slab: signature.slab,
            field: signature.field,
            vein: signature.vein,
            edge: _mixBytes(signature.vein, signature.fg3, LithicTokens.alpha.edge),
            fg: signature.fg,
            fg2: signature.fg2,
            fg3: signature.fg3,
            sel: signature.sel,
            onSel: signature.onSel,
            onSel2: signature.onSel2,
            gold: signature.gold,
            onGold: signature.onGold,
            goldInk: signature.goldInk,
            jade: minerals.jade,
            lapis: minerals.lapis,
            amber: minerals.amber,
            jasper: signature.jasper,
            scrim: signature.scrim,
            dark: isDark,
            source: "signature"
        };
    }

    function _color(roles, key, fallback) {
        return _isColor(roles[key]) ? Qt.color(roles[key]) : fallback;
    }

    function _mappedColor(roles, mapping, role, fallback) {
        return _color(roles, mapping[role], fallback);
    }

    // The look and the derived roles work on whole bytes rounded half up, like the
    // Go and web resolvers. Qt keeps 16 bits per channel, so a byte survives
    function _bytes(c) {
        var color = Qt.color(c);
        return {
            r: Math.round(color.r * 255),
            g: Math.round(color.g * 255),
            b: Math.round(color.b * 255),
            a: Math.round(color.a * 255)
        };
    }

    function _fromBytes(channels) {
        return Qt.rgba(channels.r / 255, channels.g / 255, channels.b / 255, channels.a / 255);
    }

    function _mixBytes(a, b, amount) {
        var from = _bytes(a);
        var to = _bytes(b);
        return _fromBytes({
            r: Math.round(from.r + (to.r - from.r) * amount),
            g: Math.round(from.g + (to.g - from.g) * amount),
            b: Math.round(from.b + (to.b - from.b) * amount),
            a: Math.round(from.a + (to.a - from.a) * amount)
        });
    }

    // Moves c away from ground by factor per channel, clamped to a byte
    function _away(c, ground, factor) {
        var color = _bytes(c);
        var from = _bytes(ground);
        function channel(key) {
            return Math.min(Math.max(Math.round(from[key] + factor * (color[key] - from[key])), 0), 255);
        }
        return _fromBytes({ r: channel("r"), g: channel("g"), b: channel("b"), a: color.a });
    }

    // The look's contrast step, then its tint step. tintRole is null when the
    // palette has no tint role
    function _applyLook(result, tintRole) {
        if (contrast === "high") {
            var factor = 1 + LithicTokens.alpha.contrastHigh;
            var spread = ["stone1", "stone2", "stone3", "stone4", "slab", "field", "vein"];
            for (var i = 0; i < spread.length; ++i)
                result[spread[i]] = _away(result[spread[i]], result.ground, factor);
            result.fg3 = _mixBytes(result.fg3, result.fg, LithicTokens.alpha.contrastHighFg3);
        }
        var amount = tint === "low"
                ? LithicTokens.alpha.tintLow
                : tint === "medium" ? LithicTokens.alpha.tintMedium : 0;
        if (tintRole === null || amount === 0)
            return;
        var tinted = ["ground", "stone1", "stone2", "stone3", "stone4", "slab", "field"];
        for (var j = 0; j < tinted.length; ++j)
            result[tinted[j]] = _mixBytes(result[tinted[j]], tintRole, amount);
    }

    function _luma(c) {
        return 0.299 * c.r + 0.587 * c.g + 0.114 * c.b;
    }

    function _relativeLuma(c) {
        function channel(value) {
            return value <= 0.04045
                    ? value / 12.92
                    : Math.pow((value + 0.055) / 1.055, 2.4);
        }
        return 0.2126 * channel(c.r) + 0.7152 * channel(c.g) + 0.0722 * channel(c.b);
    }

    function _rgbToHsl(c) {
        var highest = Math.max(c.r, c.g, c.b);
        var lowest = Math.min(c.r, c.g, c.b);
        var lightness = (highest + lowest) / 2;
        var delta = highest - lowest;
        if (delta === 0)
            return [0, 0, lightness];
        var saturation = delta / (1 - Math.abs(2 * lightness - 1));
        var hue;
        if (highest === c.r)
            hue = ((c.g - c.b) / delta) % 6;
        else if (highest === c.g)
            hue = (c.b - c.r) / delta + 2;
        else
            hue = (c.r - c.g) / delta + 4;
        hue /= 6;
        if (hue < 0)
            hue += 1;
        return [hue, saturation, lightness];
    }

    function _hsl(hue, saturation, lightness, alphaValue) {
        var chroma = (1 - Math.abs(2 * lightness - 1)) * saturation;
        var x = chroma * (1 - Math.abs((hue * 6) % 2 - 1));
        var offset = lightness - chroma / 2;
        var red = 0;
        var green = 0;
        var blue = 0;
        var sector = Math.floor(hue * 6);
        if (sector === 0) {
            red = chroma;
            green = x;
        } else if (sector === 1) {
            red = x;
            green = chroma;
        } else if (sector === 2) {
            green = chroma;
            blue = x;
        } else if (sector === 3) {
            green = x;
            blue = chroma;
        } else if (sector === 4) {
            red = x;
            blue = chroma;
        } else {
            red = chroma;
            blue = x;
        }
        return Qt.rgba(red + offset, green + offset, blue + offset, alphaValue);
    }

    function _combined() {
        if (_injectedSet)
            return { roles: _injected, source: "host" };
        var roles = {};
        var sourceName = "signature";
        if (_followWallpaper || mode !== "follow") {
            for (var wallpaperKey in _wallpaper)
                roles[wallpaperKey] = _wallpaper[wallpaperKey];
            if (_roleCount(_wallpaper) > 0)
                sourceName = "wallpaper";
        }
        for (var namedKey in _named)
            roles[namedKey] = _named[namedKey];
        if (_roleCount(_named) > 0)
            sourceName = "scheme";
        return { roles: roles, source: sourceName };
    }

    function _publish(result) {
        _resolved = result;
        onSel = result.onSel;
        onSel2 = result.onSel2;
        onGold = result.onGold;
    }

    function _resolve() {
        var combined = _combined();
        var roles = combined.roles;
        var forced = mode !== "follow";
        var isDark = mode !== "light";
        if (!forced) {
            isDark = true;
            if (_isColor(roles.surface))
                isDark = _luma(Qt.color(roles.surface)) <= 0.5;
        }

        var result = _signature(isDark);
        result.source = combined.source;
        var goldInk = result.goldInk;
        var tintRole = null;
        if (combined.source === "signature") {
            if (_hasEditionAccent) {
                result.gold = _editionAccent;
                result.onGold = inkOn(_editionAccent);
                goldInk = _editionAccent;
            }
        } else {
            var accents = LithicTokens.palette.accents;
            if (!forced) {
                var neutralMap = isDark ? LithicTokens.palette.dark : LithicTokens.palette.light;
                var neutrals = ["ground", "stone1", "stone2", "stone3", "stone4", "slab", "field", "vein", "fg", "fg2", "fg3"];
                for (var i = 0; i < neutrals.length; ++i)
                    result[neutrals[i]] = _mappedColor(roles, neutralMap, neutrals[i], Qt.color(result[neutrals[i]]));
            }
            var scrimKey = accents.scrim;
            if (_isColor(roles[scrimKey]))
                result.scrim = alpha(Qt.color(roles[scrimKey]), isDark
                                     ? LithicTokens.alpha.scrimDark
                                     : LithicTokens.alpha.scrimLight);

            var toned = LithicTokens.palette.toned;
            result.gold = _mappedColor(roles, accents, "gold", Qt.color(result.gold));
            result.onGold = _mappedColor(roles, accents, "onGold", Qt.color(result.onGold));
            goldInk = _mappedColor(roles, accents, toned.goldInk, result.gold);
            if (_isColor(roles[accents.sel])) {
                result.sel = _isColor(roles[accents.selMix])
                        ? _mixBytes(roles[accents.sel], roles[accents.selMix], LithicTokens.alpha.selMix)
                        : Qt.color(roles[accents.sel]);
            }
            result.onSel = _mappedColor(roles, accents, "onSel", Qt.color(result.onSel));
            result.onSel2 = _mixBytes(result.onSel, result.sel, LithicTokens.alpha.onSel2);
            result.jasper = _mappedColor(roles, accents, toned.jasper, Qt.color(result.jasper));
            if (_isColor(roles[accents.tint]))
                tintRole = Qt.color(roles[accents.tint]);
        }

        _applyLook(result, tintRole);
        result.edge = _mixBytes(result.vein, result.fg3, LithicTokens.alpha.edge);
        result.goldInk = contrastTone(goldInk, result.stone2, isDark);
        result.jasper = contrastTone(result.jasper, result.stone2, isDark);
        result.dark = isDark;
        _publish(result);
    }

    function _load() {
        _loading = true;
        _ready = false;
        _environment = _readEnvironment();
        var environmentMode = environment("LITHIC_MODE");
        if (_validMode(environmentMode))
            mode = environmentMode;

        var home = environment("HOME") || "";
        var configHome = environment("XDG_CONFIG_HOME") || (home + "/.config");
        var cacheHome = environment("XDG_CACHE_HOME") || (home + "/.cache");
        var shell = _readObject(configHome + "/ryoku/shell.json");
        _named = shell && typeof shell.themePalette === "object" && shell.themePalette !== null
                ? _cleanRoles(shell.themePalette)
                : {};
        var theme = _readObject(configHome + "/ryoku/theme.json");
        _followWallpaper = theme && typeof theme.followWallpaper === "boolean"
                ? theme.followWallpaper
                : true;
        _wallpaper = _cleanRoles(_readObject(cacheHome + "/ryoku/colors.json"));
        _editionAccent = _parseEdition(_readText("/etc/os-release"));
        _hasEditionAccent = _editionAccent.a > 0;

        var overridePath = environment("LITHIC_PALETTE_JSON");
        if (overridePath) {
            var overrideObject = _readObject(overridePath);
            if (overrideObject && typeof overrideObject.themePalette === "object")
                overrideObject = overrideObject.themePalette;
            _injected = _cleanRoles(overrideObject);
            _injectedSet = _roleCount(_injected) > 0;
        }
        _loading = false;
        _resolve();
        _ready = true;
    }

    Component.onCompleted: _load()
}

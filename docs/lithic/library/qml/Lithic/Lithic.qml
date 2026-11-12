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

    // the one copy of the faces sits in the Go library, which embeds it
    readonly property list<QtObject> _faces: [
        FontLoader { source: "../../go/fonts/Geologica-Regular.ttf" },
        FontLoader { source: "../../go/fonts/Geologica-Medium.ttf" },
        FontLoader { source: "../../go/fonts/Geologica-Bold.ttf" },
        FontLoader { source: "../../go/fonts/MartianMono-Regular.ttf" },
        FontLoader { source: "../../go/fonts/MartianMono-Bold.ttf" }
    ]

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

    // a host's translator turns the library's own English words into the reader's language
    property var translator: null

    function tr(text) {
        return translator ? translator(text) : text;
    }

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
        if (tone === "paused")
            return _resolved.mineralFields.attention;
        if (tone === "alive" || tone === "becoming" || tone === "attention" || tone === "failed")
            return _resolved.mineralFields[tone];
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
        return _rolesFrom({}, "signature", isDark, true, null);
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

    function _linearChannel(value) {
        return value <= 0.04045 ? value / 12.92 : Math.pow((value + 0.055) / 1.055, 2.4);
    }

    // The OKLCH chroma and hue of a colour: what a palette colour dyes with
    function _dyeOf(c) {
        var bytes = _bytes(c);
        var r = _linearChannel(bytes.r / 255);
        var g = _linearChannel(bytes.g / 255);
        var b = _linearChannel(bytes.b / 255);
        var l = Math.cbrt(0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * b);
        var m = Math.cbrt(0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * b);
        var s = Math.cbrt(0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * b);
        var a = 1.9779984951 * l - 2.4285922050 * m + 0.4505937099 * s;
        var bb = 0.0259040371 * l + 0.7827717662 * m - 0.8086757660 * s;
        var hue = Math.atan2(bb, a) * 180 / Math.PI;
        if (hue < 0)
            hue += 360;
        return { c: Math.hypot(a, bb), h: hue };
    }

    function _linearFromOklch(lightness, chroma, hue) {
        var a = chroma * Math.cos(hue * Math.PI / 180);
        var bb = chroma * Math.sin(hue * Math.PI / 180);
        var lc = lightness + 0.3963377774 * a + 0.2158037573 * bb;
        var mc = lightness - 0.1055613458 * a - 0.0638541728 * bb;
        var sc = lightness - 0.0894841775 * a - 1.2914855480 * bb;
        var l = lc * lc * lc;
        var m = mc * mc * mc;
        var s = sc * sc * sc;
        return [4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s,
                -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s,
                -0.0041960863 * l - 0.7034186147 * m + 1.7076147010 * s];
    }

    function _inGamut(lightness, chroma, hue) {
        var channels = _linearFromOklch(lightness, chroma, hue);
        for (var i = 0; i < 3; ++i) {
            if (channels[i] < -1e-7 || channels[i] > 1 + 1e-7)
                return false;
        }
        return true;
    }

    // Paints a role at a ladder lightness with a dye, keeping as much of the
    // dye's chroma as sRGB can show at that lightness
    function _toned(lightness, dye) {
        var chroma = dye.c;
        if (!_inGamut(lightness, chroma, dye.h)) {
            var low = 0;
            var high = chroma;
            for (var step = 0; step < 24; ++step) {
                var mid = (low + high) / 2;
                if (_inGamut(lightness, mid, dye.h))
                    low = mid;
                else
                    high = mid;
            }
            chroma = low;
        }
        var channels = _linearFromOklch(lightness, chroma, dye.h);
        function encode(value) {
            var x = Math.min(Math.max(value, 0), 1);
            var gamma = x <= 0.0031308 ? x * 12.92 : 1.055 * Math.pow(x, 1 / 2.4) - 0.055;
            return Math.floor(gamma * 255 + 0.5);
        }
        return _fromBytes({ r: encode(channels[0]), g: encode(channels[1]), b: encode(channels[2]), a: 255 });
    }

    function _accentDye(c) {
        var dye = _dyeOf(c);
        return { c: Math.min(LithicTokens.dye.chroma.gold, dye.c * LithicTokens.dye.boost.gold), h: dye.h };
    }

    // Without a palette Lithic paints its signature: the ryoku.dev stock (black
    // paper, bone ink, the sun as the one action), with an edition's signal in
    // place of the sun (edition is the os-release accent or null). A palette
    // only dyes: Lithic owns every lightness (the tone ladder for the mode and
    // contrast), and the palette lends hue and chroma, neutrals from its
    // surface, ink from its on-surface, the action from its primary, the
    // selection from its containers and jasper from its error. A forced mode
    // keeps the signature's neutrals.
    function _rolesFrom(roles, sourceName, isDark, forced, edition) {
        var ladderKey = (isDark ? "dark" : "light") + (contrast === "high" ? "High" : "");
        var steps = LithicTokens.tone[ladderKey];
        var table = LithicTokens.signature[ladderKey];
        var dye = LithicTokens.dye;
        var fallback = dye.fallback;
        var accents = LithicTokens.palette.accents;
        var result = { dark: isDark, source: sourceName };
        var neutralRoles = ["ground", "stone1", "stone2", "stone3", "stone4", "slab", "field", "vein", "edge", "fg", "fg2", "fg3", "scrim"];
        for (var n = 0; n < neutralRoles.length; ++n)
            result[neutralRoles[n]] = Qt.color(table[neutralRoles[n]]);

        if (sourceName === "signature") {
            var accentRoles = ["sel", "onSel", "onSel2", "gold", "onGold", "goldInk", "jasper"];
            for (var a = 0; a < accentRoles.length; ++a)
                result[accentRoles[a]] = Qt.color(table[accentRoles[a]]);
            if (edition !== null) {
                result.gold = edition;
                result.goldInk = edition;
                result.onGold = inkOn(edition);
            }
            return _withMinerals(result, steps);
        }

        if (!forced) {
            var neutral = { c: fallback.stoneChroma, h: fallback.neutralHue };
            var groundChroma = fallback.groundChroma;
            var ink = { c: fallback.inkChroma, h: fallback.neutralHue };
            if (_isColor(roles[LithicTokens.palette.dye.neutral])) {
                var surfaceDye = _dyeOf(roles[LithicTokens.palette.dye.neutral]);
                var factor = tint === "low" ? dye.tint.low : tint === "medium" ? dye.tint.medium : 1;
                neutral = { c: Math.min(dye.chroma.stone, surfaceDye.c) * factor, h: surfaceDye.h };
                groundChroma = Math.min(dye.chroma.ground, surfaceDye.c * dye.boost.ground) * factor;
            }
            if (_isColor(roles[LithicTokens.palette.dye.ink])) {
                var inkDye = _dyeOf(roles[LithicTokens.palette.dye.ink]);
                ink = { c: Math.min(dye.chroma.ink, inkDye.c), h: inkDye.h };
            }
            var vein = { c: neutral.c * dye.boost.vein, h: neutral.h };
            result.ground = _toned(steps.ground, { c: groundChroma, h: neutral.h });
            var planes = ["stone1", "stone2", "stone3", "stone4", "slab", "field"];
            for (var p = 0; p < planes.length; ++p)
                result[planes[p]] = _toned(steps[planes[p]], neutral);
            result.vein = _toned(steps.vein, vein);
            result.edge = _toned(steps.edge, vein);
            result.fg = _toned(steps.fg, ink);
            result.fg2 = _toned(steps.fg2, ink);
            result.fg3 = _toned(steps.fg3, ink);
        }

        var gold = { c: fallback.goldChroma, h: fallback.goldHue };
        var jasper = { c: dye.chroma.jasper, h: fallback.jasperHue };
        var sel = { c: fallback.selChroma, h: fallback.neutralHue };
        if (_isColor(roles[accents.scrim]))
            result.scrim = _fromBytes(_bytes(alpha(Qt.color(roles[accents.scrim]), isDark
                                                   ? LithicTokens.alpha.scrimDark
                                                   : LithicTokens.alpha.scrimLight)));
        if (_isColor(roles[accents.gold]))
            gold = _accentDye(roles[accents.gold]);
        if (_isColor(roles[accents.sel])) {
            var container = _isColor(roles[accents.selMix])
                    ? _mixBytes(roles[accents.sel], roles[accents.selMix], LithicTokens.alpha.selMix)
                    : Qt.color(roles[accents.sel]);
            var containerDye = _dyeOf(container);
            sel = { c: Math.min(dye.chroma.sel, containerDye.c), h: containerDye.h };
        }
        if (_isColor(roles[accents.jasper]))
            jasper.h = _dyeOf(roles[accents.jasper]).h;
        result.gold = _toned(steps.gold, gold);
        result.goldInk = _toned(steps.goldInk, gold);
        result.onGold = _toned(steps.onGold, { c: dye.chroma.onGold, h: gold.h });
        result.jasper = _toned(steps.jasper, jasper);
        result.sel = _toned(steps.sel, sel);
        result.onSel = _toned(steps.onSel, { c: dye.chroma.onSel, h: sel.h });
        result.onSel2 = _mixBytes(result.onSel, result.sel, LithicTokens.alpha.onSel2);
        return _withMinerals(result, steps);
    }

    // The fixed state minerals and the fields they dye on this ladder
    function _withMinerals(result, steps) {
        var minerals = result.dark ? LithicTokens.minerals.dark : LithicTokens.minerals.light;
        result.jade = Qt.color(minerals.jade);
        result.lapis = Qt.color(minerals.lapis);
        result.amber = Qt.color(minerals.amber);
        var pairs = [["alive", "jade"], ["becoming", "lapis"], ["attention", "amber"], ["failed", "jasper"]];
        var fields = {};
        for (var i = 0; i < pairs.length; ++i)
            fields[pairs[i][0]] = _toned(steps.mineralField, { c: LithicTokens.dye.mineral[pairs[i][1]], h: _dyeOf(result[pairs[i][1]]).h });
        result.mineralFields = fields;
        return result;
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

        _publish(_rolesFrom(roles, combined.source, isDark, forced, _hasEditionAccent ? _editionAccent : null));
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

import QtQuick

Text {
    id: root

    property string role: "body"
    property string tone: role === "micro" || role === "tag" ? "muted" : "ink"
    property bool numeric: false
    property bool strong: false

    readonly property bool technical: role === "micro" || role === "tag"

    function roleSize(name) {
        switch (name) {
        case "bar": return Style.barText;
        case "barSub": return Style.barSub;
        case "caption": return Style.caption;
        case "label": return Style.label;
        case "title": return Style.title;
        case "headline": return Style.headline;
        case "display": return Style.displaySize;
        case "micro": return Style.micro;
        case "tag": return Style.tiny;
        default: return Style.body;
        }
    }

    function roleWeight(name) {
        switch (name) {
        case "bar":
        case "label":
        case "bodyStrong": return Font.Medium;
        case "title":
        case "headline": return Font.DemiBold;
        case "display": return Font.Light;
        default: return Font.Normal;
        }
    }

    function stronger(weight) {
        if (weight >= Font.DemiBold)
            return Font.Bold;
        if (weight >= Font.Medium)
            return Font.DemiBold;
        if (weight <= Font.Light)
            return Font.Normal;
        return Font.Medium;
    }

    function toneColor(name) {
        switch (name) {
        case "variant":
        case "dim": return Style.inkDim;
        case "muted": return Style.inkMuted;
        case "faint": return Style.inkFaint;
        case "onBone": return Style.inkOnBone;
        case "alert": return Style.alert;
        case "accent": return Style.accent;
        default: return Style.ink;
        }
    }

    color: toneColor(tone)
    font.family: technical ? Style.mono : Style.font
    font.pixelSize: roleSize(role)
    font.weight: strong ? stronger(roleWeight(role)) : roleWeight(role)
    font.features: numeric ? Style.tabular : ({})
    font.capitalization: technical ? Font.AllUppercase : Font.MixedCase
    font.letterSpacing: technical ? Style.track : 0
    renderType: font.pixelSize < 20 ? Text.NativeRendering : Text.QtRendering
    elide: wrapMode === Text.NoWrap ? Text.ElideRight : Text.ElideNone
    maximumLineCount: wrapMode === Text.NoWrap ? 1 : 2147483647
}

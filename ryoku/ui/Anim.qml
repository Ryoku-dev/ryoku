import QtQuick
import Ryoku.Ui.Singletons

// Material 3 motion primitive: pick a `type` and it carries that role's duration
// and easing curve from Tokens (so global speed and reduce-motion reach it via
// Tokens.dur()). Ported from caelestia-dots/shell (components/Anim.qml).
NumberAnimation {
    id: anim

    enum Role {
        StandardSmall,
        Standard,
        StandardLarge,
        StandardExtraLarge,
        EmphasizedSmall,
        Emphasized,
        EmphasizedLarge,
        EmphasizedExtraLarge,
        FastSpatial,
        DefaultSpatial,
        SlowSpatial,
        FastEffects,
        DefaultEffects,
        SlowEffects
    }

    property int type: Anim.Standard

    readonly property var _durations: [
        Tokens.durSmall, Tokens.durNormal, Tokens.durLarge, Tokens.durXl,
        Tokens.durSmall, Tokens.durNormal, Tokens.durLarge, Tokens.durXl,
        Tokens.durFastSpatial, Tokens.durDefaultSpatial, Tokens.durSlowSpatial,
        Tokens.durFastEffects, Tokens.durDefaultEffects, Tokens.durSlowEffects
    ]
    readonly property var _curves: [
        Tokens.curveStandard, Tokens.curveStandard, Tokens.curveStandard, Tokens.curveStandard,
        Tokens.curveEmphasized, Tokens.curveEmphasized, Tokens.curveEmphasized, Tokens.curveEmphasized,
        Tokens.curveFastSpatial, Tokens.curveDefaultSpatial, Tokens.curveSlowSpatial,
        Tokens.curveFastEffects, Tokens.curveDefaultEffects, Tokens.curveSlowEffects
    ]

    duration: _durations[type] !== undefined ? _durations[type] : Tokens.durNormal
    easing.type: Easing.BezierSpline
    easing.bezierCurve: _curves[type] !== undefined ? _curves[type] : Tokens.curveStandard
}

#include "handlayout.h"

#include "cardsource.h"

#include <algorithm>
#include <cmath>

using namespace handrig;

namespace {

void setF4(float *dst, float a, float b, float c, float d)
{
    dst[0] = a;
    dst[1] = b;
    dst[2] = c;
    dst[3] = d;
}

float clampf(float v, float lo, float hi) { return v < lo ? lo : v > hi ? hi : v; }

float handSlotN(int slot, int len) { return float(slot) - (float(std::max(len, 1)) - 1.0f) * 0.5f; }

const QString kPrefix = QStringLiteral("components.wallpaperSelector.");

}  // namespace

HandLayout::HandLayout()
{
    m_rigX = Spring::forDuration(0.0, 450.0, 0.75);
    m_rigY = Spring::forDuration(0.0, 450.0, 0.75);
}

void HandLayout::readParams(const ParamSource *src, Params &out) const
{
    if (!src)
        return;
    const auto num = [&](const char *k, double fb) { return src->num(kPrefix + QLatin1String(k), fb); };
    const auto flag = [&](const char *k, bool fb) { return src->flag(kPrefix + QLatin1String(k), fb); };
    const auto text = [&](const char *k, const char *fb) {
        return src->text(kPrefix + QLatin1String(k), QLatin1String(fb));
    };

    out.offsetX = clampf(float(num("handStageX", 0) / 100.0), -1.0f, 1.0f);
    out.offsetY = clampf(float(num("handStageY", 0) / 100.0), -1.0f, 1.0f);
    out.count = int(std::lround(std::clamp(num("handCount", 5), 2.0, 16.0)));
    out.cardW = clampf(float(num("handCardWidth", 168)), 40.0f, 4096.0f);
    out.cardH = clampf(float(num("handCardHeight", 432)), 60.0f, 4096.0f);
    out.spread = clampf(float(num("handSpread", 126)), 10.0f, 600.0f);
    out.ribbons = int(std::lround(std::clamp(num("handRibbons", 6), 2.0, 14.0)));
    out.fanAngle = clampf(float(num("handFanAngle", 12)), -60.0f, 60.0f);
    out.fanRoll = clampf(float(num("handFanRoll", 8.5)), -45.0f, 45.0f);
    out.arch = clampf(float(num("handArch", 20)), -200.0f, 200.0f);
    out.radius = clampf(float(num("handCornerRadius", 0)), 0.0f, 200.0f);
    out.skew = clampf(float(num("handSkew", 0)), -200.0f, 200.0f);
    out.blur = clampf(float(num("handBackdropBlur", 100) / 100.0), 0.0f, 4.0f);
    out.speed = clampf(float(num("handSpeed", 100) / 100.0), 0.35f, 2.5f);
    out.tilt = clampf(float(num("handTilt", 100) / 100.0), 0.0f, 3.0f);
    out.perspective = clampf(float(num("handPerspective", 1700)), 400.0f, 6000.0f);
    out.axis = axisFromKey(text("handRibbonAxis", "rows"));
    out.cut = cutFromKey(text("handCut", "straight"));
    out.variance = varianceFromKey(text("handCutVariance", "none"));
    out.dealMode = dealModeFromKey(text("handMove", "cycle"));
    out.moves[0] = flag("handMoveCorkscrew", true);
    out.moves[1] = flag("handMoveCascade", true);
    out.moves[2] = flag("handMoveShuffle", true);
    out.moves[3] = flag("handMoveRibbon", true);
    out.moves[4] = flag("handMoveSpiral", true);
    out.ghosts = flag("handGhosts", true);
    out.bob = flag("handBob", false);
    out.backdrop = flag("handBackdrop", true);
    out.revealFill = flag("handRevealFill", true);
}

void HandLayout::morph(const LayoutContext &ctx, double dt)
{
    if (!m_haveParams)
        return;
    const float amt = float(approachK(dt, ctx.motion->tau(MotionProfile::Standard)));
    m_params.offsetX = lerpf(m_params.offsetX, m_target.offsetX, amt);
    m_params.offsetY = lerpf(m_params.offsetY, m_target.offsetY, amt);
    m_params.cardW = lerpf(m_params.cardW, m_target.cardW, amt);
    m_params.cardH = lerpf(m_params.cardH, m_target.cardH, amt);
    m_params.spread = lerpf(m_params.spread, m_target.spread, amt);
    m_params.perspective = lerpf(m_params.perspective, m_target.perspective, amt);
    m_params.fanAngle = lerpf(m_params.fanAngle, m_target.fanAngle, amt);
    m_params.fanRoll = lerpf(m_params.fanRoll, m_target.fanRoll, amt);
    m_params.arch = lerpf(m_params.arch, m_target.arch, amt);
    m_params.radius = lerpf(m_params.radius, m_target.radius, amt);
    m_params.skew = lerpf(m_params.skew, m_target.skew, amt);
    m_params.blur = lerpf(m_params.blur, m_target.blur, amt);
    // Topology and discrete controls copy immediately.
    const int prevCount = m_params.count;
    m_params.count = m_target.count;
    m_params.ribbons = m_target.ribbons;
    m_params.axis = m_target.axis;
    m_params.cut = m_target.cut;
    m_params.variance = m_target.variance;
    m_params.dealMode = m_target.dealMode;
    m_params.moves = m_target.moves;
    m_params.speed = m_target.speed;
    m_params.tilt = m_target.tilt;
    m_params.ghosts = m_target.ghosts;
    m_params.bob = m_target.bob;
    m_params.backdrop = m_target.backdrop;
    m_params.revealFill = m_target.revealFill;
    if (prevCount != m_params.count && !m_deal)
        m_offset = handWindow(m_current, m_count, std::max(m_params.count, 1));
}

HandLayout::Stage HandLayout::stageFor(const LayoutContext &ctx) const
{
    Stage s;
    s.vw = float(ctx.viewport.width());
    s.vh = float(ctx.viewport.height());
    s.k = stageScale(s.vw, s.vh);
    s.hw = m_params.cardW * s.k * 0.5f;
    s.hh = m_params.cardH * s.k * 0.5f;
    s.radius = clampf(m_params.radius * s.k, 0.0f, std::max(std::min(s.hw, s.hh) - 1.0f, 0.0f));
    s.skew = m_params.skew * s.k;
    s.cam.d = m_params.perspective * s.k;
    s.cam.origin = QVector2D(s.vw * 0.5f + m_params.offsetX * s.vw * 0.5f,
                             s.vh * ANCHOR_Y + m_params.offsetY * s.vh * 0.5f);
    s.cam.shift = QVector2D(0.0f, PERSPECTIVE_DROP * s.vh);
    s.rig = QVector2D(float(m_rigX.x), float(m_rigY.x));
    const auto par = parallaxShift(float(m_rigX.x), float(m_rigY.x), s.k);
    s.parallax = QVector2D(par.first, par.second);
    return s;
}

QMatrix4x4 HandLayout::cardMatrix(const Stage &stage, const Pose &pose, int slot, float rigMix) const
{
    QMatrix4x4 rig = rotationMatrix(AXIS_X, stage.rig.x() * rigMix) * rotationMatrix(AXIS_Y, stage.rig.y() * rigMix);
    QMatrix4x4 m = rig * pose.matrix();
    if (m_params.bob) {
        const Pose b = bob(m_bobT, slot, stage.k);
        const Pose still = Pose::make(QVector3D(0, 0, 0), {Rot{AXIS_Z, 0.0f}}, 1.0f);
        m = m * Pose::mix(b, still, 1.0f - rigMix).matrix();
    }
    return m;
}

float HandLayout::selT(int idx) const
{
    const auto it = m_selection.find(idx);
    if (it != m_selection.end())
        return clampf(float(it->second.x), 0.0f, 1.0f);
    return idx == m_current ? 1.0f : 0.0f;
}

float HandLayout::liftT(int idx) const
{
    const auto it = m_lift.find(idx);
    return it != m_lift.end() ? clampf(float(it->second.x), 0.0f, 1.0f) : 0.0f;
}

float HandLayout::pushSum() const
{
    float sum = 0.0f;
    for (int store : m_shown)
        sum += selT(store);
    return clampf(sum, 0.0f, 1.0f);
}

Pose HandLayout::restPose(const Stage &stage, int slot, int len, float push) const
{
    const float k = stage.k;
    const int idx = m_offset + slot;
    const float base = fanSlot(slot, len, -1);
    float n = base;
    for (int other = 0; other < len; ++other) {
        if (other == slot)
            continue;
        const float t = selT(m_offset + other);
        if (t > 0.0f)
            n += t * (fanSlot(slot, len, other) - base);
    }
    const float selt = selT(idx);
    const float lift = liftT(idx);
    const Pose fan = fanPose(n, m_params.fan(k), k, 1.0f + 0.24f * push, -24.0f * k * lift, -150.0f * k * push,
                             1.0f + 0.06f * lift);
    return Pose::mix(fan, selPose(k), selt);
}

CardVisual HandLayout::quadVisual(const Stage &stage, int row, const Quad &quad, const std::array<float, 4> &locals,
                                  bool columns, QVector2D shift, float hw, float hh, bool wantNear) const
{
    const auto b = quad.bounds();
    CardVisual v;
    v.row = row;
    CardInstance &inst = v.inst;
    setF4(inst.rect, b[0], b[1], hw, hh);
    const float rad = clampf(stage.radius, 0.0f, std::max(std::min(hw, hh) - 1.0f, 0.0f));
    setF4(inst.radii, rad, rad, rad, rad);
    inst.params[0] = stage.skew;
    inst.params[1] = 1.0f;
    inst.params[2] = 1.0f;
    inst.params[3] = 0.0f;
    inst.shape[0] = 0.0f;
    inst.shape[1] = std::abs(stage.skew) * 0.5f;
    inst.shape[2] = 0.0f;
    setF4(inst.border, 0.0f, 0.0f, 0.0f, 0.5f);
    inst.misc[3] |= CardFlag::Projected | (columns ? CardFlag::RibbonColumns : 0u);
    setF4(inst.quadA, quad.pts[0].x(), quad.pts[0].y(), quad.pts[1].x(), quad.pts[1].y());
    setF4(inst.quadB, quad.pts[2].x(), quad.pts[2].y(), quad.pts[3].x(), quad.pts[3].y());
    setF4(inst.quadW, quad.pts[0].z(), quad.pts[1].z(), quad.pts[2].z(), quad.pts[3].z());
    setF4(inst.quadL, locals[0], locals[1], locals[2], locals[3]);
    v.texture = CardVisual::TextureAuto;
    v.wantNear = wantNear;
    v.cover = true;
    v.cropZoom = CROP_ZOOM;
    v.cropShiftX = clampf(shift.x() / std::max(hw, 1.0f), -1.0f, 1.0f);
    v.cropShiftY = clampf(shift.y() / std::max(hh, 1.0f), -1.0f, 1.0f);
    v.cropAspect = hh > 0.0f ? hw / hh : 0.0f;
    return v;
}

void HandLayout::addPlain(const LayoutContext &ctx, const Stage &stage, std::vector<Draw> &draws, int store, float n,
                          const QMatrix4x4 &m, float opacity, int idx, float selt, float lift)
{
    const Quad quad = projectQuad(stage.cam, m, cardCorners(stage.hw, stage.hh, PAD + std::abs(stage.skew) * 0.5f, PAD));
    if (!quad.facing() || !quad.visible(stage.vw, stage.vh))
        return;
    const float df = 1.0f - 0.16f * std::abs(n);
    const QVector2D shift(stage.parallax.x() * df, stage.parallax.y() * df);
    CardVisual v = quadVisual(stage, store, quad, cardLocals(stage.hh, PAD), false, shift, stage.hw, stage.hh, true);
    v.inst.params[2] = opacity;
    if (idx >= 0) {
        const QColor &prim = ctx.palette.primary;
        const float pr = float(prim.redF()), pg = float(prim.greenF()), pb = float(prim.blueF());
        const float base[4] = {pr * lift, pg * lift, pb * lift, 0.5f + 0.1f * lift};
        const float cur[4] = {pr, pg, pb, 1.0f};
        setF4(v.inst.border, lerpf(base[0], cur[0], selt), lerpf(base[1], cur[1], selt), lerpf(base[2], cur[2], selt),
              lerpf(base[3], cur[3], selt));
        v.inst.params[1] = 1.0f + 2.0f * selt;
    }
    const auto b = quad.bounds();
    Draw d;
    d.depth = quad.depth;
    d.vis = v;
    if (idx >= 0) {
        d.hitRow = idx;
        d.hcx = b[0];
        d.hcy = b[1];
        d.hhw = b[2];
        d.hhh = b[3];
    }
    draws.push_back(std::move(d));
}

void HandLayout::addGhost(const Stage &stage, std::vector<Draw> &draws, int store, const QMatrix4x4 &m, int level,
                          float entrance)
{
    const Quad quad = projectQuad(stage.cam, m, cardCorners(stage.hw, stage.hh, PAD + std::abs(stage.skew) * 0.5f, PAD));
    if (!quad.visible(stage.vw, stage.vh))
        return;
    const float alpha = GHOST_LEVELS[size_t(level)][0];
    const float blur = GHOST_LEVELS[size_t(level)][1];
    CardVisual v = quadVisual(stage, store, quad, cardLocals(stage.hh, PAD), false, QVector2D(0, 0), stage.hw, stage.hh,
                              false);
    v.inst.misc[3] |= CardFlag::Ghost;
    v.inst.flip[3] = blur;
    setF4(v.inst.border, 0, 0, 0, 0);
    v.inst.params[1] = 0.0f;
    v.inst.params[2] = alpha * entrance;
    Draw d;
    d.depth = quad.depth;
    d.vis = v;
    draws.push_back(std::move(d));
}

bool HandLayout::addRibbons(const LayoutContext &ctx, const Stage &stage, std::vector<Draw> &draws, int store, int slot,
                            float n, const QMatrix4x4 &mCard, const std::vector<Pose> &poses, float opacity,
                            bool showBack, float seam)
{
    const Axis axis = m_params.axis;
    const bool columns = axis == Axis::Columns;
    const float total = columns ? stage.hw * 2.0f : stage.hh * 2.0f;
    const float padX = PAD + std::abs(stage.skew) * 0.5f;
    const float padAlong = columns ? padX : PAD;
    const float padCross = columns ? PAD : padX;
    const std::vector<RibbonCut> cuts =
        ribbonCuts(int(poses.size()), total, m_params.cut, m_params.variance, m_lseed, slot, padAlong);
    const float df = 1.0f - 0.16f * std::abs(n);
    const QVector2D shift(stage.parallax.x() * df, stage.parallax.y() * df);
    const QColor &pv = ctx.palette.surfaceVariant;
    const float plate[4] = {float(pv.redF()), float(pv.greenF()), float(pv.blueF()), 1.0f};

    bool any = false;
    for (size_t i = 0; i < poses.size() && i < cuts.size(); ++i) {
        const RibbonCut &cut = cuts[i];
        const QMatrix4x4 m = mCard * ribbonOrigin(cut, axis) * poses[i].matrix() * ribbonBack(cut, axis);
        Quad quad = projectQuad(stage.cam, m, ribbonCorners(cut, axis, stage.hw, stage.hh, padCross));
        if (!quad.visible(stage.vw, stage.vh))
            continue;
        any = true;
        if (quad.facing()) {
            CardVisual v = quadVisual(stage, store, quad, ribbonLocals(cut, axis), columns, shift, stage.hw, stage.hh,
                                      true);
            v.inst.params[2] = opacity;
            if (seam > 0.001f)
                setF4(v.inst.border, 1, 1, 1, 0.11f * seam);
            Draw d;
            d.depth = quad.depth;
            d.vis = v;
            draws.push_back(std::move(d));
        } else if (showBack) {
            const RibbonCut mir = mirroredCut(cut);
            quad = projectQuad(stage.cam, m, ribbonCorners(mir, axis, stage.hw, stage.hh, padCross));
            CardVisual v = quadVisual(stage, store, quad, ribbonLocals(mir, axis), columns, shift, stage.hw, stage.hh,
                                      true);
            v.inst.misc[3] |= CardFlag::Backface;
            setF4(v.inst.fill, plate[0], plate[1], plate[2], plate[3]);
            setF4(v.inst.border, 1, 1, 1, 0.17f * seam);
            v.inst.params[1] = 1.0f;
            v.inst.params[2] = opacity;
            v.inst.shape[2] = cut.mid;
            Draw d;
            d.depth = quad.depth;
            d.vis = v;
            draws.push_back(std::move(d));
        }
    }
    return any;
}

std::optional<HandLayout::SlatResult> HandLayout::slatInstance(const LayoutContext &ctx, const Stage &stage,
                                                               const Pose &rest, int slot, int len, float n,
                                                               const Reveal &rev, float t) const
{
    const float k = stage.k;
    const float gap = REVEAL_GAP * k;
    const bool fill = m_params.revealFill;
    const auto rowFrame = revealFrame(stage.vw, stage.vh, false);
    const auto columnFrame = revealFrame(stage.vw, stage.vh, true);
    const std::pair<float, float> card{stage.hw, stage.hh};

    const auto extent = [&](SlatLayout layout) -> std::pair<float, float> {
        switch (layout) {
        case SlatLayout::Fan:
            return card;
        case SlatLayout::Row:
            return fill ? slatHalfExtent(len, rowFrame, gap, false) : keepExtent(len, card, rowFrame, gap, false);
        case SlatLayout::Column:
            return fill ? slatHalfExtent(len, columnFrame, gap, true) : keepExtent(len, card, columnFrame, gap, true);
        }
        return card;
    };
    const auto layoutPose = [&](SlatLayout layout) -> Pose {
        switch (layout) {
        case SlatLayout::Fan:
            return rest;
        case SlatLayout::Row:
            return rowPose(n, extent(SlatLayout::Row).first * 2.0f, gap);
        case SlatLayout::Column:
            return columnPose(n, extent(SlatLayout::Column).first * 2.0f, gap);
        }
        return rest;
    };

    const SlatLayout from = rev.from;
    const SlatLayout to = rev.to;
    const float travel = EASE_REVEAL_TRAVEL.at(t);
    Pose pose;
    float shw, shh;
    if (from == to) {
        pose = layoutPose(to);
        const auto e = extent(to);
        shw = e.first;
        shh = e.second;
    } else {
        const auto ef = extent(from);
        const auto et = extent(to);
        pose = Pose::mix(layoutPose(from), layoutPose(to), travel);
        shw = lerpf(ef.first, et.first, travel);
        shh = lerpf(ef.second, et.second, travel);
    }
    float gather = 1.0f;
    if (from == SlatLayout::Fan && to == SlatLayout::Fan)
        gather = 0.0f;
    else if (from == SlatLayout::Fan)
        gather = t;
    else if (to == SlatLayout::Fan)
        gather = 1.0f - t;

    const float dir = revealDirFor(slot, rev.turnsDone);
    const auto turn = slatTurnPose(t, dir, k);
    const QMatrix4x4 m = cardMatrix(stage, pose, slot, 1.0f) *
                         rotationMatrix(AXIS_Y, 180.0f * float(rev.turnsDone) * dir) * turn.first.matrix();
    Quad quad = projectQuad(stage.cam, m, cardCorners(shw, shh, PAD + std::abs(stage.skew) * 0.5f, PAD));
    if (!quad.visible(stage.vw, stage.vh))
        return std::nullopt;
    if (!quad.facing())
        quad = quad.mirrored();
    const auto locals = cardLocals(shh, PAD);
    const Face face = rev.faces[size_t(revealFace(rev.turnsDone, turn.second))];
    const float df = 1.0f - 0.16f * std::abs(n) * (1.0f - gather);
    const QVector2D shift(stage.parallax.x() * df, stage.parallax.y() * df);
    const QColor &prim = ctx.palette.primary;

    const float nf = float(len);
    const auto rowExt = extent(SlatLayout::Row);
    const float rowW = nf * rowExt.first * 2.0f + (nf - 1.0f) * gap;
    const float rowH = rowExt.second * 2.0f;
    const auto colExt = extent(SlatLayout::Column);
    const float stackW = colExt.second * 2.0f;
    const float stackH = nf * colExt.first * 2.0f + (nf - 1.0f) * gap;

    SlatResult res;
    res.depth = quad.depth;
    res.faceKind = face.kind;
    res.aabb = quad.bounds();
    if (face.kind == FaceKind::Card) {
        res.vis = quadVisual(stage, m_shown[size_t(slot)], quad, locals, false, shift, shw, shh, true);
        setF4(res.vis.inst.border, float(prim.redF()), float(prim.greenF()), float(prim.blueF()), 0.5f);
        res.hitIndex = m_offset + slot;
        return res;
    }

    const auto &b = res.aabb;
    CardVisual v;
    v.row = face.store;
    CardInstance &inst = v.inst;
    setF4(inst.rect, b[0], b[1], shw, shh);
    const float rad = clampf(stage.radius, 0.0f, std::max(std::min(shw, shh) - 1.0f, 0.0f));
    setF4(inst.radii, rad, rad, rad, rad);
    inst.params[0] = stage.skew;
    inst.shape[1] = std::abs(stage.skew) * 0.5f;
    v.texture = CardVisual::TextureNear;
    v.wantNear = true;
    v.cover = false;  // authored crop; CardField keeps inst.crop verbatim
    if (face.column) {
        const auto c = columnCrop(slot, len, stackW, stackH, gap);
        setF4(inst.crop, c[0], c[1], c[2], c[3]);
    } else {
        const auto c = sliceCrop(slot, len, rowW, rowH, gap);
        setF4(inst.crop, c[0], c[1], c[2], c[3]);
    }
    inst.misc[3] |= CardFlag::Projected;
    if (face.kind == FaceKind::Back) {
        inst.misc[3] |= CardFlag::Muted;
        const QColor &pv = ctx.palette.surfaceVariant;
        setF4(inst.fill, float(pv.redF()), float(pv.greenF()), float(pv.blueF()), 1.0f);
    }
    setF4(inst.border, float(prim.redF()), float(prim.greenF()), float(prim.blueF()), 0.35f);
    inst.params[1] = 1.0f;
    setF4(inst.quadA, quad.pts[0].x(), quad.pts[0].y(), quad.pts[1].x(), quad.pts[1].y());
    setF4(inst.quadB, quad.pts[2].x(), quad.pts[2].y(), quad.pts[3].x(), quad.pts[3].y());
    setF4(inst.quadW, quad.pts[0].z(), quad.pts[1].z(), quad.pts[2].z(), quad.pts[3].z());
    setF4(inst.quadL, locals[0], locals[1], locals[2], locals[3]);
    res.vis = v;
    res.hitIndex = m_current;
    return res;
}

void HandLayout::addSlat(const LayoutContext &ctx, const Stage &stage, std::vector<Draw> &draws, const Pose &rest,
                         int idx, int slot, int len, float n, float opacity)
{
    Q_UNUSED(idx)
    const Reveal &rev = *m_reveal;
    const float ss = m_params.speedScale();
    const int order = revealOrder(slot, len, rev.turnsDone);
    const float t = revealTurn(rev.turn, order, ss);
    if (t > 0.0f && t < 1.0f) {
        for (const auto &lag : REVEAL_GHOST_LAG) {
            const float lagged = revealTurn(rev.turn - lag[0] * ss, order, ss);
            if (lagged <= 0.0f || std::abs(lagged - t) < 1e-4f)
                continue;
            auto res = slatInstance(ctx, stage, rest, slot, len, n, rev, lagged);
            if (!res || res->faceKind == FaceKind::Back)
                continue;
            CardVisual v = res->vis;
            v.inst.misc[3] |= CardFlag::Ghost;
            v.inst.flip[3] = lag[2];
            setF4(v.inst.border, 0, 0, 0, 0);
            v.inst.params[1] = 0.0f;
            v.inst.params[2] = lag[1] * opacity;
            Draw d;
            d.depth = res->depth;
            d.vis = v;
            draws.push_back(std::move(d));
        }
    }
    auto res = slatInstance(ctx, stage, rest, slot, len, n, rev, t);
    if (!res)
        return;
    res->vis.inst.params[2] = opacity;
    Draw d;
    d.depth = res->depth;
    d.vis = res->vis;
    d.hitRow = res->hitIndex;
    d.hcx = res->aabb[0];
    d.hcy = res->aabb[1];
    d.hhw = res->aabb[2];
    d.hhh = res->aabb[3];
    draws.push_back(std::move(d));
}

void HandLayout::addFlipped(const LayoutContext &ctx, const Stage &stage, std::vector<Draw> &draws,
                            std::vector<HitRect> &lateHits, int store, int idx, int slot, float n, const Pose &rest,
                            float opacity)
{
    const int nr = std::max(m_params.ribbons, 1);
    const float p = clampf(float(ctx.flip.progress), 0.0f, 1.0f);
    const bool closing = m_flipClosing;
    const float totalMs = closing ? flipCloseMs(nr) : flipOpenMs(nr);
    const float tau = closing ? clampf((FLIP_LAND_AT - p) / FLIP_LAND_AT, 0.0f, 1.0f) * totalMs
                              : std::min(p / FLIP_LAND_AT, 1.0f) * totalMs;
    const float relaxv = relax(p);
    Pose twist = flipTwistPose(tau, closing, stage.k);
    const Pose flat = Pose::make(QVector3D(0, 0, 0), {Rot{AXIS_Y, 0.0f}}, 1.0f);
    twist = Pose::mix(twist, flat, relaxv);
    const QMatrix4x4 mCard = cardMatrix(stage, rest, slot, 1.0f - relaxv) * twist.matrix();

    std::vector<Pose> poses;
    poses.reserve(size_t(nr));
    for (int r = 0; r < nr; ++r)
        poses.push_back(flipRibbonPose(tau, closing, r, nr, ribbonDir(m_flipSeed, slot, r, false), stage.k, m_params.axis));
    const float seam = p > 0.001f ? 1.0f - relaxv : 0.0f;
    addRibbons(ctx, stage, draws, store, slot, n, mCard, poses, opacity, p > 0.001f, seam);

    const Quad outline =
        projectQuad(stage.cam, mCard, cardCorners(stage.hw, stage.hh, PAD + std::abs(stage.skew) * 0.5f, PAD));
    const auto b = outline.bounds();
    lateHits.push_back(HitRect{idx, b[0], b[1], b[2], b[3]});
    m_rects[idx] = QRectF(b[0] - b[2], b[1] - b[3], b[2] * 2.0f, b[3] * 2.0f);
}

void HandLayout::addBackdrop(const LayoutContext &ctx, std::vector<CardVisual> &out, int store, float entrance)
{
    const float vw = float(ctx.viewport.width());
    const float vh = float(ctx.viewport.height());
    CardVisual v;
    v.row = store;
    CardInstance &inst = v.inst;
    setF4(inst.rect, vw * 0.5f, vh * 0.5f, vw * 0.5f, vh * 0.5f);
    setF4(inst.radii, 0, 0, 0, 0);
    inst.params[2] = entrance;
    inst.flip[3] = m_params.blur;
    setF4(inst.border, 0, 0, 0, 0);
    inst.misc[3] |= CardFlag::Backdrop;
    v.texture = CardVisual::TextureNear;
    v.wantNear = true;
    v.cover = true;
    v.cropAspect = 0.0f;  // full viewport; aspect from the rect half extents
    out.push_back(std::move(v));
}

void HandLayout::configure(const LayoutContext &ctx, bool animate)
{
    readParams(ctx.params, m_target);
    if (!m_haveParams || !animate) {
        m_params = m_target;
        m_haveParams = true;
    }
    m_count = ctx.count;
    m_current = std::clamp(ctx.current, 0, std::max(ctx.count - 1, 0));
    m_offset = handWindow(m_current, ctx.count, std::max(m_params.count, 1));
}

void HandLayout::reset(const LayoutContext &ctx)
{
    m_count = ctx.count;
    m_current = std::clamp(ctx.current, 0, std::max(ctx.count - 1, 0));
    m_offset = handWindow(m_current, ctx.count, std::max(m_params.count, 1));
    m_deal.reset();
    m_reveal.reset();
    m_lift.clear();
    m_selection.clear();
}

void HandLayout::select(const LayoutContext &ctx, int from, int to)
{
    m_count = ctx.count;
    if (ctx.count == 0)
        return;
    m_current = std::clamp(to, 0, ctx.count - 1);
    const int size = std::max(m_params.count, 1);
    const int len = handLen(m_offset, ctx.count, size);
    const bool inside = to >= m_offset && to < m_offset + len;

    const auto snapSel = [&](int idx, float value) {
        Spring s = Spring::forDuration(value, 1.0);
        s.target = value;
        m_selection[idx] = s;
    };

    if (inside && !m_deal) {
        if (m_reveal) {
            snapSel(from, 0.0f);
            snapSel(to, 1.0f);
            return;
        }
        const double ms = std::max(ctx.motion->ms(MotionProfile::Standard), 35.0);
        Spring off = Spring::forDuration(m_selection.count(from) ? m_selection[from].x : 1.0, ms);
        off.target = 0.0;
        m_selection[from] = off;
        Spring on = Spring::forDuration(m_selection.count(to) ? m_selection[to].x : 0.0, ms);
        on.target = 1.0;
        m_selection[to] = on;
        return;
    }
    if (inside)
        return;

    const int newOffset = handWindow(to, ctx.count, size);
    if (m_reveal) {
        snapSel(from, 0.0f);
        snapSel(to, 1.0f);
        m_offset = newOffset;
        return;
    }
    beginDeal(ctx, newOffset);
}

void HandLayout::beginDeal(const LayoutContext &ctx, int newOffset)
{
    const float vw = float(ctx.viewport.width());
    const float vh = float(ctx.viewport.height());
    const float k = stageScale(vw, vh);
    const float ss = m_params.speedScale();
    const float pickSeed = rnd(float(m_deals) + 0.5f, 9.0f, 8.0f, 7.0f);
    const Move mv = pickMove(m_params.dealMode, m_params.moves, m_cycle, pickSeed);
    m_cycle = mv;
    m_deals += 1;
    const float seed = rnd(float(m_deals), 1.0f, 2.0f, 3.0f);
    m_lseed = rnd(float(m_deals), 5.0f, 6.0f, 7.0f);
    const int nr = m_params.ribbons;
    const Axis axis = m_params.axis;
    const float push = pushSum();

    std::vector<CardAnim> cards;
    if (m_deal) {
        const float now = m_deal->t;
        cards = std::move(m_deal->cards);
        for (CardAnim &card : cards) {
            const Pose to = movePose(mv, card.n, 1.0f, k);
            const float delay = float(card.slot) * CARD_STAGGER_MS * ss;
            card.pose.retarget(now, to, delay, OUT_MS * ss, EASE_DEAL_OUT);
            for (int level = 0; level < 2; ++level)
                card.ghosts[size_t(level)].retarget(now, to, delay, OUT_MS * GHOST_LEVELS[size_t(level)][2] * ss,
                                                     EASE_DEAL_OUT);
            if (mv == Move::Ribbon) {
                card.ribbons.resize(size_t(nr),
                                    PoseTween::make(Pose::rest(), Pose::rest(), 0.0f, 1.0f, EASE_RIBBON_OUT));
                for (int r = 0; r < nr; ++r)
                    card.ribbons[size_t(r)].retarget(
                        now, ribbonDealPose(r, 1.0f, k, axis),
                        (float(card.slot) * RIBBON_CARD_STAGGER_MS + float(r) * RIBBON_STAGGER_MS) * ss, OUT_MS * ss,
                        EASE_RIBBON_OUT);
            } else {
                for (PoseTween &ribbon : card.ribbons)
                    ribbon.retarget(now, Pose::rest(), 0.0f, OUT_MS * ss, EASE_RIBBON_OUT);
            }
        }
    } else {
        const Stage stage = stageFor(ctx);
        const int len = int(m_shown.size());
        for (int slot = 0; slot < len; ++slot) {
            const float n = handSlotN(slot, len);
            const Pose from = restPose(stage, slot, len, push);
            const Pose to = movePose(mv, n, 1.0f, k);
            const float delay = float(slot) * CARD_STAGGER_MS * ss;
            std::vector<PoseTween> ribbons;
            if (mv == Move::Ribbon)
                for (int r = 0; r < nr; ++r)
                    ribbons.push_back(PoseTween::make(
                        Pose::rest(), ribbonDealPose(r, 1.0f, k, axis),
                        (float(slot) * RIBBON_CARD_STAGGER_MS + float(r) * RIBBON_STAGGER_MS) * ss, OUT_MS * ss,
                        EASE_RIBBON_OUT));
            CardAnim card;
            card.store = m_shown[size_t(slot)];
            card.slot = slot;
            card.n = n;
            card.pose = PoseTween::make(from, to, delay, OUT_MS * ss, EASE_DEAL_OUT);
            card.ribbons = std::move(ribbons);
            for (int level = 0; level < 2; ++level)
                card.ghosts[size_t(level)] =
                    PoseTween::make(from, to, delay, OUT_MS * GHOST_LEVELS[size_t(level)][2] * ss, EASE_DEAL_OUT);
            cards.push_back(std::move(card));
        }
    }

    float end = 0.0f;
    for (const CardAnim &card : cards) {
        float e = card.pose.end();
        for (const PoseTween &ribbon : card.ribbons)
            e = std::max(e, ribbon.end());
        end = std::max(end, e);
    }
    end += SWAP_GAP_MS * ss / 1000.0f;

    Deal deal;
    deal.phase = DealPhase::Out;
    deal.t = 0.0f;
    deal.mv = mv;
    deal.cards = std::move(cards);
    deal.end = end;
    m_deal = std::move(deal);
    m_reveal.reset();
    m_offset = newOffset;
    m_selection.clear();
    m_lift.clear();
    m_rigY.v += double((rnd(seed, 1.0f, 2.0f, 3.0f) - 0.5f) * DEAL_KICK_Y);
    m_rigX.v += double((rnd(seed, 3.0f, 2.0f, 1.0f) - 0.5f) * DEAL_KICK_X);
}

void HandLayout::swapDeal(const LayoutContext &ctx)
{
    if (!m_deal)
        return;
    const float vw = float(ctx.viewport.width());
    const float vh = float(ctx.viewport.height());
    const float k = stageScale(vw, vh);
    const float ss = m_params.speedScale();
    const int size = std::max(m_params.count, 1);
    const int len = handLen(m_offset, ctx.count, size);
    const int nr = m_params.ribbons;
    const Axis axis = m_params.axis;
    const Move mv = m_deal->mv;

    std::vector<CardAnim> cards;
    for (int slot = 0; slot < len; ++slot) {
        const float n = handSlotN(slot, len);
        const Pose from = movePose(mv, n, -1.0f, k);
        const Pose to = fanPose(n, m_params.fan(k), k, 1.0f, 0.0f, 0.0f, 1.0f);
        const float delay = float(slot) * CARD_STAGGER_MS * ss;
        std::vector<PoseTween> ribbons;
        if (mv == Move::Ribbon)
            for (int r = 0; r < nr; ++r)
                ribbons.push_back(PoseTween::make(
                    ribbonDealPose(r, -1.0f, k, axis), Pose::rest(),
                    (float(slot) * RIBBON_CARD_STAGGER_MS + float(nr - 1 - r) * RIBBON_STAGGER_MS) * ss, IN_MS * ss,
                    EASE_RIBBON_IN));
        CardAnim card;
        card.store = m_offset + slot;
        card.slot = slot;
        card.n = n;
        card.pose = PoseTween::make(from, to, delay, IN_MS * ss, EASE_DEAL_IN);
        card.ribbons = std::move(ribbons);
        for (int level = 0; level < 2; ++level)
            card.ghosts[size_t(level)] =
                PoseTween::make(from, to, delay, IN_MS * GHOST_LEVELS[size_t(level)][2] * ss, EASE_DEAL_IN);
        cards.push_back(std::move(card));
    }
    float end = 0.0f;
    for (const CardAnim &card : cards) {
        float e = card.pose.end();
        for (const PoseTween &ribbon : card.ribbons)
            e = std::max(e, ribbon.end());
        end = std::max(end, e);
    }
    m_deal->cards = std::move(cards);
    m_deal->end = end;
    m_deal->phase = DealPhase::In;
    m_deal->t = 0.0f;
}

void HandLayout::landDeal(const LayoutContext &ctx)
{
    m_deal.reset();
    const double ms = std::max(ctx.motion->ms(MotionProfile::Standard), 35.0);
    Spring on = Spring::forDuration(0.0, ms);
    on.target = 1.0;
    m_selection[m_current] = on;
}

void HandLayout::revealToggle(const LayoutContext &ctx)
{
    Q_UNUSED(ctx)
    const bool opening = !m_reveal || !m_reveal->open;
    m_rigY.v += opening ? double(REVEAL_KICK) : double(-REVEAL_KICK);
    m_rigX.v -= double(REVEAL_KICK) * 0.4;
    for (auto &kv : m_selection)
        kv.second.snap(kv.second.target);
    if (!m_reveal) {
        m_lift.clear();
        Reveal rev;
        rev.open = true;
        rev.phase = RevealPhase::Held;
        rev.from = SlatLayout::Fan;
        rev.to = SlatLayout::Fan;
        rev.turn = 0.0f;
        rev.turnsDone = 0;
        rev.faces = {Face{FaceKind::Card, 0, false}, Face{FaceKind::Card, 0, false}};
        rev.len = int(m_shown.size());
        m_reveal = rev;
    } else {
        m_reveal->open = !m_reveal->open;
    }
}

void HandLayout::revealClock(const LayoutContext &ctx, float step)
{
    if (!m_reveal)
        return;
    const float ss = m_params.speedScale();
    const int cur = m_current;
    Reveal &rev = *m_reveal;
    if (rev.phase == RevealPhase::Turning) {
        rev.turn += step;
        if (rev.turn < revealTurnEnd(rev.len, ss))
            return;
        rev.turnsDone += 1;
        rev.turn = 0.0f;
        rev.phase = RevealPhase::Held;
        rev.from = rev.to;
        if (rev.to == SlatLayout::Fan && !rev.open) {
            m_reveal.reset();
            return;
        }
    }
    const Face visible = rev.faces[size_t(revealFace(rev.turnsDone, 0.0f))];
    const bool details = m_flipActive && ctx.flip.progress > 0.5;
    std::optional<Face> incoming;
    SlatLayout to = rev.to;
    if (rev.open && details) {
        const Face back{FaceKind::Back, cur, false};
        if (!(visible == back))
            incoming = back;
    } else if (rev.open) {
        const Face slice{FaceKind::Slice, cur, false};
        if (!(visible == slice)) {
            incoming = slice;
            to = SlatLayout::Row;
        }
    } else if (rev.turnsDone == 0) {
        m_reveal.reset();
        return;
    } else {
        incoming = Face{FaceKind::Card, 0, false};
        to = SlatLayout::Fan;
    }
    if (incoming) {
        rev.faces[size_t((rev.turnsDone + 1) % 2)] = *incoming;
        rev.len = std::max(int(m_shown.size()), 1);
        rev.turn = 0.0f;
        rev.phase = RevealPhase::Turning;
        rev.to = to;
        const float dir = revealDirFor(0, rev.turnsDone);
        m_rigY.v += double(dir * SLAT_KICK);
        m_rigX.v += double(SLAT_KICK) * 0.5;
    }
}

void HandLayout::syncFlip(const LayoutContext &ctx)
{
    const bool onCurrent = ctx.flip.row >= 0 && ctx.flip.row == m_current && ctx.flip.progress > 0.001;
    if (onCurrent) {
        if (!m_flipActive) {
            m_flipActive = true;
            m_flipClosing = false;
            m_flipSeed = ctx.flip.seed;
            m_rigY.v += double(FLIP_KICK);
            if (m_deal) {
                m_deal.reset();
                m_selection.clear();
            }
        } else if (ctx.flip.progress < m_flipPrev - 1e-4 && !m_flipClosing) {
            m_flipClosing = true;
            m_rigY.v -= double(FLIP_KICK);
        } else if (ctx.flip.progress > m_flipPrev + 1e-4 && m_flipClosing) {
            m_flipClosing = false;
        }
    } else {
        m_flipActive = false;
        m_flipClosing = false;
    }
    m_flipPrev = float(ctx.flip.progress);
}

bool HandLayout::tick(const LayoutContext &ctx, double dt)
{
    m_count = ctx.count;
    m_current = std::clamp(ctx.current, 0, std::max(ctx.count - 1, 0));
    morph(ctx, dt);
    m_rigX.tick(dt);
    m_rigY.tick(dt);
    if (m_params.bob)
        m_bobT += float(dt);
    for (auto it = m_lift.begin(); it != m_lift.end();) {
        it->second.tick(dt);
        if (it->second.settled() && it->second.target == 0.0)
            it = m_lift.erase(it);
        else
            ++it;
    }
    const float step = float(std::min(dt, 0.05));
    if (m_deal) {
        m_deal->t += step;
        if (m_deal->t >= m_deal->end) {
            if (m_deal->phase == DealPhase::Out) {
                if (ctx.count == 0)
                    m_deal.reset();
                else
                    swapDeal(ctx);
            } else {
                landDeal(ctx);
            }
        }
    }
    syncFlip(ctx);
    revealClock(ctx, step);
    for (auto it = m_selection.begin(); it != m_selection.end();) {
        it->second.tick(dt);
        if (it->second.settled() && it->second.target == 0.0 && it->first != m_current)
            it = m_selection.erase(it);
        else
            ++it;
    }

    bool animating = false;
    if (m_deal)
        animating = true;
    if (m_flipActive && ctx.flip.progress > 0.001 && ctx.flip.progress < 0.999)
        animating = true;
    if (m_reveal && m_reveal->phase != RevealPhase::Held)
        animating = true;
    if (!m_rigX.settled() || !m_rigY.settled())
        animating = true;
    for (const auto &kv : m_lift)
        if (!kv.second.settled())
            animating = true;
    for (const auto &kv : m_selection)
        if (!kv.second.settled())
            animating = true;
    if (m_params.bob)
        animating = true;
    return animating;
}

void HandLayout::build(const LayoutContext &ctx, std::vector<CardVisual> &out)
{
    m_hits.clear();
    m_rects.clear();
    m_count = ctx.count;
    if (ctx.count == 0) {
        m_shown.clear();
        return;
    }
    m_current = std::clamp(ctx.current, 0, ctx.count - 1);
    syncFlip(ctx);
    const Stage stage = stageFor(ctx);
    const float entrance = float(ctx.entrance);

    if (m_params.backdrop)
        addBackdrop(ctx, out, m_current, entrance);

    std::vector<Draw> draws;
    draws.reserve(64);
    std::vector<HitRect> lateHits;
    const int size = std::max(m_params.count, 1);

    if (m_deal) {
        m_shown.clear();
        for (const CardAnim &card : m_deal->cards)
            m_shown.push_back(card.store);
        for (const CardAnim &card : m_deal->cards) {
            const Pose pose = card.pose.at(m_deal->t);
            const QMatrix4x4 m = cardMatrix(stage, pose, card.slot, 1.0f);
            if (m_params.ghosts)
                for (int level = 0; level < 2; ++level) {
                    const QMatrix4x4 gm = cardMatrix(stage, card.ghosts[size_t(level)].at(m_deal->t), card.slot, 1.0f);
                    addGhost(stage, draws, card.store, gm, level, entrance);
                }
            if (card.ribbons.empty()) {
                addPlain(ctx, stage, draws, card.store, card.n, m, entrance, -1, 0.0f, 0.0f);
            } else {
                std::vector<Pose> poses;
                poses.reserve(card.ribbons.size());
                for (const PoseTween &ribbon : card.ribbons)
                    poses.push_back(ribbon.at(m_deal->t));
                addRibbons(ctx, stage, draws, card.store, card.slot, card.n, m, poses, entrance, false, 0.0f);
            }
        }
    } else {
        int len = handLen(m_offset, ctx.count, size);
        if (m_current < m_offset || m_current >= m_offset + len)
            m_offset = handWindow(m_current, ctx.count, size);
        len = handLen(m_offset, ctx.count, size);
        m_shown.clear();
        for (int slot = 0; slot < len; ++slot)
            m_shown.push_back(m_offset + slot);
        const float push = pushSum();
        for (int slot = 0; slot < len; ++slot) {
            const int idx = m_offset + slot;
            const int store = idx;
            const float n = handSlotN(slot, len);
            const float selt = selT(idx);
            const float lift = liftT(idx);
            const Pose pose = restPose(stage, slot, len, push);
            if (m_reveal) {
                addSlat(ctx, stage, draws, pose, idx, slot, len, n, entrance);
            } else if (m_flipActive && ctx.flip.row == idx && ctx.flip.progress > 0.001) {
                addFlipped(ctx, stage, draws, lateHits, store, idx, slot, n, pose, entrance);
            } else {
                const QMatrix4x4 m = cardMatrix(stage, pose, slot, 1.0f);
                addPlain(ctx, stage, draws, store, n, m, entrance, idx, selt, lift);
            }
        }
    }

    std::sort(draws.begin(), draws.end(), [](const Draw &a, const Draw &b) { return a.depth < b.depth; });
    for (Draw &d : draws) {
        out.push_back(std::move(d.vis));
        if (d.hitRow >= 0) {
            m_hits.push_back(HitRect{d.hitRow, d.hcx, d.hcy, d.hhw, d.hhh});
            m_rects[d.hitRow] = QRectF(d.hcx - d.hhw, d.hcy - d.hhh, d.hhw * 2.0f, d.hhh * 2.0f);
        }
    }
    for (const HitRect &h : lateHits) {
        m_hits.push_back(h);
        m_rects[h.row] = QRectF(h.cx - h.hw, h.cy - h.hh, h.hw * 2.0f, h.hh * 2.0f);
    }
}

int HandLayout::hitTest(QPointF point) const
{
    for (auto it = m_hits.rbegin(); it != m_hits.rend(); ++it) {
        if (point.x() >= it->cx - it->hw && point.x() <= it->cx + it->hw && point.y() >= it->cy - it->hh &&
            point.y() <= it->cy + it->hh)
            return it->row;
    }
    return -1;
}

QRectF HandLayout::cardRect(int row) const
{
    const auto it = m_rects.find(row);
    return it != m_rects.end() ? it->second : QRectF();
}

int HandLayout::step(const LayoutContext &ctx, int dx, int dy) const
{
    Q_UNUSED(dy)
    if (ctx.count == 0)
        return ctx.current;
    const int dir = dx > 0 ? 1 : dx < 0 ? -1 : 0;
    return std::clamp(ctx.current + dir, 0, ctx.count - 1);
}

int HandLayout::page(const LayoutContext &ctx, int dir) const
{
    Q_UNUSED(ctx)
    return dir * std::max(m_params.count, 1);
}

int HandLayout::wheel(const LayoutContext &ctx, QPointF angle, QPointF pixels)
{
    Q_UNUSED(ctx)
    m_wheelAccum += float(angle.y()) + float(pixels.y());
    const int notches = int(m_wheelAccum / 120.0f);
    if (notches != 0)
        m_wheelAccum -= float(notches) * 120.0f;
    return -notches;
}

bool HandLayout::pointer(const LayoutContext &ctx, QPointF pos, bool inside)
{
    const float vw = float(ctx.viewport.width());
    const float vh = float(ctx.viewport.height());
    const auto tilt = tiltTarget(float(pos.x()), float(pos.y()), vw, vh, m_params.tilt);
    m_rigX.target = inside ? tilt.first : 0.0;
    m_rigY.target = inside ? tilt.second : 0.0;

    const double ms = std::max(ctx.motion->ms(MotionProfile::Fast), 35.0);
    const int hovered = (m_deal || m_reveal || !inside) ? -1 : ctx.hovered;
    for (auto &kv : m_lift)
        if (kv.first != hovered)
            kv.second.target = 0.0;
    if (hovered >= 0) {
        auto it = m_lift.find(hovered);
        if (it == m_lift.end())
            it = m_lift.emplace(hovered, Spring::forDuration(0.0, ms)).first;
        it->second.target = 1.0;
    }
    return true;
}

bool HandLayout::drag(const LayoutContext &ctx, QPointF delta, bool released)
{
    Q_UNUSED(ctx)
    if (released)
        return true;
    const double dx = delta.x();
    const double dy = delta.y();
    m_rigY.x += dx * DRAG_GAIN_Y;
    m_rigX.x -= dy * DRAG_GAIN_X;
    m_rigY.v = dx * DRAG_GAIN_Y * 30.0;
    m_rigX.v = -dy * DRAG_GAIN_X * 30.0;
    return true;
}

Layout::Click HandLayout::click(const LayoutContext &ctx, int row)
{
    // A click never applies from the fan; the Apply key does.
    return row == ctx.current ? Click::Ignore : Click::Select;
}

bool HandLayout::action(const LayoutContext &ctx, const QString &name)
{
    if (name == QLatin1String("reveal")) {
        revealToggle(ctx);
        return true;
    }
    if (name == QLatin1String("deal")) {
        beginDeal(ctx, handWindow(m_current, ctx.count, std::max(m_params.count, 1)));
        return true;
    }
    return false;
}

double HandLayout::cameraMs(const LayoutContext &ctx) const { return ctx.motion->ms(MotionProfile::Standard); }

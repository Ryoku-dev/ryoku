#include "walllayout.h"

#include "../scene/cardsource.h"
#include "geometry.h"

#include <QSizeF>

#include <algorithm>
#include <cmath>

namespace {

// Breathing room either side of a pinned-column grid inside its pane.
constexpr float kPinnedMargin = 12.0f;

QString wsKey(const char *name)
{
    return QStringLiteral("components.wallpaperSelector.") + QLatin1String(name);
}

void setColor(float dst[4], const QColor &c, float a)
{
    dst[0] = float(c.redF());
    dst[1] = float(c.greenF());
    dst[2] = float(c.blueF());
    dst[3] = a;
}

void setRadii(float dst[4], float r)
{
    dst[0] = dst[1] = dst[2] = dst[3] = r;
}

WallLayout::Kind parseKind(const QString &v)
{
    if (v == QLatin1String("brick"))
        return WallLayout::Brick;
    if (v == QLatin1String("masonry"))
        return WallLayout::Masonry;
    if (v == QLatin1String("justified"))
        return WallLayout::Justified;
    if (v == QLatin1String("editorial"))
        return WallLayout::Editorial;
    if (v == QLatin1String("cylinder"))
        return WallLayout::Cylinder;
    return WallLayout::Uniform;
}

// angleDelta is in eighths of a degree (120 per notch); touchpad pixels convert at /60.
float wheelAmount(QPointF angle, QPointF pixels)
{
    const float ly = float(angle.y()) / 120.0f;
    const float lx = float(angle.x()) / 120.0f;
    if (ly != 0.0f || lx != 0.0f)
        return ly != 0.0f ? ly : lx;
    const float py = float(pixels.y());
    const float px = float(pixels.x());
    if (py != 0.0f || px != 0.0f)
        return (py != 0.0f ? py : px) / 60.0f;
    return 0.0f;
}

bool gridNeighbour(int idx, int cur, int count, int cols)
{
    if (idx == cur)
        return true;
    if ((idx == cur - 1 && cur % cols != 0) || (idx == cur + 1 && idx % cols != 0))
        return true;
    return (idx == cur - cols && cur >= cols) || (idx == cur + cols && idx < count);
}

} // namespace

WallLayout::WallLayout()
{
    m_hoverFades.reserve(8);
    m_cells.reserve(96);
    m_places.reserve(128);
    m_bottoms.reserve(16);
}

QString WallLayout::key() const
{
    return QStringLiteral("wall");
}

double WallLayout::cameraMs(const LayoutContext &ctx) const
{
    // Grid is the one mode whose camera runs on the slow tier.
    return ctx.motion->ms(MotionProfile::Slow);
}

QRectF WallLayout::clip(const LayoutContext &ctx) const
{
    Q_UNUSED(ctx)
    return m_frame;
}

WallLayout::Stage WallLayout::readStage(const LayoutContext &ctx) const
{
    const ParamSource *p = ctx.params;
    Stage s;
    s.offsetX = std::clamp(float(p->num(wsKey("gridStageX"), 0)) / 100.0f, -1.0f, 1.0f);
    s.offsetY = std::clamp(float(p->num(wsKey("gridStageY"), 0)) / 100.0f, -1.0f, 1.0f);
    s.scale = std::clamp(float(p->num(wsKey("gridStageScale"), 100)) / 100.0f, 0.25f, 2.5f);
    s.rotation = std::clamp(float(p->num(wsKey("gridStageRotation"), 0)), -45.0f, 45.0f);
    s.perspective = std::clamp(float(p->num(wsKey("gridStagePerspective"), 0)) / 100.0f, -0.8f, 0.8f);
    s.shearX = std::clamp(float(p->num(wsKey("gridStageShearX"), 0)) / 100.0f, -1.0f, 1.0f);
    s.shearY = std::clamp(float(p->num(wsKey("gridStageShearY"), 0)) / 100.0f, -1.0f, 1.0f);
    s.depthAngle = std::clamp(float(p->num(wsKey("gridStageDepthAngle"), 0)), -180.0f, 180.0f);
    return s;
}

WallLayout::Params WallLayout::readParams(const LayoutContext &ctx) const
{
    const ParamSource *p = ctx.params;
    const bool small = p->smallScreen();
    Params np;
    np.cols = int(std::lround(std::clamp(p->num(wsKey("gridColumns"), small ? 4 : 6), 1.0, 64.0)));
    np.rows = int(std::lround(std::clamp(p->num(wsKey("gridRows"), 3), 1.0, 64.0)));
    np.thumbW = std::clamp(float(p->num(wsKey("gridThumbWidth"), small ? 220 : 300)),
                           24.0f, 4096.0f);
    np.thumbH = std::clamp(float(p->num(wsKey("gridThumbHeight"), small ? 124 : 169)),
                           24.0f, 4096.0f);
    np.gapX = std::clamp(float(p->num(wsKey("gridGapX"), 8)), 0.0f, 256.0f);
    np.gapY = std::clamp(float(p->num(wsKey("gridGapY"), 8)), 0.0f, 256.0f);
    const bool round = p->flag(wsKey("gridRoundCorners"), true);
    np.cornerRadius = round ? std::clamp(float(p->num(wsKey("gridCornerRadius"), 6)), 0.0f, 128.0f) : 0.0f;
    np.borderWidth = std::clamp(float(p->num(wsKey("gridBorderWidth"), 2)), 0.0f, 32.0f);
    np.layout = parseKind(p->text(wsKey("gridLayout"), QStringLiteral("uniform")));
    np.stagger = std::clamp(float(p->num(wsKey("gridStagger"), 50)) / 100.0f, -1.0f, 1.0f);
    np.selectedScale = std::clamp(float(p->num(wsKey("gridSelectedScale"), 100)) / 100.0f, 1.0f, 2.0f);
    np.flowWave = std::clamp(float(p->num(wsKey("gridFlowWave"), 0)), -200.0f, 200.0f);
    np.flowFrequency = std::clamp(float(p->num(wsKey("gridFlowFrequency"), 1)), 0.1f, 8.0f);
    np.scatter = std::clamp(float(p->num(wsKey("gridScatter"), 0)), 0.0f, 150.0f);
    np.scaleVariance = std::clamp(float(p->num(wsKey("gridScaleVariance"), 0)) / 100.0f, 0.0f, 0.75f);
    np.cylinderBend = std::clamp(float(p->num(wsKey("gridCylinderBend"), 65)) / 100.0f, -1.0f, 1.0f);
    np.cylinderRadius = std::clamp(float(p->num(wsKey("gridCylinderRadius"), 720)), 150.0f, 2400.0f);
    np.stage = readStage(ctx);
    return np;
}

void WallLayout::Stage::transform(float x, float y, float ox, float oy, float vw, float vh,
                                  float &outX, float &outY, float &outScale) const
{
    const float rad = rotation * float(M_PI) / 180.0f;
    const float s = std::sin(rad), c = std::cos(rad);
    const float lx = (x - ox) * scale;
    const float ly = (y - oy) * scale;
    const float shx = lx + shearX * ly;
    const float shy = ly + shearY * lx;
    const float rx = shx * c - shy * s;
    const float ry = shx * s + shy * c;
    const float drad = depthAngle * float(M_PI) / 180.0f;
    const float axis = rx * std::sin(drad) + ry * std::cos(drad);
    const float depth = std::clamp(1.0f + perspective * axis / std::max(vh * 0.5f, 1.0f), 0.35f, 2.0f);
    outX = ox + offsetX * vw * 0.5f + rx * depth;
    outY = oy + offsetY * vh * 0.5f + ry;
    outScale = scale * depth;
}

void WallLayout::Stage::morphToward(const Stage &t, float amt)
{
    offsetX = geom::lerp(offsetX, t.offsetX, amt);
    offsetY = geom::lerp(offsetY, t.offsetY, amt);
    scale = geom::lerp(scale, t.scale, amt);
    rotation = geom::lerp(rotation, t.rotation, amt);
    perspective = geom::lerp(perspective, t.perspective, amt);
    shearX = geom::lerp(shearX, t.shearX, amt);
    shearY = geom::lerp(shearY, t.shearY, amt);
    depthAngle = geom::lerp(depthAngle, t.depthAngle, amt);
}

bool WallLayout::Stage::settledTo(const Stage &t) const
{
    return geom::feq(offsetX, t.offsetX) && geom::feq(offsetY, t.offsetY)
        && geom::feq(scale, t.scale) && geom::feq(rotation, t.rotation)
        && geom::feq(perspective, t.perspective) && geom::feq(shearX, t.shearX)
        && geom::feq(shearY, t.shearY) && geom::feq(depthAngle, t.depthAngle);
}

void WallLayout::Params::cylinderTransform(float x, float y, float ox, float &outX, float &outY,
                                           float &outScale) const
{
    const float bend = std::clamp(cylinderBend, -1.0f, 1.0f);
    if (layout != Cylinder || std::abs(bend) <= 0.001f) {
        outX = x;
        outY = y;
        outScale = 1.0f;
        return;
    }
    const float radius = std::max(cylinderRadius, 1.0f);
    const float localX = x - ox;
    const float angle = std::clamp(localX / radius, -1.35f, 1.35f);
    const float projected = std::sin(angle) * radius;
    const float mix = std::abs(bend);
    const float curved = geom::lerp(localX, projected, mix);
    const float edgeDepth = 1.0f - std::cos(angle);
    outX = ox + curved;
    outY = y;
    outScale = std::clamp(1.0f - bend * edgeDepth, 0.35f, 2.0f);
}

void WallLayout::Params::morphToward(const Params &t, float amt)
{
    cols = t.cols;
    rows = t.rows;
    layout = t.layout;
    thumbW = geom::lerp(thumbW, t.thumbW, amt);
    thumbH = geom::lerp(thumbH, t.thumbH, amt);
    gapX = geom::lerp(gapX, t.gapX, amt);
    gapY = geom::lerp(gapY, t.gapY, amt);
    cornerRadius = geom::lerp(cornerRadius, t.cornerRadius, amt);
    borderWidth = geom::lerp(borderWidth, t.borderWidth, amt);
    stagger = geom::lerp(stagger, t.stagger, amt);
    selectedScale = geom::lerp(selectedScale, t.selectedScale, amt);
    flowWave = geom::lerp(flowWave, t.flowWave, amt);
    flowFrequency = geom::lerp(flowFrequency, t.flowFrequency, amt);
    scatter = geom::lerp(scatter, t.scatter, amt);
    scaleVariance = geom::lerp(scaleVariance, t.scaleVariance, amt);
    cylinderBend = geom::lerp(cylinderBend, t.cylinderBend, amt);
    cylinderRadius = geom::lerp(cylinderRadius, t.cylinderRadius, amt);
    stage.morphToward(t.stage, amt);
}

bool WallLayout::Params::settledTo(const Params &t) const
{
    return cols == t.cols && rows == t.rows && layout == t.layout && geom::feq(thumbW, t.thumbW)
        && geom::feq(thumbH, t.thumbH) && geom::feq(gapX, t.gapX) && geom::feq(gapY, t.gapY)
        && geom::feq(cornerRadius, t.cornerRadius) && geom::feq(borderWidth, t.borderWidth)
        && geom::feq(stagger, t.stagger) && geom::feq(selectedScale, t.selectedScale)
        && geom::feq(flowWave, t.flowWave) && geom::feq(flowFrequency, t.flowFrequency)
        && geom::feq(scatter, t.scatter) && geom::feq(scaleVariance, t.scaleVariance)
        && geom::feq(cylinderBend, t.cylinderBend) && geom::feq(cylinderRadius, t.cylinderRadius)
        && stage.settledTo(t.stage);
}

void WallLayout::configure(const LayoutContext &ctx, bool animate)
{
    const Params np = readParams(ctx);
    m_settingsCols = np.cols;
    m_preferredThumbW = np.thumbW;
    m_preferredThumbH = np.thumbH;
    if (animate && m_haveParams) {
        m_target = np;
        // Topology jumps at once; the scene crossfade hides the reflow.
        m_live.cols = np.cols;
        m_live.rows = np.rows;
        m_live.layout = np.layout;
    } else {
        m_live = np;
        m_target = np;
        m_hoverFades.clear();
        m_snapCamera = true;
        m_camera = Spring::forDuration(0.0, cameraMs(ctx));
    }
    m_camera.setDuration(cameraMs(ctx));
    m_haveParams = true;
}

void WallLayout::reset(const LayoutContext &ctx)
{
    Q_UNUSED(ctx)
    m_hoverFades.clear();
    m_snapCamera = true;
}

void WallLayout::gridFollow(int idx)
{
    const Params &p = m_live;
    const float viewH = m_viewHeight > 0 ? m_viewHeight : p.totalH();
    float rowTop;
    float itemH;
    if (p.layout == Editorial) {
        const float blockH = p.thumbH * 2.0f + p.gapY;
        rowTop = float(idx / 5) * (blockH + p.gapY);
        itemH = blockH;
    } else {
        const int cols = std::max(p.cols, 1);
        rowTop = float(idx / cols) * p.cellH();
        itemH = p.cellH();
    }
    const float target = float(m_camera.target);
    if (rowTop < target)
        m_camera.target = std::max(rowTop, 0.0f);
    else if (rowTop + itemH > target + viewH)
        m_camera.target = std::max(rowTop + itemH - viewH, 0.0f);
}

void WallLayout::select(const LayoutContext &ctx, int from, int to)
{
    Q_UNUSED(ctx)
    Q_UNUSED(from)
    gridFollow(to);
}

bool WallLayout::tick(const LayoutContext &ctx, double dt)
{
    bool moving = m_camera.tick(dt);
    const double fastMs = ctx.motion->ms(MotionProfile::Fast);
    const int hv = ctx.hovered;
    if (hv >= 0) {
        bool found = false;
        for (auto &hf : m_hoverFades) {
            if (hf.first == hv) {
                hf.second.target = 1.0;
                found = true;
                break;
            }
        }
        if (!found) {
            Spring sp = Spring::forDuration(0.0, fastMs);
            sp.target = 1.0;
            m_hoverFades.push_back({hv, sp});
        }
    }
    for (std::size_t i = 0; i < m_hoverFades.size();) {
        auto &e = m_hoverFades[i];
        if (e.first != hv)
            e.second.target = 0.0;
        const bool mv = e.second.tick(dt);
        moving = moving || mv;
        if (!mv && e.second.target <= 0.0 && e.second.x <= 0.001) {
            e = m_hoverFades.back();
            m_hoverFades.pop_back();
        } else {
            ++i;
        }
    }
    if (!m_live.settledTo(m_target)) {
        const float amt = float(approachK(dt, ctx.motion->tau(MotionProfile::Standard)));
        m_live.morphToward(m_target, amt);
        moving = true;
    }
    return moving;
}

void WallLayout::placements(const LayoutContext &ctx, float totalW)
{
    const Params &p = m_live;
    const int count = ctx.count;
    const int cols = std::max(p.cols, 1);
    const CardSource *src = ctx.source;
    m_places.clear();

    auto aspectOf = [&](int idx, float lo, float hi) -> float {
        if (src) {
            const QSizeF s = src->cardImageSize(idx);
            if (s.width() > 0 && s.height() > 0)
                return std::clamp(float(s.width() / s.height()), lo, hi);
        }
        return p.thumbW / std::max(p.thumbH, 1.0f);
    };

    switch (p.layout) {
    case Uniform:
    case Brick:
    case Cylinder:
        for (int idx = 0; idx < count; ++idx) {
            const int row = idx / cols;
            const int col = idx % cols;
            const float stagger =
                (p.layout == Brick && (row % 2 == 1)) ? p.cellW() * p.stagger : 0.0f;
            m_places.push_back({idx, col * p.cellW() + stagger, row * p.cellH(), p.thumbW, p.thumbH});
        }
        break;
    case Masonry: {
        m_bottoms.assign(cols, 0.0f);
        for (int idx = 0; idx < count; ++idx) {
            int col = 0;
            float best = m_bottoms[0];
            for (int c = 1; c < cols; ++c) {
                if (m_bottoms[c] < best) {
                    best = m_bottoms[c];
                    col = c;
                }
            }
            const float aspect = std::max(aspectOf(idx, 0.15f, 1e9f), 0.15f);
            const float height =
                std::clamp(p.thumbW / aspect, p.thumbH * 0.55f, p.thumbH * 1.8f);
            const float y = m_bottoms[col];
            m_places.push_back({idx, col * p.cellW(), y, p.thumbW, height});
            m_bottoms[col] = y + height + p.gapY;
        }
        break;
    }
    case Justified: {
        float y = 0;
        for (int start = 0; start < count; start += cols) {
            const int end = std::min(start + cols, count);
            float sumAspect = 0;
            for (int i = start; i < end; ++i)
                sumAspect += aspectOf(i, 0.45f, 2.5f);
            const float gaps = p.gapX * float(std::max(end - start - 1, 0));
            const float height = std::clamp((totalW - gaps) / std::max(sumAspect, 0.1f),
                                            p.thumbH * 0.58f, p.thumbH * 1.5f);
            float x = 0;
            for (int i = start; i < end; ++i) {
                const float w = aspectOf(i, 0.45f, 2.5f) * height;
                m_places.push_back({i, x, y, w, height});
                x += w + p.gapX;
            }
            y += height + p.gapY;
        }
        break;
    }
    case Editorial: {
        const float blockH = p.thumbH * 2.0f + p.gapY;
        const float heroFraction = std::clamp(0.5f + 0.16f * p.stagger, 0.35f, 0.72f);
        const float heroW = std::max(totalW * heroFraction, 1.0f);
        const float sideW = std::max(totalW - heroW - p.gapX, 1.0f);
        const float smallW = std::max((sideW - p.gapX) * 0.5f, 1.0f);
        const float smallH = std::max((blockH - p.gapY) * 0.5f, 1.0f);
        for (int start = 0; start < count; start += 5) {
            const int block = start / 5;
            const float y = float(block) * (blockH + p.gapY);
            const bool heroRight = (block % 2 == 1);
            const float heroX = heroRight ? sideW + p.gapX : 0.0f;
            m_places.push_back({start, heroX, y, heroW, blockH});
            for (int s = 1; s < 5; ++s) {
                const int idx = start + s;
                if (idx >= count)
                    break;
                const int slot = s - 1;
                const float x0 = heroRight ? 0.0f : heroW + p.gapX;
                const float x = x0 + float(slot % 2) * (smallW + p.gapX);
                const float sy = y + float(slot / 2) * (smallH + p.gapY);
                m_places.push_back({idx, x, sy, smallW, smallH});
            }
        }
        break;
    }
    }

    const float flowSpan = std::max(p.spanH(std::max(p.rows, 1)), 1.0f);
    for (auto &pl : m_places) {
        if (std::abs(p.flowWave) > 0.001f) {
            const float phase =
                (pl.y + pl.h * 0.5f) / flowSpan * 6.2831853f * p.flowFrequency;
            pl.x += std::sin(phase) * p.flowWave;
        }
        if (p.scatter > 0.001f) {
            pl.x += geom::signedHash(unsigned(pl.idx), 0xc2b2ae35u) * p.scatter;
            pl.y += geom::signedHash(unsigned(pl.idx), 0x27d4eb2fu) * p.scatter;
        }
        if (p.scaleVariance > 0.001f) {
            const float factor = std::clamp(
                1.0f + geom::signedHash(unsigned(pl.idx), 0x165667b1u) * p.scaleVariance, 0.3f, 2.0f);
            const float nw = pl.w * factor;
            const float nh = pl.h * factor;
            pl.x += (pl.w - nw) * 0.5f;
            pl.y += (pl.h - nh) * 0.5f;
            pl.w = nw;
            pl.h = nh;
        }
    }
}

void WallLayout::build(const LayoutContext &ctx, std::vector<CardVisual> &out)
{
    if (!m_haveParams)
        configure(ctx, false);

    int effectiveCols = m_settingsCols;
    const float vw = float(ctx.viewport.width());
    const float vh = float(ctx.viewport.height());
    if (ctx.columnsOverride > 0) {
        const float room = std::max(vw - 2.0f * kPinnedMargin, 1.0f);
        const float preferredCell = std::max(m_preferredThumbW + m_target.gapX, 1.0f);
        effectiveCols = std::max(1, int((room + m_target.gapX) / preferredCell));
        const float stretchedW =
            std::max((room - m_target.gapX * float(effectiveCols - 1)) / float(effectiveCols), 1.0f);
        const float aspect = m_preferredThumbW / std::max(m_preferredThumbH, 1.0f);
        const float stretchedH = stretchedW / std::max(aspect, 0.1f);
        m_live.thumbW = stretchedW;
        m_target.thumbW = stretchedW;
        m_live.thumbH = stretchedH;
        m_target.thumbH = stretchedH;
    }
    m_live.cols = effectiveCols;
    m_target.cols = effectiveCols;

    m_cells.clear();
    const int count = ctx.count;
    if (count <= 0) {
        m_frame = QRectF();
        return;
    }

    const Params &p = m_live;
    const float entrance = float(ctx.entrance);
    const int current = std::clamp(ctx.current, 0, count - 1);

    const float totalW = p.totalW();
    m_viewHeight = ctx.columnsOverride > 0
        ? std::max(vh - 2.0f * kPinnedMargin, p.thumbH)
        : p.totalH();
    const float viewH = m_viewHeight;
    const QPointF center = compositionCenter(ctx);
    const float cx = float(center.x());
    const float cy = float(center.y());

    placements(ctx, totalW);
    float contentH = 0;
    for (const auto &pl : m_places)
        contentH = std::max(contentH, pl.y + pl.h);
    m_maxScroll = std::max(contentH - viewH, 0.0f);

    if (m_snapCamera) {
        float snapTo = 0;
        for (const auto &pl : m_places) {
            if (pl.idx == current) {
                snapTo = std::clamp(pl.y + pl.h * 0.5f - viewH * 0.5f, 0.0f, m_maxScroll);
                break;
            }
        }
        m_camera.snap(snapTo);
        m_snapCamera = false;
    } else {
        m_camera.target = std::clamp(float(m_camera.target), 0.0f, m_maxScroll);
    }
    const float cam = float(m_camera.x);

    const float left = cx - totalW * 0.5f;
    const float top = cy - viewH * 0.5f;
    const float fL = std::max(left, 0.0f);
    const float fT = std::max(top, 0.0f);
    const float fR = std::min(left + totalW, vw);
    const float fB = std::min(top + viewH, vh);
    m_frame = QRectF(fL, fT, std::max(fR - fL, 0.0f), std::max(fB - fT, 0.0f));

    for (const auto &pl : m_places) {
        const float rawX = left + pl.x + pl.w * 0.5f;
        const float rawY = top + pl.y - cam + pl.h * 0.5f;
        float sx, sy, ss;
        p.cylinderTransform(rawX, rawY, cx, sx, sy, ss);
        float ex, ey, stageScale;
        p.stage.transform(sx, sy, cx, cy, vw, vh, ex, ey, stageScale);
        const float focus = (pl.idx == current) ? p.selectedScale : 1.0f;
        const float hw = pl.w * stageScale * ss * focus * 0.5f;
        const float hh = pl.h * stageScale * ss * focus * 0.5f;
        if (ex + hw <= fL || ex - hw >= fR || ey + hh <= fT || ey - hh >= fB)
            continue;
        m_cells.push_back({pl.idx, ex, ey, hw, hh});
        emitCell(ctx, out, pl.idx, ex, ey, hw, hh, entrance);
    }
}

void WallLayout::emitCell(const LayoutContext &ctx, std::vector<CardVisual> &out, int idx, float px,
                          float py, float hw, float hh, float entrance)
{
    const Params &p = m_live;
    const int current = std::clamp(ctx.current, 0, ctx.count - 1);
    const float radius = std::clamp(p.cornerRadius, 0.0f, std::min(hw, hh));
    const float bw = std::clamp(p.borderWidth, 0.0f, std::max(std::min(hw, hh) - 1.0f, 0.0f));

    float lift = (idx == ctx.hovered) ? 1.0f : 0.0f;
    for (const auto &hf : m_hoverFades) {
        if (hf.first == idx) {
            lift = std::clamp(float(hf.second.x), 0.0f, 1.0f);
            break;
        }
    }
    const float borderAlpha = (idx == current) ? std::max(lift, 0.8f) : lift;

    CardVisual plate;
    plate.row = -1;
    plate.texture = CardVisual::TextureNone;
    plate.inst.rect[0] = px;
    plate.inst.rect[1] = py;
    plate.inst.rect[2] = hw;
    plate.inst.rect[3] = hh;
    setRadii(plate.inst.radii, radius);
    setColor(plate.inst.fill, ctx.palette.surface, 0.6f * entrance);
    plate.inst.params[2] = entrance;
    plate.inst.misc[2] = 1; // clip to the scroll frame
    out.push_back(plate);

    CardVisual body;
    body.row = idx;
    body.texture = CardVisual::TextureAuto;
    body.inst.rect[0] = px;
    body.inst.rect[1] = py;
    body.inst.rect[2] = hw;
    body.inst.rect[3] = hh;
    setRadii(body.inst.radii, radius);
    body.inst.params[2] = entrance;
    body.inst.misc[2] = 1;
    if (borderAlpha > 0.003f) {
        setColor(body.inst.border, ctx.palette.primary, borderAlpha);
        body.inst.params[1] = bw;
    }
    body.cover = true;
    body.cropZoom = 1.0f;
    body.cropAspect = 0.0f;
    body.wantNear = gridNeighbour(idx, current, ctx.count, std::max(p.cols, 1));
    out.push_back(body);
}

int WallLayout::hitTest(QPointF point) const
{
    const float px = float(point.x());
    const float py = float(point.y());
    for (auto it = m_cells.rbegin(); it != m_cells.rend(); ++it) {
        if (geom::shearedContains(it->cx, it->cy, it->hw, it->hh, 0.0f, 0.0f, px, py))
            return it->row;
    }
    return -1;
}

QRectF WallLayout::cardRect(int row) const
{
    for (const auto &c : m_cells) {
        if (c.row == row)
            return QRectF(c.cx - c.hw, c.cy - c.hh, c.hw * 2.0f, c.hh * 2.0f);
    }
    return QRectF();
}

QRectF WallLayout::stageRect(const LayoutContext &ctx) const
{
    const QPointF c = compositionCenter(ctx);
    const double w = m_live.totalW();
    const double h = m_viewHeight > 0 ? m_viewHeight : m_live.totalH();
    return QRectF(c.x() - w * 0.5, c.y() - h * 0.5, w, h);
}

QPointF WallLayout::compositionCenter(const LayoutContext &ctx) const
{
    const float vw = float(ctx.viewport.width());
    double cx = geom::centerLayout(vw, 0.0f).centerX;
    const double cy = ctx.viewport.height() * 0.5 + ctx.barReserve.height() * 0.5;
    return QPointF(cx, cy);
}

int WallLayout::step(const LayoutContext &ctx, int dx, int dy) const
{
    const int count = ctx.count;
    if (count <= 0)
        return ctx.current;
    const int cols = std::max(m_live.cols, 1);
    const int cur = std::clamp(ctx.current, 0, count - 1);
    if (dx != 0) {
        const int n = cur + (dx > 0 ? 1 : -1);
        if (n >= 0 && n < count)
            return n;
    } else if (dy != 0) {
        const int n = cur + (dy > 0 ? cols : -cols);
        if (n >= 0 && n < count)
            return n;
    }
    return cur;
}

int WallLayout::page(const LayoutContext &ctx, int dir) const
{
    Q_UNUSED(ctx)
    const int visibleRows = std::max(1, int(m_viewHeight / std::max(m_live.cellH(), 1.0f)));
    return dir * visibleRows * std::max(m_live.cols, 1);
}

int WallLayout::wheel(const LayoutContext &ctx, QPointF angle, QPointF pixels)
{
    Q_UNUSED(ctx)
    const float amount = wheelAmount(angle, pixels);
    if (amount != 0.0f)
        m_camera.target += double(-amount * m_live.cellH());
    return 0;
}

void WallLayout::setScrollPosition(const LayoutContext &ctx, qreal position)
{
    Q_UNUSED(ctx)
    m_camera.target = std::clamp(double(position), 0.0, double(m_maxScroll));
}

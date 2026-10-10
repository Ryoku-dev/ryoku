#include "hexlayout.h"

#include "geometry.h"

#include <algorithm>
#include <climits>
#include <cmath>
#include <utility>

namespace {

constexpr float kPi = 3.14159265358979f;

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

HexLayout::Curve parseCurve(const QString &v)
{
    if (v == QLatin1String("flat"))
        return HexLayout::Flat;
    if (v == QLatin1String("wave"))
        return HexLayout::Wave;
    if (v == QLatin1String("s"))
        return HexLayout::S;
    if (v == QLatin1String("cylinder"))
        return HexLayout::Cylinder;
    return HexLayout::Arc;
}

HexLayout::Shape parseShape(const QString &v)
{
    if (v == QLatin1String("triangle"))
        return HexLayout::Triangle;
    if (v == QLatin1String("diamond"))
        return HexLayout::Diamond;
    if (v == QLatin1String("rhombus"))
        return HexLayout::Rhombus;
    return HexLayout::Hexagon;
}

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

bool hexNeighbour(int idx, int cur, int count, int rows)
{
    return idx == cur || (idx == cur - 1 && idx >= 0) || (idx == cur + 1 && idx < count)
        || (idx == cur - rows && idx >= 0) || (idx == cur + rows && idx < count);
}

} // namespace

HexLayout::HexLayout()
{
    m_colScale.reserve(64);
    m_colSeen.reserve(64);
    m_selFades.reserve(8);
    m_centers.reserve(96);
    m_deferred.reserve(16);
}

QString HexLayout::key() const
{
    return QStringLiteral("hex");
}

float HexLayout::Params::hexH() const
{
    return std::ceil(r * 1.73205f);
}

float HexLayout::Params::itemHalfW() const
{
    return shape == Rhombus ? 2.0f * r * aspect : r * aspect;
}

float HexLayout::Params::itemHalfH() const
{
    return shape == Diamond ? hexH() : hexH() / 2.0f;
}

float HexLayout::Params::stepX() const
{
    switch (shape) {
    case Triangle:
        return r * aspect + gapX;
    case Diamond:
    case Rhombus:
        return itemHalfW() + gapX;
    default:
        return 1.5f * r * aspect + gapX;
    }
}

float HexLayout::Params::stepY() const
{
    return itemHalfH() * 2.0f + gapY;
}

float HexLayout::Params::contentH() const
{
    const float tail =
        (shape == Hexagon || shape == Diamond || shape == Rhombus) ? stepY() * stagger : 0.0f;
    return float(std::max(rows - 1, 0)) * stepY() + itemHalfH() * 2.0f + tail;
}

float HexLayout::Params::visibleBand() const
{
    return columnX(std::max(cols - 1, 0)) + itemHalfW();
}

float HexLayout::Params::columnX(int col) const
{
    return float(col) * stepX() + itemHalfW();
}

float HexLayout::Params::rowY(int row) const
{
    return float(row) * stepY() + itemHalfH();
}

float HexLayout::Params::staggerOffset(int col) const
{
    if ((col % 2) == 0 || !(shape == Hexagon || shape == Diamond || shape == Rhombus))
        return 0.0f;
    return stepY() * stagger;
}

void HexLayout::Params::deform(int index, float x, float y, float ox, float oy, float &outX,
                               float &outY) const
{
    float lx = x - ox;
    float ly = y - oy;
    const float radius = std::max(orbitRadius, 1.0f);
    const float orbitAmt = (curve == Cylinder) ? std::clamp(curveStrength, -1.0f, 1.0f) : orbit;
    const float mix = std::clamp(std::abs(orbitAmt), 0.0f, 1.0f);
    if (mix > 0.0f) {
        const float sign = orbitAmt < 0.0f ? -1.0f : 1.0f;
        const float angle = lx / radius * sign;
        const float ring = std::max(radius + ly, radius * 0.08f);
        lx = geom::lerp(lx, std::sin(angle) * ring, mix);
        ly = geom::lerp(ly, radius - std::cos(angle) * ring, mix);
    }
    if (std::abs(twist) > 0.001f) {
        const float angle = twist * kPi / 180.0f * std::hypot(lx, ly) / radius;
        const float s = std::sin(angle), c = std::cos(angle);
        const float nx = lx * c - ly * s;
        ly = lx * s + ly * c;
        lx = nx;
    }
    if (scatter > 0.001f) {
        lx += geom::signedHash(unsigned(index), 0x4f1bbcddu) * scatter;
        ly += geom::signedHash(unsigned(index), 0x9e3779b9u) * scatter;
    }
    outX = ox + lx;
    outY = oy + ly;
}

void HexLayout::Params::morphToward(const Params &t, float amt)
{
    parallax = t.parallax;
    rows = t.rows;
    cols = t.cols;
    scrollStep = t.scrollStep;
    curve = t.curve;
    shape = t.shape;
    r = geom::lerp(r, t.r, amt);
    curveStrength = geom::lerp(curveStrength, t.curveStrength, amt);
    curveFrequency = geom::lerp(curveFrequency, t.curveFrequency, amt);
    gapX = geom::lerp(gapX, t.gapX, amt);
    gapY = geom::lerp(gapY, t.gapY, amt);
    aspect = geom::lerp(aspect, t.aspect, amt);
    stagger = geom::lerp(stagger, t.stagger, amt);
    lens = geom::lerp(lens, t.lens, amt);
    lensRadius = geom::lerp(lensRadius, t.lensRadius, amt);
    orbit = geom::lerp(orbit, t.orbit, amt);
    orbitRadius = geom::lerp(orbitRadius, t.orbitRadius, amt);
    twist = geom::lerp(twist, t.twist, amt);
    scatter = geom::lerp(scatter, t.scatter, amt);
    stage.morphToward(t.stage, amt);
}

bool HexLayout::Params::settledTo(const Params &t) const
{
    return parallax == t.parallax && rows == t.rows && cols == t.cols
        && scrollStep == t.scrollStep && curve == t.curve && shape == t.shape && geom::feq(r, t.r)
        && geom::feq(curveStrength, t.curveStrength) && geom::feq(curveFrequency, t.curveFrequency)
        && geom::feq(gapX, t.gapX) && geom::feq(gapY, t.gapY) && geom::feq(aspect, t.aspect)
        && geom::feq(stagger, t.stagger) && geom::feq(lens, t.lens)
        && geom::feq(lensRadius, t.lensRadius) && geom::feq(orbit, t.orbit)
        && geom::feq(orbitRadius, t.orbitRadius) && geom::feq(twist, t.twist)
        && geom::feq(scatter, t.scatter) && stage.settledTo(t.stage);
}

void HexLayout::Stage::transform(float x, float y, float ox, float oy, float vw, float vh,
                                 float &outX, float &outY, float &outScale) const
{
    const float rad = rotation * kPi / 180.0f;
    const float s = std::sin(rad), c = std::cos(rad);
    const float lx = (x - ox) * scale;
    const float ly = (y - oy) * scale;
    const float shx = lx + shearX * ly;
    const float shy = ly + shearY * lx;
    const float rx = shx * c - shy * s;
    const float ry = shx * s + shy * c;
    const float drad = depthAngle * kPi / 180.0f;
    const float axis = rx * std::sin(drad) + ry * std::cos(drad);
    const float depth = std::clamp(1.0f + perspective * axis / std::max(vh * 0.5f, 1.0f), 0.35f, 2.0f);
    outX = ox + offsetX * vw * 0.5f + rx * depth;
    outY = oy + offsetY * vh * 0.5f + ry;
    outScale = scale * depth;
}

void HexLayout::Stage::morphToward(const Stage &t, float amt)
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

bool HexLayout::Stage::settledTo(const Stage &t) const
{
    return geom::feq(offsetX, t.offsetX) && geom::feq(offsetY, t.offsetY)
        && geom::feq(scale, t.scale) && geom::feq(rotation, t.rotation)
        && geom::feq(perspective, t.perspective) && geom::feq(shearX, t.shearX)
        && geom::feq(shearY, t.shearY) && geom::feq(depthAngle, t.depthAngle);
}

HexLayout::Stage HexLayout::readStage(const LayoutContext &ctx) const
{
    const ParamSource *p = ctx.params;
    Stage s;
    s.offsetX = std::clamp(float(p->num(wsKey("hexStageX"), 0)) / 100.0f, -1.0f, 1.0f);
    s.offsetY = std::clamp(float(p->num(wsKey("hexStageY"), 0)) / 100.0f, -1.0f, 1.0f);
    s.scale = std::clamp(float(p->num(wsKey("hexStageScale"), 100)) / 100.0f, 0.25f, 2.5f);
    s.rotation = std::clamp(float(p->num(wsKey("hexStageRotation"), 0)), -45.0f, 45.0f);
    s.perspective = std::clamp(float(p->num(wsKey("hexStagePerspective"), 0)) / 100.0f, -0.8f, 0.8f);
    s.shearX = std::clamp(float(p->num(wsKey("hexStageShearX"), 0)) / 100.0f, -1.0f, 1.0f);
    s.shearY = std::clamp(float(p->num(wsKey("hexStageShearY"), 0)) / 100.0f, -1.0f, 1.0f);
    s.depthAngle = std::clamp(float(p->num(wsKey("hexStageDepthAngle"), 0)), -180.0f, 180.0f);
    return s;
}

HexLayout::Params HexLayout::readParams(const LayoutContext &ctx) const
{
    const ParamSource *p = ctx.params;
    const bool small = p->smallScreen();
    Params np;
    np.parallax = p->flag(wsKey("hexParallax"), false);
    np.r = std::clamp(float(p->num(wsKey("hexRadius"), small ? 100 : 140)), 20.0f, 2048.0f);
    np.rows = int(std::lround(std::clamp(p->num(wsKey("hexRows"), 3), 1.0, 64.0)));
    np.cols = int(std::lround(std::clamp(p->num(wsKey("hexCols"), small ? 5 : 7), 3.0, 128.0)));
    np.scrollStep = int(std::lround(std::clamp(p->num(wsKey("hexScrollStep"), 1), 1.0, 128.0)));
    const bool arc = p->flag(wsKey("hexArc"), true);
    np.curve = arc ? parseCurve(p->text(wsKey("hexCurve"), QStringLiteral("arc"))) : Flat;
    np.curveStrength = std::clamp(float(p->num(wsKey("hexArcIntensityX10"), 12)) / 10.0f,
                                  -8.0f, 8.0f);
    np.curveFrequency = std::clamp(float(p->num(wsKey("hexCurveFrequency"), 1)), 0.25f, 6.0f);
    np.shape = parseShape(p->text(wsKey("hexShape"), QStringLiteral("hexagon")));
    np.gapX = std::clamp(float(p->num(wsKey("hexGapX"), 6)), -100.0f, 256.0f);
    np.gapY = std::clamp(float(p->num(wsKey("hexGapY"), 6)), -100.0f, 256.0f);
    np.aspect = std::clamp(float(p->num(wsKey("hexAspect"), 100)) / 100.0f, 0.5f, 2.0f);
    np.stagger = std::clamp(float(p->num(wsKey("hexStagger"), 50)) / 100.0f, -1.0f, 1.5f);
    np.lens = std::clamp(float(p->num(wsKey("hexLens"), 0)) / 100.0f, -0.5f, 1.5f);
    np.lensRadius = std::clamp(float(p->num(wsKey("hexLensRadius"), 620)), 100.0f, 2400.0f);
    np.orbit = std::clamp(float(p->num(wsKey("hexOrbit"), 0)) / 100.0f, -1.0f, 1.0f);
    np.orbitRadius = std::clamp(float(p->num(wsKey("hexOrbitRadius"), 620)), 100.0f, 2400.0f);
    np.twist = std::clamp(float(p->num(wsKey("hexTwist"), 0)), -180.0f, 180.0f);
    np.scatter = std::clamp(float(p->num(wsKey("hexScatter"), 0)), 0.0f, 160.0f);
    np.stage = readStage(ctx);
    return np;
}

void HexLayout::configure(const LayoutContext &ctx, bool animate)
{
    const Params np = readParams(ctx);
    if (animate && m_haveParams) {
        m_target = np;
        m_live.rows = np.rows;
        m_live.cols = np.cols;
        m_live.scrollStep = np.scrollStep;
        m_live.curve = np.curve;
        m_live.shape = np.shape;
        m_live.parallax = np.parallax;
    } else {
        m_live = np;
        m_target = np;
        m_colScale.clear();
        m_colSeen.clear();
        m_selFades.clear();
        m_snapCamera = true;
        m_camera = Spring::forDuration(np.r, cameraMs(ctx));
    }
    m_camera.setDuration(cameraMs(ctx));
    m_haveParams = true;
}

void HexLayout::reset(const LayoutContext &ctx)
{
    Q_UNUSED(ctx)
    m_colScale.clear();
    m_colSeen.clear();
    m_selFades.clear();
    m_snapCamera = true;
}

void HexLayout::setSelection(int row, double target, double ms, double defaultStart)
{
    for (auto &s : m_selFades) {
        if (s.first == row) {
            s.second.setDuration(ms);
            s.second.target = target;
            return;
        }
    }
    Spring sp = Spring::forDuration(defaultStart, ms);
    sp.target = target;
    m_selFades.push_back({row, sp});
}

void HexLayout::select(const LayoutContext &ctx, int from, int to)
{
    const double ms = ctx.motion->ms(MotionProfile::Standard);
    setSelection(from, 0.0, ms, 1.0);
    setSelection(to, 1.0, ms, 0.0);
}

bool HexLayout::tick(const LayoutContext &ctx, double dt)
{
    bool moving = m_camera.tick(dt);

    const int hi = std::min(m_colHi, int(m_colScale.size()) - 1);
    for (int c = std::max(m_colLo, 0); c <= hi; ++c) {
        if (m_colSeen[std::size_t(c)])
            moving = m_colScale[std::size_t(c)].tick(dt) || moving;
    }

    for (std::size_t i = 0; i < m_selFades.size();) {
        const bool mv = m_selFades[i].second.tick(dt);
        moving = moving || mv;
        if (!mv && std::abs(m_selFades[i].second.x - m_selFades[i].second.target) < 1e-3) {
            m_selFades[i] = m_selFades.back();
            m_selFades.pop_back();
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

float HexLayout::hexCamera(float selCenter, float cx, float fadeZone, float vw) const
{
    const float leftBound = fadeZone + m_live.stepX();
    const float rightBound = vw - fadeZone - m_live.stepX();
    if (rightBound <= leftBound)
        return selCenter;
    const float camT = float(m_camera.target);
    const float screenX = cx - camT + selCenter;
    if (screenX < leftBound)
        return camT + screenX - leftBound;
    if (screenX > rightBound)
        return camT + screenX - rightBound;
    return camT;
}

float HexLayout::hexScale(int col, float colCenter, float fadeZone, float vw, double colMs)
{
    const float left = fadeZone;
    const float right = vw - fadeZone;
    const float halfBand = std::max((right - left) * 0.5f, 1.0f);
    const float mid = (left + right) * 0.5f;
    const float dn = std::abs(colCenter - mid) / halfBand;
    const float base = 1.0f - 0.2f * geom::smoothstep((dn - 0.7f) / 0.3f);
    const float over = std::max(std::max(left - colCenter, colCenter - right), 0.0f);
    const float cut = std::clamp(1.0f - over / (m_live.stepX() * 1.2f), 0.0f, 1.0f);
    const float target = (m_live.curve == Flat) ? 1.0f : base * cut * cut;

    if (int(m_colScale.size()) <= col) {
        m_colScale.resize(std::size_t(col) + 1);
        m_colSeen.resize(std::size_t(col) + 1, 0);
    }
    if (!m_colSeen[std::size_t(col)]) {
        m_colScale[std::size_t(col)] = Spring::forDuration(target, colMs);
        m_colSeen[std::size_t(col)] = 1;
        return target;
    }
    m_colScale[std::size_t(col)].target = target;
    return std::clamp(float(m_colScale[std::size_t(col)].x), 0.0f, 1.0f);
}

void HexLayout::hexOffsets(int col, float colCenter, float cx, float &curve, float &odd) const
{
    const Params &p = m_live;
    const float normalized = (colCenter - cx) / std::max(cx, 1.0f);
    const float phase = normalized * kPi * p.curveFrequency;
    float cv = 0.0f;
    switch (p.curve) {
    case Flat:
    case Cylinder:
        cv = 0.0f;
        break;
    case Arc:
        cv = -normalized * normalized * p.r;
        break;
    case Wave:
        cv = std::sin(phase) * p.r;
        break;
    case S:
        cv = normalized * normalized * normalized * p.r;
        break;
    }
    curve = cv * p.curveStrength;
    odd = p.staggerOffset(col);
}

void HexLayout::build(const LayoutContext &ctx, std::vector<CardVisual> &out)
{
    if (!m_haveParams)
        configure(ctx, false);

    m_centers.clear();
    m_deferred.clear();
    const int count = ctx.count;
    if (count <= 0) {
        m_colLo = 0;
        m_colHi = -1;
        return;
    }

    const Params &p = m_live;
    const float vw = float(ctx.viewport.width());
    const float vh = float(ctx.viewport.height());
    const float entrance = float(ctx.entrance);
    const int rows = std::max(p.rows, 1);
    const int totalCols = (count + rows - 1) / rows;
    const int current = std::clamp(ctx.current, 0, count - 1);
    const int selCol = current / rows;

    const float contentH = p.contentH();
    const float cardH = contentH + geom::kTopBar + 90.0f;
    const float cy = vh * 0.5f;
    const float viewTop = cy - cardH * 0.5f + geom::kTopBar + 15.0f;
    const float viewH = cardH - geom::kTopBar - 15.0f - 20.0f;
    const float yOffset = std::max((viewH - contentH) * 0.5f, 0.0f);

    const auto cl = geom::centerLayout(vw, 0.0f);
    const float cx = cl.centerX;
    m_lastCenterX = cx;
    const float fadeZone = std::max((cl.availW - p.visibleBand()) * 0.5f, 0.0f);
    const float selCenter = p.columnX(selCol);
    const float minCam = p.r;
    const float maxCam = p.columnX(std::max(totalCols - 1, 0));

    float target = hexCamera(selCenter, cx, fadeZone, vw);
    target = std::clamp(target, std::min(minCam, maxCam), std::max(minCam, maxCam));
    if (m_snapCamera || !m_live.settledTo(m_target)) {
        m_camera.snap(target);
        m_snapCamera = false;
    } else {
        m_camera.target = target;
    }
    const float cam = float(m_camera.x);

    const float coarseMargin = vw + std::hypot(p.itemHalfW(), p.itemHalfH()) * 2.0f;
    float lensX, lensY, lensS;
    p.stage.transform(cx, cy, cx, cy, vw, vh, lensX, lensY, lensS);
    const double colMs = ctx.motion->ms(MotionProfile::Standard);

    int firstCol = -1;
    int lastCol = -1;
    for (int col = 0; col < totalCols; ++col) {
        const float rawX = cx - cam + p.columnX(col);
        if (rawX < -coarseMargin || rawX > vw + coarseMargin)
            continue;
        const float orbitWindow = std::max(p.orbitRadius * kPi * 1.25f, vw * 1.75f);
        if (std::abs(p.orbit) > 0.001f && std::abs(rawX - cx) > orbitWindow)
            continue;
        float dcx, dcy;
        p.deform(col * rows, rawX, cy, cx, cy, dcx, dcy);
        float scx, scy, sscale;
        p.stage.transform(dcx, dcy, cx, cy, vw, vh, scx, scy, sscale);
        const float scale = hexScale(col, scx, fadeZone, vw, colMs);
        if (scale <= 0.01f)
            continue;
        float curve, odd;
        hexOffsets(col, rawX, cx, curve, odd);
        if (firstCol < 0)
            firstCol = col;
        lastCol = col;
        for (int row = 0; row < rows; ++row) {
            const int idx = col * rows + row;
            if (idx >= count)
                break;
            const float rawY = viewTop + yOffset + p.rowY(row) + odd + curve;
            float dx, dy;
            p.deform(idx, rawX, rawY, cx, cy, dx, dy);
            float ix, iy, istage;
            p.stage.transform(dx, dy, cx, cy, vw, vh, ix, iy, istage);
            const float distance = std::hypot(ix - lensX, iy - lensY);
            const float lensT = std::clamp(1.0f - distance / std::max(p.lensRadius, 1.0f), 0.0f, 1.0f);
            const float lensScale = std::max(1.0f + p.lens * geom::smoothstep(lensT), 0.15f);
            const float renderScale = scale * istage * lensScale;
            const float cardHw = p.itemHalfW() * renderScale;
            const float cardHh = p.itemHalfH() * renderScale;
            const float guard = 10.0f;
            if (ix + cardHw < -guard || ix - cardHw > vw + guard || iy + cardHh < -guard
                || iy - cardHh > vh + guard)
                continue;
            hexCard(ctx, out, idx, ix, iy, renderScale, entrance, cx);
        }
    }

    std::sort(m_deferred.begin(), m_deferred.end(),
              [](const Deferred &a, const Deferred &b) { return a.key < b.key; });
    for (auto &d : m_deferred) {
        if (d.hasShadow)
            out.push_back(d.shadow);
        out.push_back(d.body);
        m_centers.push_back(d.center);
    }
    m_colLo = firstCol < 0 ? 0 : firstCol;
    m_colHi = lastCol;
}

void HexLayout::hexCard(const LayoutContext &ctx, std::vector<CardVisual> &out, int idx,
                        float colCenter, float itemCy, float scale, float entrance, float centerX)
{
    const Params &p = m_live;
    const int rows = std::max(p.rows, 1);
    const int row = idx % rows;
    const int col = idx / rows;
    const int triDir = (p.shape == Triangle) ? (((row + col) % 2 == 0) ? 0 : 1) : 0;
    uint32_t shapeCode = 0;
    switch (p.shape) {
    case Hexagon:
        shapeCode = 0;
        break;
    case Triangle:
        shapeCode = 1u + uint32_t(triDir);
        break;
    case Diamond:
        shapeCode = 5;
        break;
    case Rhombus:
        shapeCode = 6;
        break;
    }

    const int current = std::clamp(ctx.current, 0, ctx.count - 1);
    const bool isSel = idx == current;
    const bool isHover = (idx == ctx.hovered) && !isSel;
    float selT = isSel ? 1.0f : 0.0f;
    for (const auto &s : m_selFades) {
        if (s.first == idx) {
            selT = std::clamp(float(s.second.x), 0.0f, 1.0f);
            break;
        }
    }

    const float hw = p.itemHalfW() * scale;
    const float hh = p.itemHalfH() * scale;
    const float vw = float(ctx.viewport.width());

    CardVisual body;
    body.row = idx;
    body.texture = CardVisual::TextureAuto;
    body.inst.rect[0] = colCenter;
    body.inst.rect[1] = itemCy;
    body.inst.rect[2] = hw;
    body.inst.rect[3] = hh;
    body.inst.params[2] = std::clamp(scale, 0.0f, 1.0f) * entrance;
    body.inst.misc[3] = CardFlag::shape(shapeCode);
    if (selT > 0.01f) {
        setColor(body.inst.border, ctx.palette.primary, 0.55f + 0.45f * selT);
        body.inst.params[1] = 1.5f + 1.5f * selT;
    } else if (isHover) {
        setColor(body.inst.border, ctx.palette.primary, 0.55f);
        body.inst.params[1] = 2.0f;
    } else {
        body.inst.border[0] = 0.0f;
        body.inst.border[1] = 0.0f;
        body.inst.border[2] = 0.0f;
        body.inst.border[3] = 0.5f;
        body.inst.params[1] = 1.5f;
    }
    body.cover = true;
    body.cropAspect = 0.0f;
    if (p.parallax) {
        const float span = (colCenter < centerX) ? centerX : (vw - centerX);
        const float pos = (colCenter - centerX) / std::max(span + hw, 1.0f);
        body.cropZoom = 1.4f;
        body.cropShiftX = std::clamp(pos, -1.0f, 1.0f);
        body.cropShiftY = 0.0f;
    } else {
        body.cropZoom = 1.3f; // inner zoom so the thumbnail fills the tile edges
    }
    body.wantNear = hexNeighbour(idx, current, ctx.count, rows);

    const Center ctr{idx, colCenter, itemCy, hw, hh, triDir};
    const float deferKey = selT + (isHover ? 0.3f : 0.0f);
    if (deferKey > 0.01f) {
        Deferred d;
        d.key = deferKey;
        d.center = ctr;
        d.body = body;
        d.hasShadow = selT > 0.01f;
        if (d.hasShadow) {
            CardVisual sh;
            sh.row = -1;
            sh.texture = CardVisual::TextureNone;
            sh.inst.rect[0] = colCenter + 3.0f;
            sh.inst.rect[1] = itemCy + 7.0f;
            sh.inst.rect[2] = hw;
            sh.inst.rect[3] = hh;
            sh.inst.fill[0] = 0.0f;
            sh.inst.fill[1] = 0.0f;
            sh.inst.fill[2] = 0.0f;
            sh.inst.fill[3] = 0.35f * selT * scale * entrance;
            sh.inst.params[2] = 1.0f;
            sh.inst.misc[3] = CardFlag::shape(shapeCode);
            d.shadow = sh;
        }
        m_deferred.push_back(std::move(d));
    } else {
        out.push_back(body);
        m_centers.push_back(ctr);
    }
}

int HexLayout::hitTest(QPointF point) const
{
    const float px = float(point.x());
    const float py = float(point.y());
    const Shape shape = m_live.shape;
    for (auto it = m_centers.rbegin(); it != m_centers.rend(); ++it) {
        const float hw = it->hw, hh = it->hh;
        if (hw <= 0.0f || hh <= 0.0f)
            continue;
        bool hit;
        if (shape == Diamond || shape == Rhombus) {
            hit = (std::abs(px - it->cx) / hw + std::abs(py - it->cy) / hh) <= 1.0f;
        } else if (shape == Triangle) {
            float x = (px - it->cx) / hw;
            float y = (py - it->cy) / hh;
            switch (it->triDir) {
            case 1:
                y = -y;
                break;
            case 2:
                std::swap(x, y);
                break;
            case 3: {
                const float t = y;
                y = -x;
                x = t;
                break;
            }
            default:
                break;
            }
            auto side = [&](float ax, float ay, float bx, float by) {
                return (x - bx) * (ay - by) - (ax - bx) * (y - by);
            };
            const float d1 = side(0.0f, -1.0f, 1.0f, 1.0f);
            const float d2 = side(1.0f, 1.0f, -1.0f, 1.0f);
            const float d3 = side(-1.0f, 1.0f, 0.0f, -1.0f);
            const bool neg = d1 < 0.0f || d2 < 0.0f || d3 < 0.0f;
            const bool pos = d1 > 0.0f || d2 > 0.0f || d3 > 0.0f;
            hit = !(neg && pos);
        } else {
            const float qx = std::abs(px - it->cx);
            const float qy = std::abs(py - it->cy);
            hit = qy <= hh && (qx / hw + 0.5f * qy / hh) <= 1.0f;
        }
        if (hit)
            return it->row;
    }
    return -1;
}

QRectF HexLayout::cardRect(int row) const
{
    for (const auto &c : m_centers) {
        if (c.row == row)
            return QRectF(c.cx - c.hw, c.cy - c.hh, c.hw * 2.0f, c.hh * 2.0f);
    }
    return QRectF();
}

QRectF HexLayout::stageRect(const LayoutContext &ctx) const
{
    const double vh = ctx.viewport.height();
    const double h = m_live.contentH() + geom::kTopBar + 90.0;
    return QRectF(0, (vh - h) * 0.5, ctx.viewport.width(), h);
}

int HexLayout::step(const LayoutContext &ctx, int dx, int dy) const
{
    if ((dx == 0 && dy == 0) || m_centers.empty())
        return ctx.current;
    const Center *origin = nullptr;
    for (const auto &c : m_centers) {
        if (c.row == ctx.current) {
            origin = &c;
            break;
        }
    }
    if (!origin)
        return ctx.current;

    float dirX = float(dx);
    float dirY = float(dy);
    const float mag = std::sqrt(dirX * dirX + dirY * dirY);
    dirX /= mag;
    dirY /= mag;

    // Nearest neighbour within a 60-degree cone, so four arrows reach all six neighbours.
    int best = -1;
    float bestScore = 1e30f;
    for (const auto &c : m_centers) {
        if (c.row == ctx.current)
            continue;
        const float ex = c.cx - origin->cx;
        const float ey = c.cy - origin->cy;
        const float proj = ex * dirX + ey * dirY;
        if (proj <= 1.0f)
            continue;
        const float perp = std::abs(ex * (-dirY) + ey * dirX);
        if (perp > proj * 1.732f)
            continue;
        const float score = proj + perp * 2.0f;
        if (score < bestScore) {
            bestScore = score;
            best = c.row;
        }
    }
    return best >= 0 ? best : ctx.current;
}

int HexLayout::wheel(const LayoutContext &ctx, QPointF angle, QPointF pixels)
{
    const int count = ctx.count;
    if (count <= 0)
        return 0;
    const float amount = wheelAmount(angle, pixels);
    if (amount == 0.0f)
        return 0;
    const int rows = std::max(m_live.rows, 1);
    const int totalCols = (count + rows - 1) / rows;
    const int current = std::clamp(ctx.current, 0, count - 1);
    const int col = current / rows;
    const int dir = amount > 0.0f ? 1 : -1;
    const int newCol = std::clamp(col - dir * std::max(m_live.scrollStep, 1), 0, totalCols - 1);
    const int target = std::clamp(newCol * rows + (current % rows), 0, count - 1);
    return target - current;
}

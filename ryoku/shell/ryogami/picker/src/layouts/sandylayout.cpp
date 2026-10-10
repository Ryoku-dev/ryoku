#include "sandylayout.h"

#include "../scene/cardsource.h"

#include <algorithm>
#include <cmath>

namespace {

// Breathing room under the strip, and the least the hero keeps clear at the top; the
// masthead's own band (ctx.barReserve) wins when it reaches further down.
constexpr double kFootZone = 48.0;
constexpr double kTopClear = 50.0;
constexpr double kLodVelLo = 1.5;
constexpr double kLodVelHi = 6.0;
constexpr double kPi = 3.14159265358979323846;

double smoothstep(double t)
{
    t = std::clamp(t, 0.0, 1.0);
    return t * t * (3.0 - 2.0 * t);
}

double window(double p, double start, double end)
{
    const double span = end - start;
    if (std::abs(span) < 1e-9)
        return p >= end ? 1.0 : 0.0;
    return smoothstep((p - start) / span);
}

double lerp(double a, double b, double t)
{
    return a + (b - a) * t;
}

bool feq(double a, double b)
{
    return std::abs(a - b) < 0.05;
}

double sandyMotionFromVel(double vel, double lo, double hi)
{
    return smoothstep((std::abs(vel) - lo) / std::max(hi - lo, 1e-3));
}

double sandyLodGrain(double base, double lodMax, double motion)
{
    return std::clamp(base * (1.0 + (lodMax - 1.0) * std::clamp(motion, 0.0, 1.0)), 1.0, 32.0);
}

void grainGrid(double grain, double halfW, double halfH, double &columns, double &rows)
{
    grain = std::max(grain, 1.0);
    columns = std::clamp(std::round(halfW * 2.0 / grain), 32.0, 640.0);
    rows = std::clamp(std::round(halfH * 2.0 / grain), 18.0, 360.0);
}

double clampCam(double camera, int count)
{
    return std::clamp(camera, 0.0, double(std::max(count - 1, 0)));
}

double edgePan(double x, double y, double vw, double vh, double stripHeight)
{
    const double bottom = vh - kFootZone;
    if (y < bottom - stripHeight || y > bottom)
        return 0.0;
    const double zone = std::max(vw * 0.14, 1.0);
    if (x < zone)
        return -std::clamp(1.0 - x / zone, 0.0, 1.0);
    if (x > vw - zone)
        return std::clamp(1.0 - (vw - x) / zone, 0.0, 1.0);
    return 0.0;
}

struct Place {
    int i = 0;
    double x = 0;
    double s = 0;
};

double stripScroll(double camera, int count, double stride, double margin, double availW)
{
    const double usable = std::max(availW - 2.0 * margin, stride);
    const double contentSpan = (std::max(count, 1) - 1.0) * stride;
    const double maxScroll = std::max(contentSpan - usable, 0.0);
    return std::clamp(camera * stride - usable * 0.5, 0.0, maxScroll);
}

std::vector<Place> stripPlace(int count, double camera, double stride, double centerX, double availW)
{
    std::vector<Place> places;
    if (count == 0)
        return places;
    stride = std::max(stride, 8.0);
    const double margin = stride;
    const double left = centerX - availW * 0.5;
    const double right = centerX + availW * 0.5;
    const double usable = std::max(availW - 2.0 * margin, stride);
    const double contentSpan = double(std::max(count - 1, 0)) * stride;
    const double scroll = stripScroll(camera, count, stride, margin, availW);
    const double origin =
        contentSpan < usable ? centerX - contentSpan * 0.5 : left + margin - scroll;
    const long long span = (long long)std::ceil((availW + margin) / stride) + 1;
    const long long anchor = (long long)std::llround(scroll / stride) + 1;
    const long long lo = std::max<long long>(anchor - span, 0);
    const long long hi = std::min<long long>(anchor + span, count - 1);
    for (long long index = lo; index <= hi; ++index) {
        const double x = origin + double(index) * stride;
        const double distanceToEdge = std::min(x - left, right - x);
        const double fade = std::clamp(distanceToEdge / margin, 0.0, 1.0);
        if (fade < 0.05)
            continue;
        places.push_back(Place{int(index), x, fade});
    }
    return places;
}

bool shearedContains(double cx, double cy, double hw, double hh, double skew, double px, double py)
{
    if (hw <= 0.0 || hh <= 0.0)
        return false;
    const double sx = skew * 0.5;
    const double bx = std::max(hw - std::abs(sx), 1.0);
    const double by = std::max(hh, 1.0);
    const double det = bx * by;
    if (std::abs(det) < 1.0)
        return false;
    const double x = px - cx;
    const double y = py - cy;
    const double u = (by * x + sx * y) / det;
    const double v = (bx * y) / det;
    return std::abs(u) <= 1.0 && std::abs(v) <= 1.0;
}

int scrollSteps(double &accumulator, double amount)
{
    if (std::abs(amount) >= 1.0) {
        accumulator = 0.0;
        return int(std::llround(amount));
    }
    accumulator += amount;
    const int steps = int(std::trunc(accumulator));
    accumulator -= double(steps);
    return steps;
}

void setFill(CardInstance &inst, const QColor &c, float alpha)
{
    inst.fill[0] = float(c.redF());
    inst.fill[1] = float(c.greenF());
    inst.fill[2] = float(c.blueF());
    inst.fill[3] = alpha;
}

// The eleven swap styles and their sandy shader indices.
double sandyStyleIndex(const QString &key)
{
    if (key == QLatin1String("hourglass"))
        return 2.0;
    if (key == QLatin1String("castle"))
        return 3.0;
    if (key == QLatin1String("saltation"))
        return 6.0;
    if (key == QLatin1String("pour"))
        return 7.0;
    if (key == QLatin1String("orbit"))
        return 8.0;
    if (key == QLatin1String("burst"))
        return 10.0;
    if (key == QLatin1String("weave"))
        return 11.0;
    if (key == QLatin1String("bloom"))
        return 13.0;
    if (key == QLatin1String("flock"))
        return 16.0;
    if (key == QLatin1String("ring"))
        return 17.0;
    return 1.0;
}

}

SandyLayout::SandyLayout()
{
    m_camera.setDuration(250.0);
    m_pop = Spring::forDuration(1.0, 450.0);
    m_pop.epsilon = 0.05;
    m_bmix = Spring::forDuration(1.0, 180.0);
    m_bmix.epsilon = 0.05;
    m_swirl = Spring::forDuration(0.0, 450.0);
    m_swirl.epsilon = 0.05;
    m_heroFade = Spring::forDuration(1.0, 250.0);
    m_heroFade.epsilon = 0.05;
    m_prog.snap(1.0);
}

SandyLayout::Params SandyLayout::readParams(const LayoutContext &ctx) const
{
    const ParamSource &p = *ctx.params;
    const bool small = p.smallScreen();
    auto sel = [&](const char *key, double wide, double narrow) {
        return p.num(QString::fromLatin1(key), small ? narrow : wide);
    };
    auto num = [&](const char *key, double def) { return p.num(QString::fromLatin1(key), def); };

    Params t;
    t.offsetX = std::clamp(num("components.wallpaperSelector.sandyStageX", 0.0) / 100.0, -1.0, 1.0);
    t.offsetY = std::clamp(num("components.wallpaperSelector.sandyStageY", 0.0) / 100.0, -1.0, 1.0);
    t.centerH = std::clamp(sel("components.wallpaperSelector.sandyCenter", 440.0, 330.0), 160.0, 4096.0);
    t.sliceW = std::clamp(sel("components.wallpaperSelector.sandySliceWidth", 96.0, 68.0), 24.0, 2048.0);
    t.sliceH = std::clamp(sel("components.wallpaperSelector.sandySliceHeight", 180.0, 130.0), 60.0, 2048.0);
    t.skew = std::clamp(sel("components.wallpaperSelector.sandySkew", 12.0, 8.0), -2048.0, 2048.0);
    t.spacing = std::clamp(sel("components.wallpaperSelector.sandySpacing", 26.0, 20.0), 0.0, 1024.0);
    t.durationMs = std::clamp(num("components.wallpaperSelector.sandyDuration", 1250.0), 200.0, 6000.0);
    t.blendMs = std::clamp(num("components.wallpaperSelector.sandyBlend", 700.0), 100.0, 4000.0);
    t.strands = std::clamp(sel("components.wallpaperSelector.sandyStrands", 22.0, 18.0), 2.0, 64.0);
    t.twist = std::clamp(num("components.wallpaperSelector.sandyTwist", 100.0) / 100.0, 0.0, 3.0);
    t.orbit = std::clamp(num("components.wallpaperSelector.sandyOrbit", 100.0) / 100.0, 0.0, 3.0);
    t.turbulence = std::clamp(num("components.wallpaperSelector.sandyTurbulence", 100.0) / 100.0, 0.0, 3.0);
    t.waist = std::clamp(num("components.wallpaperSelector.sandyWaist", 100.0) / 100.0, 0.1, 3.0);
    t.front = std::clamp(num("components.wallpaperSelector.sandyFront", 100.0) / 100.0 * 0.65, 0.1, 1.6);
    t.arc = std::clamp(num("components.wallpaperSelector.sandyArc", 100.0) / 100.0, 0.0, 3.0);
    t.fan = std::clamp(num("components.wallpaperSelector.sandyFan", 100.0) / 100.0 * 0.6, 0.1, 1.2);
    t.edgeSpeed = std::clamp(num("components.wallpaperSelector.sandyEdgeSpeed", 100.0) / 100.0 * 14.0, 0.0, 80.0);
    t.ringSize = std::clamp(num("components.wallpaperSelector.sandyRingSize", 100.0) / 100.0, 0.25, 3.0);
    t.ringSpin = std::clamp(num("components.wallpaperSelector.sandyRingSpin", 100.0) / 100.0, 0.0, 3.0);
    t.ringWave = std::clamp(num("components.wallpaperSelector.sandyRingWave", 100.0) / 100.0, 0.0, 3.0);
    t.ringSoft = std::clamp(num("components.wallpaperSelector.sandyRingSoft", 100.0) / 100.0, 0.25, 3.0);
    t.ringBlend = std::clamp(num("components.wallpaperSelector.sandyRingBlend", 100.0) / 100.0, 0.25, 4.0);
    t.ringHold = std::clamp(num("components.wallpaperSelector.sandyRingHold", 200.0) / 1000.0, 0.03, 2.0);
    t.grain = std::clamp(num("components.wallpaperSelector.sandyGrain", 3.0), 1.0, 32.0);
    t.swapLoop = p.flag(QStringLiteral("components.wallpaperSelector.sandySwapLoop"), false);
    t.videoOutLive = p.flag(QStringLiteral("components.wallpaperSelector.sandyOutgoingLive"), true);
    t.swapStyle = sandyStyleIndex(p.text(QStringLiteral("components.wallpaperSelector.sandySwapStyle"),
                                         QStringLiteral("vortex")));

    if (!p.flag(QStringLiteral("components.wallpaperSelector.roundCorners"), true)) {
        t.corners[0] = t.corners[1] = t.corners[2] = t.corners[3] = 0.0;
    } else {
        const double base = std::clamp(num("components.wallpaperSelector.cornerRadius", 18.0), 0.0, 1024.0);
        t.corners[0] = std::clamp(num("components.wallpaperSelector.cornerTL", base), 0.0, 1024.0);
        t.corners[1] = std::clamp(num("components.wallpaperSelector.cornerTR", base), 0.0, 1024.0);
        t.corners[2] = std::clamp(num("components.wallpaperSelector.cornerBR", base), 0.0, 1024.0);
        t.corners[3] = std::clamp(num("components.wallpaperSelector.cornerBL", base), 0.0, 1024.0);
    }
    return t;
}

void SandyLayout::Params::morphToward(const Params &t, double amount)
{
    offsetX = lerp(offsetX, t.offsetX, amount);
    offsetY = lerp(offsetY, t.offsetY, amount);
    centerH = lerp(centerH, t.centerH, amount);
    sliceW = lerp(sliceW, t.sliceW, amount);
    sliceH = lerp(sliceH, t.sliceH, amount);
    spacing = lerp(spacing, t.spacing, amount);
    skew = lerp(skew, t.skew, amount);
    twist = lerp(twist, t.twist, amount);
    orbit = lerp(orbit, t.orbit, amount);
    turbulence = lerp(turbulence, t.turbulence, amount);
    waist = lerp(waist, t.waist, amount);
    front = lerp(front, t.front, amount);
    fan = lerp(fan, t.fan, amount);
    arc = lerp(arc, t.arc, amount);
    ringSize = lerp(ringSize, t.ringSize, amount);
    ringSpin = lerp(ringSpin, t.ringSpin, amount);
    ringWave = lerp(ringWave, t.ringWave, amount);
    ringSoft = lerp(ringSoft, t.ringSoft, amount);
    grain = lerp(grain, t.grain, amount);
    for (int i = 0; i < 4; ++i)
        corners[i] = lerp(corners[i], t.corners[i], amount);
    durationMs = t.durationMs;
    blendMs = t.blendMs;
    strands = t.strands;
    edgeSpeed = t.edgeSpeed;
    ringBlend = t.ringBlend;
    ringHold = t.ringHold;
    swapLoop = t.swapLoop;
    swapStyle = t.swapStyle;
    videoOutLive = t.videoOutLive;
}

bool SandyLayout::Params::settledTo(const Params &t) const
{
    return feq(offsetX, t.offsetX) && feq(offsetY, t.offsetY) && feq(centerH, t.centerH)
        && feq(sliceW, t.sliceW) && feq(sliceH, t.sliceH) && feq(spacing, t.spacing)
        && feq(skew, t.skew) && feq(twist, t.twist) && feq(orbit, t.orbit)
        && feq(turbulence, t.turbulence) && feq(waist, t.waist) && feq(front, t.front)
        && feq(fan, t.fan) && feq(arc, t.arc) && feq(ringSize, t.ringSize)
        && feq(ringSpin, t.ringSpin) && feq(ringWave, t.ringWave) && feq(ringSoft, t.ringSoft)
        && feq(grain, t.grain) && feq(corners[0], t.corners[0]) && feq(corners[1], t.corners[1])
        && feq(corners[2], t.corners[2]) && feq(corners[3], t.corners[3]);
}

Spring SandyLayout::overrideSpring(double value, double ms, double target)
{
    Spring s = Spring::forDuration(value, std::max(ms, 1.0));
    s.epsilon = 0.05;
    s.target = target;
    return s;
}

void SandyLayout::configure(const LayoutContext &ctx, bool animate)
{
    m_target = readParams(ctx);
    m_lodMax = ctx.params->flag(QStringLiteral("components.wallpaperSelector.sandyLodAuto"), true)
        ? std::clamp(ctx.params->num(QStringLiteral("components.wallpaperSelector.sandyLod"), 2.0), 1.0, 4.0)
        : 1.0;
    m_resScale =
        std::clamp(ctx.params->num(QStringLiteral("components.wallpaperSelector.sandyResScale"), 100.0) / 100.0,
                   0.25, 1.0);
    m_camera.setDuration(ctx.motion->ms(MotionProfile::Standard));
    if (!animate || !m_configured) {
        m_live = m_target;
        m_current = std::clamp(ctx.current, 0, std::max(ctx.count - 1, 0));
        m_camera.snap(clampCam(m_current, ctx.count));
        m_configured = true;
    }
}

void SandyLayout::reset(const LayoutContext &ctx)
{
    settleNow(ctx);
    m_camFree = false;
    m_current = std::clamp(ctx.current, 0, std::max(ctx.count - 1, 0));
    m_camera.snap(clampCam(m_current, ctx.count));
    m_hasPass = false;
}

void SandyLayout::settleNow(const LayoutContext &ctx)
{
    m_prog.snap(1.0);
    m_swirl = overrideSpring(0.0, ctx.motion->ms(MotionProfile::Slow), 0.0);
    m_swirlHold = 0.0;
    m_bmix = overrideSpring(1.0, ctx.motion->ms(MotionProfile::Fast), 1.0);
    m_bfrom = -1;
    m_bfrom2 = -1;
    m_bcut = 1.0;
    m_bto = -1;
    m_stormTo = -1;
    m_ringTo = -1;
    m_carry = 0.0;
}

void SandyLayout::swapPick(int from, int to)
{
    m_bfrom = m_bto >= 0 ? m_bto : from;
    m_bfrom2 = -1;
    m_bcut = 1.0;
    m_bto = to;
    m_bmix = overrideSpring(0.0, std::clamp(m_live.blendMs, 100.0, 4000.0), 1.0);
}

void SandyLayout::ringPick(int from, int to)
{
    m_swirl.target = 1.0;
    m_swirlHold = m_live.ringHold;
    m_bto = -1;
    m_bfrom2 = -1;
    m_bcut = 1.0;
    m_bfrom = m_stormTo >= 0 ? m_stormTo : from;
    m_ringTo = to;
    m_bmix = overrideSpring(0.0, std::clamp(m_live.blendMs * 0.5 * m_live.ringBlend, 150.0, 3000.0), 1.0);
}

void SandyLayout::stormPick(const LayoutContext &ctx, int from, int to, double newDir)
{
    m_from = m_bto >= 0 ? m_bto : from;
    m_bto = -1;
    m_dir = newDir;
    m_carry = 0.0;
    m_bfrom = -1;
    m_bfrom2 = -1;
    m_bcut = 1.0;
    m_stormTo = to;
    m_ringTo = -1;
    m_bmix = overrideSpring(1.0, ctx.motion->ms(MotionProfile::Fast), 1.0);
    m_prog.snap(0.0);
    m_prog.setDurationMs(std::clamp(m_live.durationMs, 200.0, 6000.0));
    m_prog.retarget(1.0);
}

void SandyLayout::ringFade(int to)
{
    const bool done = m_bmix.settled() || m_bmix.x >= 0.90;
    if (!done)
        return;
    const int displayed = m_ringTo >= 0 ? m_ringTo : m_current;
    if (displayed == to) {
        m_ringTo = to;
        return;
    }
    m_bfrom2 = m_bfrom >= 0 ? m_bfrom : displayed;
    m_bcut = m_bmix.settled() ? 1.0 : std::clamp(m_bmix.x, 0.0, 1.0);
    m_bfrom = displayed;
    m_ringTo = to;
    m_bmix = overrideSpring(0.0, std::clamp(m_live.blendMs * 0.5 * m_live.ringBlend, 150.0, 3000.0), 1.0);
}

void SandyLayout::ringChainTick()
{
    const bool ringLive = m_swirl.target > 0.5 || m_swirl.x > 0.001;
    if (ringLive && m_ringTo >= 0 && m_ringTo != m_current)
        ringFade(m_current);
    if (m_bmix.settled() && m_swirl.settled() && m_swirl.x < 0.5) {
        m_bto = -1;
        m_ringTo = -1;
    }
}

bool SandyLayout::heroAnimating() const
{
    return !m_prog.settled() || m_bto >= 0 || m_stormTo >= 0 || m_ringTo >= 0 || m_swirl.target > 0.5
        || m_swirl.x > 0.001;
}

void SandyLayout::select(const LayoutContext &ctx, int from, int to)
{
    m_camFree = false;
    m_pop.snap(0.0);
    m_pop.target = 1.0;
    if (ctx.motion->reduced) {
        settleNow(ctx);
        m_current = to;
        return;
    }
    const bool swirling = m_swirl.target > 0.5 || m_swirl.x > 0.02;
    if (swirling) {
        m_swirl.target = 1.0;
        m_swirlHold = m_live.ringHold;
        m_bto = -1;
        ringFade(to);
        m_current = to;
        return;
    }
    const double newDir = to >= from ? 1.0 : -1.0;
    const bool flipped = newDir != m_dir;
    const bool ringActive = m_swirl.target > 0.5 || m_swirl.x > 0.001;
    const bool blending =
        (m_bfrom >= 0 && !m_bmix.settled() && m_bmix.x < 0.90) || ringActive;
    const bool interrupted = !m_prog.settled() || blending;
    if (interrupted && m_live.swapLoop && !flipped && !blending) {
        swapPick(from, to);
    } else if (interrupted && blending) {
        m_swirl.target = 1.0;
        m_swirlHold = m_live.ringHold;
        m_bto = -1;
        m_stormTo = -1;
        ringFade(to);
    } else if (interrupted) {
        ringPick(from, to);
    } else {
        stormPick(ctx, from, to, newDir);
    }
    m_current = to;
}

bool SandyLayout::tick(const LayoutContext &ctx, double dt)
{
    m_current = std::clamp(ctx.current, 0, std::max(ctx.count - 1, 0));

    bool moving = false;
    if (!m_camFree)
        m_camera.target = clampCam(m_current, ctx.count);
    if (m_edgePan != 0.0 && ctx.count > 0) {
        m_camera.target = clampCam(m_camera.target + m_edgePan * m_live.edgeSpeed * dt, ctx.count);
        moving = true;
    }
    moving |= m_camera.tick(dt);

    if (!ringEngaged())
        moving |= m_prog.tick(dt);
    moving |= m_bmix.tick(dt);
    moving |= m_pop.tick(dt);
    moving |= m_swirl.tick(dt);
    if (m_swirl.target > 0.5) {
        moving = true;
        m_swirlHold -= dt;
        if (m_swirlHold <= 0.0) {
            m_swirl.target = 0.0;
            if (!m_prog.settled())
                m_prog.snap(1.0);
            m_stormTo = -1;
        }
    }
    moving |= m_heroFade.tick(dt);

    if (m_bfrom >= 0 && m_bmix.settled()) {
        m_bfrom = -1;
        m_bfrom2 = -1;
        m_bcut = 1.0;
    }
    ringChainTick();

    const double motionTarget =
        m_lodMax > 1.0 ? sandyMotionFromVel(m_camera.v, kLodVelLo, kLodVelHi) : 0.0;
    const double tau = ctx.motion->tau(MotionProfile::Standard);
    const double next = m_motion + (motionTarget - m_motion) * approachK(dt, tau);
    if (std::abs(next - m_motion) > 1e-3)
        moving = true;
    m_motion = next;

    if (!m_live.settledTo(m_target)) {
        m_live.morphToward(m_target, approachK(dt, tau));
        moving = true;
    }

    return moving || heroAnimating();
}

QString SandyLayout::keyOf(const LayoutContext &ctx, int row) const
{
    if (!ctx.source || row < 0 || row >= ctx.count)
        return QString();
    return ctx.source->cardKey(row);
}

void SandyLayout::buildPass(const LayoutContext &ctx, int current, double prog, double cx, double cy,
                            double chw, double chh)
{
    const int btoRow = m_bto >= 0 ? m_bto : (m_ringTo >= 0 ? m_ringTo : m_stormTo);
    const int fromRow = m_from >= 0 ? m_from : current;
    const int bRow = btoRow >= 0 ? btoRow : current;

    m_pass.keyA = keyOf(ctx, fromRow);
    m_pass.keyB = keyOf(ctx, bRow);
    if (m_bfrom >= 0)
        m_pass.keyB2 = keyOf(ctx, m_bfrom);
    else if (m_live.swapStyle >= 16.5)
        m_pass.keyB2 = keyOf(ctx, fromRow);
    else
        m_pass.keyB2 = QString();
    m_pass.keyB3 = m_bfrom2 >= 0 ? keyOf(ctx, m_bfrom2) : QString();

    m_pass.center[0] = float(cx);
    m_pass.center[1] = float(cy);
    m_pass.hero[0] = float(chw);
    m_pass.hero[1] = float(chh);
    m_pass.resolution[0] = float(ctx.viewport.width());
    m_pass.resolution[1] = float(ctx.viewport.height());

    m_pass.progress = float(prog);
    m_pass.time = float(ctx.time);
    m_pass.dir = float(m_dir);
    const int seedRow = m_stormTo >= 0 ? m_stormTo : (m_ringTo >= 0 ? m_ringTo : current);
    m_pass.seed = float((seedRow % 977) * 0.013);
    m_pass.carry = float(m_carry);
    m_pass.bcut = float(std::clamp(m_bcut, 0.0, 1.0));
    m_pass.bmix = m_pass.keyB2.isEmpty() ? 1.0f : float(std::clamp(m_bmix.x, 0.0, 1.0));
    m_pass.swirl = float(std::clamp(m_swirl.x, 0.0, 1.0));
    m_pass.wave = m_bto >= 0 ? 1.0f : 0.0f;
    m_pass.videoIn = 0.0f;
    m_pass.videoOut = 0.0f;
    m_pass.resScale = float(m_resScale);

    m_pass.ringSpin = float(std::clamp(m_live.ringSpin, 0.0, 3.0));
    m_pass.ringWave = float(std::clamp(m_live.ringWave, 0.0, 3.0));
    m_pass.ringSoft = float(std::clamp(m_live.ringSoft, 0.25, 3.0));
    m_pass.ringSize = float(std::clamp(m_live.ringSize, 0.25, 3.0));

    m_pass.strands = float(std::clamp(m_live.strands, 2.0, 64.0));
    m_pass.twist = float(m_live.twist);
    m_pass.orbit = float(m_live.orbit);
    m_pass.turbulence = float(m_live.turbulence);
    m_pass.waist = float(m_live.waist);
    m_pass.front = float(std::clamp(m_live.front, 0.1, 1.6));
    m_pass.fan = float(std::clamp(m_live.fan, 0.1, 1.2));
    m_pass.arc = float(std::clamp(m_live.arc, 0.0, 3.0));
    m_pass.swapLoop = m_live.swapLoop ? 1.0f : 0.0f;
    m_pass.swapStyle = float(std::clamp(std::round(m_live.swapStyle), 0.0, 17.0));

    const double base = std::max(m_live.grain, 1.0);
    const double grain = sandyLodGrain(base, m_lodMax, m_motion);
    double columns = 0, rows = 0;
    grainGrid(grain, chw, chh, columns, rows);
    m_pass.grid[0] = float(columns);
    m_pass.grid[1] = float(rows);

    m_hasPass = true;
}

void SandyLayout::build(const LayoutContext &ctx, std::vector<CardVisual> &out)
{
    m_placed.clear();
    m_heroRow = -1;
    m_hasPass = false;
    if (ctx.count == 0)
        return;

    const int current = std::clamp(ctx.current, 0, ctx.count - 1);
    m_current = current;
    const double vw = ctx.viewport.width();
    const double vh = ctx.viewport.height();
    const double cx = vw * 0.5 + m_live.offsetX * vw * 0.5;
    const double offsetY = m_live.offsetY * vh * 0.5;
    const double stripH = m_live.stripH();
    const double stripBottom = (vh - kFootZone) + offsetY;
    const double stripCy = stripBottom - stripH * 0.5;
    const double topClear = std::max(kTopClear, ctx.barReserve.height());
    const double cy = (topClear + 20.0 + ((vh - kFootZone) - stripH)) * 0.5 + offsetY;
    const double chw = m_live.centerHw();
    const double chh = m_live.centerHh();
    const double entrance = std::clamp(ctx.entrance, 0.0, 1.0);
    const double heroVis = std::clamp(double(m_heroFade.x), 0.0, 1.0);

    const bool blending = !m_bmix.settled() || !m_swirl.settled() || m_swirl.x > 0.001;
    double prog = std::clamp(double(m_prog.x), 0.0, 1.0);
    if (blending)
        prog = std::min(prog, 0.995);

    const double shadowT = window(prog, 0.86, 0.96);
    const double ringT = window(m_swirl.x, 0.08, 0.30);
    const double shadowOp = shadowT * entrance * heroVis * (1.0 - ringT);
    if (shadowOp > 0.01) {
        CardVisual sv;
        sv.row = -1;
        sv.texture = CardVisual::TextureNone;
        sv.inst.rect[0] = float(cx);
        sv.inst.rect[1] = float(cy + 12.0);
        sv.inst.rect[2] = float(chw + 44.0);
        sv.inst.rect[3] = float(chh + 44.0);
        for (float &r : sv.inst.radii)
            r = 18.0f;
        sv.inst.fill[0] = sv.inst.fill[1] = sv.inst.fill[2] = 0.0f;
        sv.inst.fill[3] = float(0.55 * shadowOp);
        sv.inst.params[1] = 48.0f;
        sv.inst.params[2] = 1.0f;
        sv.inst.misc[3] = CardFlag::Shadow;
        out.push_back(sv);
    }

    const double stride = m_live.stride();
    const std::vector<Place> places = stripPlace(ctx.count, m_camera.x, stride, cx, vw);
    m_placed.reserve(places.size());
    for (const Place &pl : places) {
        const bool isHover = pl.i == ctx.hovered;
        const bool isCurrent = pl.i == current;
        const double pop = isCurrent ? std::sin(kPi * std::clamp(double(m_pop.x), 0.0, 1.0)) : 0.0;
        const double scale =
            pl.s * (isHover ? 1.12 : isCurrent ? 1.06 + 0.11 * pop : 1.0);
        const double shw = m_live.sliceW * 0.5 * scale;
        const double shh = m_live.sliceH * 0.5 * scale;

        CardVisual cv;
        cv.row = pl.i;
        cv.texture = CardVisual::TextureAuto;
        cv.wantNear = isCurrent || isHover;
        cv.cover = true;
        cv.cropAspect = 0.0f;
        cv.inst.rect[0] = float(pl.x);
        cv.inst.rect[1] = float(stripCy);
        cv.inst.rect[2] = float(shw);
        cv.inst.rect[3] = float(shh);
        cv.inst.radii[0] = float(m_live.corners[0]);
        cv.inst.radii[1] = float(m_live.corners[1]);
        cv.inst.radii[2] = float(m_live.corners[2]);
        cv.inst.radii[3] = float(m_live.corners[3]);
        setFill(cv.inst, ctx.palette.surfaceVariant, 0.8f);
        cv.inst.params[0] = float(m_live.skew);
        cv.inst.params[2] = float(pl.s * entrance);
        if (isHover || isCurrent) {
            const double alpha = isHover ? 1.0 : 0.7 + 0.3 * pop;
            const QColor prim = ctx.palette.primary;
            cv.inst.border[0] = float(prim.redF());
            cv.inst.border[1] = float(prim.greenF());
            cv.inst.border[2] = float(prim.blueF());
            cv.inst.border[3] = float(alpha);
            cv.inst.params[1] = float(isHover ? 2.0 : 1.6 + 0.9 * pop);
        } else {
            cv.inst.border[0] = cv.inst.border[1] = cv.inst.border[2] = 0.0f;
            cv.inst.border[3] = 0.5f;
            cv.inst.params[1] = 1.1f;
        }
        out.push_back(cv);
        m_placed.push_back(Placed{pl.i, pl.x, stripCy, shw, shh, m_live.skew});
    }

    const double solidOp = prog < 0.999 ? 0.0 : 1.0;
    if (solidOp > 0.01) {
        CardVisual hv;
        hv.row = current;
        hv.texture = CardVisual::TextureNear;
        hv.wantNear = true;
        hv.cover = true;
        hv.cropAspect = 0.0f;
        hv.inst.rect[0] = float(cx);
        hv.inst.rect[1] = float(cy);
        hv.inst.rect[2] = float(chw);
        hv.inst.rect[3] = float(chh);
        setFill(hv.inst, ctx.palette.surfaceVariant, 0.8f);
        hv.inst.params[2] = float(solidOp * entrance * heroVis);
        if (ctx.flip.row == current && ctx.flip.progress > 0.0) {
            hv.inst.flip[0] = float(ctx.flip.progress);
            hv.inst.flip[1] = ctx.flip.seed;
            hv.inst.flip[2] = float(ctx.flip.effect);
        }
        out.push_back(hv);
        m_heroRect = QRectF(cx - chw, cy - chh, 2.0 * chw, 2.0 * chh);
        m_heroRow = current;
    } else {
        buildPass(ctx, current, prog, cx, cy, chw, chh);
    }
}

int SandyLayout::hitTest(QPointF point) const
{
    if (m_heroRow >= 0 && m_heroRect.contains(point))
        return m_heroRow;
    int found = -1;
    for (const Placed &p : m_placed) {
        if (shearedContains(p.cx, p.cy, p.hw, p.hh, p.skew, point.x(), point.y())) {
            if (p.row == m_current)
                return p.row;
            found = p.row;
        }
    }
    return found;
}

QRectF SandyLayout::cardRect(int row) const
{
    if (row == m_heroRow && !m_heroRect.isEmpty())
        return m_heroRect;
    for (const Placed &p : m_placed) {
        if (p.row == row)
            return QRectF(p.cx - p.hw, p.cy - p.hh, 2.0 * p.hw, 2.0 * p.hh);
    }
    return QRectF();
}

QRectF SandyLayout::stageRect(const LayoutContext &ctx) const
{
    const double stripH = m_live.stripH();
    return QRectF(0, ctx.viewport.height() - kFootZone - stripH, ctx.viewport.width(), stripH);
}

int SandyLayout::step(const LayoutContext &ctx, int dx, int dy) const
{
    Q_UNUSED(dy)
    if (dx == 0 || ctx.count == 0)
        return ctx.current;
    return std::clamp(ctx.current + dx, 0, ctx.count - 1);
}

int SandyLayout::wheel(const LayoutContext &ctx, QPointF angle, QPointF pixels)
{
    Q_UNUSED(ctx)
    double amount = 0.0;
    if (angle.y() != 0.0 || angle.x() != 0.0)
        amount = (angle.y() != 0.0 ? angle.y() : angle.x()) / 120.0;
    else if (pixels.y() != 0.0 || pixels.x() != 0.0)
        amount = (pixels.y() != 0.0 ? pixels.y() : pixels.x()) / 40.0;
    if (amount == 0.0)
        return 0;
    return scrollSteps(m_wheelAcc, -amount);
}

bool SandyLayout::pointer(const LayoutContext &ctx, QPointF pos, bool inside)
{
    const double vw = ctx.viewport.width();
    const double vh = ctx.viewport.height();
    if (!inside) {
        bool changed = m_edgePan != 0.0 || m_camFree;
        m_edgePan = 0.0;
        m_camFree = false;
        return changed;
    }
    const double localY = pos.y() - m_live.offsetY * vh * 0.5;
    const double pan = edgePan(pos.x(), localY, vw, vh, m_live.stripH());
    bool started = false;
    if (pan != m_edgePan) {
        m_edgePan = pan;
        started = true;
    }
    if (pan != 0.0) {
        m_camFree = true;
        return true;
    }
    const double bottom = vh - kFootZone;
    const bool overStrip = localY >= bottom - m_live.stripH() && localY <= bottom;
    if (!overStrip) {
        if (m_camFree) {
            m_camFree = false;
            started = true;
        }
        return started;
    }
    m_camFree = true;
    return started;
}

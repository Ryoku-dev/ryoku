#include "cardfield.h"

#include "../layouts/geometry.h"
#include "../render/cardrendernode.h"
#include "../render/previewvideo.h"
#include "../render/thumbdecoder.h"
#include "cardsource.h"
#include "frameticker.h"
#include "layoutfactory.h"

#include <QHoverEvent>
#include <QKeyEvent>
#include <QMetaMethod>
#include <QMouseEvent>
#include <QQuickWindow>
#include <QWheelEvent>

#include <algorithm>
#include <cmath>

namespace {

constexpr int kUploadBudget = 8;

void setVec4(float *dst, float a, float b, float c, float d)
{
    dst[0] = a;
    dst[1] = b;
    dst[2] = c;
    dst[3] = d;
}

QColor colorOf(const QVariantMap &map, const char *key, QColor fallback)
{
    const auto it = map.constFind(QLatin1String(key));
    if (it == map.constEnd())
        return fallback;
    const QColor c = it.value().value<QColor>();
    return c.isValid() ? c : fallback;
}

// Quantised so a new card at an old card's spot lines up with it.
qint64 cellKey(float x, float y)
{
    const qint32 cx = qint32(std::lround(x * 0.5f));
    const qint32 cy = qint32(std::lround(y * 0.5f));
    return (qint64(quint32(cx)) << 32) | qint64(quint32(cy));
}

void coverCrop(QSizeF image, float boxAspect, float *crop)
{
    setVec4(crop, 0.0f, 0.0f, 1.0f, 1.0f);
    if (image.width() <= 0.0 || image.height() <= 0.0 || boxAspect <= 0.0f)
        return;
    const float imageAspect = float(image.width() / image.height());
    if (imageAspect > boxAspect) {
        crop[2] = boxAspect / imageAspect;
        crop[0] = (1.0f - crop[2]) * 0.5f;
    } else {
        crop[3] = imageAspect / boxAspect;
        crop[1] = (1.0f - crop[3]) * 0.5f;
    }
}

} // namespace

CardField::CardField(QQuickItem *parent)
    : QQuickItem(parent)
    , m_ticker(std::make_unique<FrameTicker>([this] { tick(); }))
    , m_decoder(std::make_unique<ThumbDecoder>())
    , m_preview(std::make_unique<PreviewVideo>())
{
    setFlag(ItemHasContents);
    setFlag(ItemIsFocusScope);
    setAcceptedMouseButtons(Qt::AllButtons);
    setAcceptHoverEvents(true);
    setActiveFocusOnTab(true);
    m_entrance = Spring::forDuration(0, 180);
    m_entrance.epsilon = 0.001;
    m_flip = Spring::forDuration(0, 1500);
    m_flip.epsilon = 0.01;
    m_trans = Spring::forDuration(0, 250);
    m_trans.epsilon = 0.01;
    m_hover = Spring::forDuration(0, 180);
    m_hover.epsilon = 0.01;
    m_press = Spring::forDuration(0, 120);
    m_press.epsilon = 0.01;
    m_layout = makeLayout(m_mode);
    connect(m_decoder.get(), &ThumbDecoder::ready, this, [this] { kick(); }, Qt::QueuedConnection);
    connect(m_preview.get(), &PreviewVideo::frameReady, this, &QQuickItem::update,
            Qt::QueuedConnection);
    m_previewTimer = new QTimer(this);
    m_previewTimer->setSingleShot(true);
    connect(m_previewTimer, &QTimer::timeout, this, &CardField::startPreview);
    // The process starts hidden, so the multimedia backend loads now instead of stalling the first preview.
    QTimer::singleShot(400, this, [this] {
        if (m_previewEnabled)
            m_preview->prewarm();
    });
}

CardField::~CardField() = default;

void CardField::connectSource()
{
    if (!m_source)
        return;
    CardSourceNotifier *n = dynamic_cast<CardSourceNotifier *>(m_sourceObj.data());
    if (!n)
        n = m_source->cardNotifier();
    if (n) {
        connect(n, &CardSourceNotifier::cardsChanged, this, &CardField::onSourceChanged);
        connect(n, &CardSourceNotifier::cardUpdated, this, &CardField::onCardUpdated);
        connect(n, &CardSourceNotifier::cardsAppended, this, &CardField::onCardsAppended);
    } else if (m_sourceObj) {
        connect(m_sourceObj, SIGNAL(cardsChanged()), this, SLOT(onSourceChanged()));
    }
}

void CardField::disconnectSource()
{
    if (m_sourceObj)
        disconnect(m_sourceObj, nullptr, this, nullptr);
    if (m_source) {
        if (CardSourceNotifier *n = m_source->cardNotifier())
            disconnect(n, nullptr, this, nullptr);
    }
}

void CardField::resetGenerationState()
{
    m_filterIn.clear();
    m_filterWave = 0.0f;
    m_filterElapsed = 0.0;
    m_transVisuals.clear();
    m_transBuf.clear();
    m_transActive = false;
    m_trans.snap(1.0);
    m_visuals.clear();
    m_fades.clear();
    m_hoverRow = -1;
    m_pressRow = -1;
    m_hover.snap(0.0);
    m_press.snap(0.0);
    if (m_flippedRow >= 0) {
        m_flippedRow = -1;
        m_flip.snap(0.0);
        emit flippedIndexChanged();
        emit flipChanged();
    }
    stopPreview();
}

void CardField::setSource(QObject *source)
{
    if (source == m_sourceObj)
        return;
    disconnectSource();
    m_sourceObj = source;
    m_source = dynamic_cast<CardSource *>(source);
    connectSource();
    m_generation = m_source ? m_source->cardGeneration() : 0;
    resetGenerationState();
    if (m_decoder)
        m_decoder->cancelQueued();
    m_currentKey.clear();
    restoreSelection();
    if (m_layout && m_source) {
        LayoutContext ctx = makeContext();
        m_layout->configure(ctx, false);
        m_layout->reset(ctx);
    }
    emit sourceChanged();
    schedulePreview();
    kick();
}

void CardField::restoreSelection()
{
    if (!m_source) {
        m_current = 0;
        return;
    }
    const int count = m_source->cardCount();
    int row = m_current;
    if (!m_currentKey.isEmpty()) {
        const int k = m_source->rowOfKey(m_currentKey);
        if (k >= 0)
            row = k;
    }
    m_current = count > 0 ? std::clamp(row, 0, count - 1) : 0;
    m_currentKey = count > 0 ? m_source->cardKey(m_current) : QString();
}

void CardField::onSourceChanged()
{
    if (!m_source)
        return;
    const quint64 gen = m_source->cardGeneration();
    if (gen == m_generation) {
        update();
        return;
    }
    const int before = m_current;
    resetGenerationState();
    if (m_decoder)
        m_decoder->cancelQueued();
    m_generation = gen;
    restoreSelection();
    if (m_layout) {
        LayoutContext ctx = makeContext();
        m_layout->reset(ctx);
    }
    if (m_current != before)
        emit currentIndexChanged();
    schedulePreview();
    kick();
}

void CardField::onCardsAppended()
{
    if (!m_source)
        return;
    m_generation = m_source->cardGeneration();
    kick();
}

void CardField::onCardUpdated(int row)
{
    Q_UNUSED(row)
    update();
}

void CardField::setSettings(QObject *settings)
{
    if (settings == m_settingsObj)
        return;
    if (m_settingsObj)
        disconnect(m_settingsObj, nullptr, this, nullptr);
    m_settingsObj = settings;
    m_params.setSettings(settings);
    if (settings) {
        const QMetaObject *mo = settings->metaObject();
        const int sig = mo->indexOfSignal("changed(QString,QVariant)");
        const int slot = metaObject()->indexOfSlot("onSettingChanged()");
        if (sig >= 0 && slot >= 0)
            connect(settings, mo->method(sig), this, metaObject()->method(slot));
    }
    refreshSettings(false);
    emit settingsChanged();
    kick();
}

void CardField::refreshSettings(bool animate)
{
    m_motion = readMotionProfile(m_params);
    m_flipOpts = readFlipOptions(m_params);
    m_filterMs = filterSwapMotionMs(m_params, m_motion);
    m_previewEnabled = m_params.flag(QStringLiteral("videoPreview.enabled"), true);
    m_previewDelayMs = m_params.num(QStringLiteral("videoPreview.delayMs"), 250.0);
    if (m_previewTimer)
        schedulePreview();
    m_flip.setDuration(std::max(1.0, m_flipOpts.durationMs * (m_motion.reduced ? 1.0 : m_motion.scale)));
    if (m_layout && m_source) {
        LayoutContext ctx = makeContext();
        m_layout->configure(ctx, animate);
    }
}

void CardField::onSettingChanged()
{
    refreshSettings(true);
    kick();
}

void CardField::setMode(const QString &mode)
{
    if (mode == m_mode)
        return;
    m_mode = mode;
    if (m_layout && !m_visuals.empty()) {
        m_transVisuals.assign(m_visuals.begin(), m_visuals.end());
        m_trans.snap(0);
        m_trans.target = 1;
        m_trans.setDuration(m_motion.ms(MotionProfile::Standard));
        m_transActive = true;
    }
    m_layout = makeLayout(mode);
    if (m_source) {
        LayoutContext ctx = makeContext();
        m_layout->configure(ctx, false);
        m_layout->reset(ctx);
        restoreSelection();
    }
    emit modeChanged();
    emit flipsInPlaceChanged();
    kick();
}

void CardField::setCurrentIndex(int index)
{
    const int count = m_source ? m_source->cardCount() : 0;
    if (count == 0) {
        // Keep the request so the first data load restores it.
        if (index != m_current) {
            m_current = std::max(0, index);
            emit currentIndexChanged();
        }
        return;
    }
    index = std::clamp(index, 0, count - 1);
    if (index == m_current)
        return;
    const int old = m_current;
    m_current = index;
    m_currentKey = m_source->cardKey(m_current);
    if (m_flippedRow >= 0) {
        m_flippedRow = -1;
        m_flip.snap(0);
        emit flippedIndexChanged();
        emit flipChanged();
    }
    if (m_layout) {
        LayoutContext ctx = makeContext();
        m_layout->select(ctx, old, m_current);
    }
    emit currentIndexChanged();
    schedulePreview();
    kick();
}

void CardField::setActive(bool active)
{
    if (active == m_active)
        return;
    m_active = active;
    if (active) {
        if (m_entrance.x <= 0.001)
            m_entrance.snap(openFadeFrom(m_params));
        m_entrance.setDuration(launchMotionMs(m_params, m_motion));
        m_entrance.target = 1;
        schedulePreview();
    } else {
        m_entrance.target = 0;
        stopPreview();
    }
    emit activeChanged();
    kick();
}

void CardField::setPalette(const QVariantMap &palette)
{
    m_paletteMap = palette;
    m_palette.primary = colorOf(palette, "primary", m_palette.primary);
    m_palette.accent = colorOf(palette, "accent", m_palette.accent);
    m_palette.surface = colorOf(palette, "surface", m_palette.surface);
    m_palette.surfaceVariant = colorOf(palette, "surfaceVariant", m_palette.surfaceVariant);
    m_palette.outline = colorOf(palette, "outline", m_palette.outline);
    m_palette.text = colorOf(palette, "text", m_palette.text);
    m_palette.shadow = colorOf(palette, "shadow", m_palette.shadow);
    emit paletteChanged();
    update();
}

void CardField::setInteractive(bool interactive)
{
    if (interactive == m_interactive)
        return;
    m_interactive = interactive;
    emit interactiveChanged();
}

void CardField::setColumns(int columns)
{
    if (columns == m_columns)
        return;
    m_columns = columns;
    emit columnsChanged();
    kick();
}

void CardField::setActivateOnClick(bool activate)
{
    if (activate == m_activateOnClick)
        return;
    m_activateOnClick = activate;
    emit activateOnClickChanged();
}

void CardField::setBarReserve(const QSizeF &reserve)
{
    if (reserve == m_barReserve)
        return;
    m_barReserve = reserve;
    emit barReserveChanged();
    kick();
}

void CardField::step(int dx, int dy)
{
    if (!m_layout)
        return;
    LayoutContext ctx = makeContext();
    setCurrentIndex(m_layout->step(ctx, dx, dy));
}

void CardField::page(int dir)
{
    if (!m_layout)
        return;
    LayoutContext ctx = makeContext();
    setCurrentIndex(m_current + m_layout->page(ctx, dir));
}

void CardField::home()
{
    setCurrentIndex(0);
}

void CardField::end()
{
    const int count = m_source ? m_source->cardCount() : 0;
    setCurrentIndex(count > 0 ? count - 1 : 0);
}

void CardField::select(int row)
{
    setCurrentIndex(row);
}

void CardField::setScrollPosition(qreal position)
{
    if (!m_layout)
        return;
    LayoutContext ctx = makeContext();
    m_layout->setScrollPosition(ctx, position);
    kick();
}

void CardField::scrollBy(qreal delta)
{
    setScrollPosition(m_layout ? m_layout->scrollPosition() + delta : delta);
}

void CardField::flip(int row)
{
    if (!m_layout)
        return;
    if (m_flippedRow == row && m_flip.target > 0.5) {
        unflip();
        return;
    }
    if (m_flippedRow != row) {
        m_flippedRow = row;
        m_flip.snap(0);
    }
    m_flip.setDuration(std::max(1.0, m_flipOpts.durationMs * (m_motion.reduced ? 1.0 : m_motion.scale)));
    if (!m_flipOpts.shader && !m_flipOpts.back)
        m_flip.snap(1);
    else
        m_flip.target = 1;
    emit flippedIndexChanged();
    emit flipChanged();
    kick();
}

void CardField::unflip()
{
    if (m_flippedRow < 0)
        return;
    if (m_layout && m_layout->flipsInPlace() && !m_flipOpts.shader && !m_flipOpts.back) {
        m_flip.snap(0);
        m_flippedRow = -1;
        emit flippedIndexChanged();
    } else {
        m_flip.target = 0;
    }
    emit flipChanged();
    kick();
}

QRectF CardField::rectOf(int row) const
{
    return m_layout ? m_layout->cardRect(row) : QRectF();
}

bool CardField::action(const QString &name)
{
    if (!m_layout)
        return false;
    LayoutContext ctx = makeContext();
    const bool handled = m_layout->action(ctx, name);
    kick();
    return handled;
}

LayoutContext CardField::makeContext()
{
    m_params.setViewportWidth(width());
    LayoutContext ctx;
    ctx.viewport = QSizeF(width(), height());
    ctx.count = m_source ? m_source->cardCount() : 0;
    ctx.current = ctx.count > 0 ? std::clamp(m_current, 0, ctx.count - 1) : 0;
    ctx.hovered = m_hovered;
    ctx.pointer = m_pointer;
    ctx.pointerInside = m_pointerInside;
    // Layouts lay cards out at full presence; the staggered open bloom is folded in per
    // card by applyMicroAnim so it can spring each card from the focus outward.
    ctx.entrance = 1.0;
    ctx.time = m_time;
    ctx.motion = &m_motion;
    ctx.params = &m_params;
    ctx.source = m_source;
    ctx.palette = m_palette;
    ctx.flip.row = m_flippedRow;
    ctx.flip.progress = std::clamp(m_flip.x, 0.0, 1.0);
    ctx.flip.effect = m_flipOpts.effect;
    ctx.flip.seed = std::fmod(float(m_flippedRow < 0 ? 0 : m_flippedRow), 7.0f) * 1.3f + 0.5f;
    ctx.columnsOverride = m_columns;
    ctx.barReserve = m_barReserve;
    return ctx;
}

void CardField::keyPressEvent(QKeyEvent *event)
{
    // Modified arrows are keymap chords (shift+down search, alt+left sort, ...), not navigation.
    const Qt::KeyboardModifiers mods = event->modifiers() & ~Qt::KeypadModifier;
    if (!m_interactive || !m_layout || mods != Qt::NoModifier) {
        QQuickItem::keyPressEvent(event);
        return;
    }
    const int count = m_source ? m_source->cardCount() : 0;
    LayoutContext ctx = makeContext();
    switch (event->key()) {
    case Qt::Key_Left:
        setCurrentIndex(m_layout->step(ctx, -1, 0));
        break;
    case Qt::Key_Right:
        setCurrentIndex(m_layout->step(ctx, 1, 0));
        break;
    case Qt::Key_Up:
        setCurrentIndex(m_layout->step(ctx, 0, -1));
        break;
    case Qt::Key_Down:
        setCurrentIndex(m_layout->step(ctx, 0, 1));
        break;
    case Qt::Key_PageUp:
        setCurrentIndex(m_current + m_layout->page(ctx, -1));
        break;
    case Qt::Key_PageDown:
        setCurrentIndex(m_current + m_layout->page(ctx, 1));
        break;
    case Qt::Key_Home:
        setCurrentIndex(0);
        break;
    case Qt::Key_End:
        setCurrentIndex(count > 0 ? count - 1 : 0);
        break;
    case Qt::Key_Return:
    case Qt::Key_Enter:
        emit activated(m_current);
        break;
    default:
        QQuickItem::keyPressEvent(event);
        return;
    }
    event->accept();
}

void CardField::mousePressEvent(QMouseEvent *event)
{
    if (!m_interactive || !m_layout) {
        event->ignore();
        return;
    }
    forceActiveFocus();
    const QPointF pos = event->position();
    m_pointer = pos;
    const int row = m_layout->hitTest(pos);
    if (event->button() == Qt::RightButton) {
        emit contextRequested(row, pos);
        event->accept();
        return;
    }
    if (event->button() == Qt::LeftButton) {
        m_dragging = true;
        m_dragLast = pos;
        if (row >= 0) {
            // A short inward tap the release springs back out of.
            m_pressRow = row;
            m_press.snap(1.0);
            m_press.target = 0.0;
            kick();
        }
        if (row < 0) {
            emit backgroundClicked();
            event->accept();
            return;
        }
        if (m_activateOnClick) {
            setCurrentIndex(row);
            emit activated(row);
            event->accept();
            return;
        }
        LayoutContext ctx = makeContext();
        switch (m_layout->click(ctx, row)) {
        case Layout::Click::Select:
            setCurrentIndex(row);
            break;
        case Layout::Click::Apply:
            emit activated(row);
            break;
        case Layout::Click::Ignore:
            break;
        }
        event->accept();
        return;
    }
    event->ignore();
}

void CardField::mouseMoveEvent(QMouseEvent *event)
{
    if (!m_interactive || !m_layout || !m_dragging) {
        event->ignore();
        return;
    }
    const QPointF pos = event->position();
    const QPointF delta = pos - m_dragLast;
    m_dragLast = pos;
    m_pointer = pos;
    LayoutContext ctx = makeContext();
    if (m_layout->drag(ctx, delta, false))
        kick();
    event->accept();
}

void CardField::mouseReleaseEvent(QMouseEvent *event)
{
    if (m_dragging && m_layout) {
        LayoutContext ctx = makeContext();
        if (m_layout->drag(ctx, QPointF(), true))
            kick();
    }
    m_dragging = false;
    event->accept();
}

void CardField::hoverMoveEvent(QHoverEvent *event)
{
    if (!m_interactive || !m_layout)
        return;
    const QPointF pos = event->position();
    m_pointer = pos;
    m_pointerInside = true;
    // The masthead sits over the top band; a card found under it would make the hover key
    // (and its live palette preview) churn as the pointer travels the bar, flashing the desktop.
    const bool overBar = m_barReserve.height() > 0.0 && pos.y() < m_barReserve.height();
    const int row = overBar ? -1 : m_layout->hitTest(pos);
    if (row != m_hovered) {
        m_hovered = row;
        if (row >= 0) {
            m_hoverRow = row;
            m_hover.target = 1.0;
        } else {
            m_hover.target = 0.0;
        }
        emit hoveredIndexChanged();
        kick();
    }
    LayoutContext ctx = makeContext();
    if (m_layout->pointer(ctx, pos, true))
        kick();
    else
        update();
}

void CardField::hoverLeaveEvent(QHoverEvent *event)
{
    Q_UNUSED(event)
    m_pointerInside = false;
    if (m_hovered != -1) {
        m_hovered = -1;
        emit hoveredIndexChanged();
    }
    m_hover.target = 0.0;
    if (m_layout) {
        LayoutContext ctx = makeContext();
        m_layout->pointer(ctx, m_pointer, false);
    }
    kick();
}

void CardField::wheelEvent(QWheelEvent *event)
{
    if (!m_interactive || !m_layout) {
        event->ignore();
        return;
    }
    LayoutContext ctx = makeContext();
    const int delta = m_layout->wheel(ctx, QPointF(event->angleDelta()), QPointF(event->pixelDelta()));
    if (delta != 0)
        setCurrentIndex(m_current + delta);
    kick();
    event->accept();
}

void CardField::kick()
{
    if (!m_animating) {
        m_animating = true;
        m_clock.restart();
        m_ticker->start();
        emit movingChanged();
    }
    update();
}

bool CardField::tickFades(double dt)
{
    bool moving = false;
    for (auto it = m_fades.begin(); it != m_fades.end();) {
        it->second.setDuration(m_motion.ms(MotionProfile::Fast));
        const bool m = it->second.tick(dt);
        moving |= m;
        if (!m)
            it = m_fades.erase(it);
        else
            ++it;
    }
    return moving;
}

bool CardField::advanceFilterSwap(double dt)
{
    const float step = float(std::min(dt, 0.05) * 1000.0 / std::max(m_filterMs, 50.0));
    if (!m_filterIn.empty()) {
        m_filterElapsed += std::min(dt, 0.05);
        if (m_filterElapsed * 1000.0 > std::max(m_filterMs * 3.0, 2000.0)) {
            m_filterIn.clear();
            m_filterWave = 10.0f;
            return false;
        }
    }
    if (m_filterWave < 10.0f)
        m_filterWave += step;
    for (auto it = m_filterIn.begin(); it != m_filterIn.end();) {
        it.value() += step;
        if (it.value() >= 1.0f)
            it = m_filterIn.erase(it);
        else
            ++it;
    }
    return !m_filterIn.empty() || m_filterWave < 2.0f;
}

void CardField::tick()
{
    const double dt = std::min(m_clock.restart() / 1000.0, 0.05);
    m_time += dt;
    bool moving = false;

    moving |= m_entrance.tick(dt);

    m_hover.setDuration(m_motion.ms(MotionProfile::Fast));
    m_press.setDuration(std::max(1.0, m_motion.ms(MotionProfile::Fast) * 0.7));
    moving |= m_hover.tick(dt);
    moving |= m_press.tick(dt);

    const bool flipMoving = m_flip.tick(dt);
    moving |= flipMoving;
    if (!flipMoving && m_flip.target <= 0.0 && m_flippedRow >= 0 && m_flip.x <= 0.001) {
        m_flippedRow = -1;
        emit flippedIndexChanged();
        emit flipChanged();
    }

    if (m_transActive) {
        if (m_trans.tick(dt))
            moving = true;
        else {
            m_transActive = false;
            m_transVisuals.clear();
        }
    }

    if (advanceFilterSwap(dt))
        moving = true;
    if (tickFades(dt))
        moving = true;

    if (m_layout && m_source && m_source->cardCount() > 0) {
        LayoutContext ctx = makeContext();
        if (m_layout->tick(ctx, dt))
            moving = true;
    }

    update();
    if (!moving) {
        m_ticker->stop();
        m_animating = false;
        emit movingChanged();
    }
}

float CardField::flipPhase(float x, float y) const
{
    const float vw = std::max(float(width()), 1.0f);
    const float vh = std::max(float(height()), 1.0f);
    float phase;
    if (m_mode == QLatin1String("wall") || m_mode == QLatin1String("grid")) {
        phase = (x / vw + y / vh) * 0.5f;
    } else if (m_mode == QLatin1String("hex")) {
        const float dx = (x - vw * 0.5f) / std::max(vw * 0.5f, 1.0f);
        const float dy = (y - vh * 0.5f) / std::max(vh * 0.5f, 1.0f);
        phase = std::sqrt(dx * dx + dy * dy) * 0.7f;
    } else {
        phase = x / vw;
    }
    return std::clamp(phase, 0.0f, 1.0f);
}

float CardField::filterRollFor(qint64 cell, float x, float y)
{
    if (m_filterIn.empty() && m_filterWave >= 2.0f)
        return 1.0f;
    const auto it = m_filterIn.constFind(cell);
    if (it != m_filterIn.constEnd())
        return geom::smoothstep(it.value());
    const float virt = m_filterWave - flipPhase(x, y) * geom::kFilterFlipSweep;
    if (virt < 1.0f) {
        m_filterIn.insert(cell, virt);
        return geom::smoothstep(virt);
    }
    return 1.0f;
}

void CardField::drainDecoder(CardRenderNode *node)
{
    for (ThumbDecoder::Result &r : m_decoder->take(kUploadBudget)) {
        const QString key = r.image.key;
        TextureTier &tier = r.tier == ThumbDecoder::Near ? node->nearTier() : node->farTier();
        tier.admit(std::move(r.image));
        if (m_fades.find(key) == m_fades.end()) {
            Spring s = Spring::forDuration(0, m_motion.ms(MotionProfile::Fast));
            s.epsilon = 0.01;
            s.target = 1;
            m_fades.emplace(key, s);
        }
    }
}

float CardField::fadeFor(const QString &key) const
{
    const auto it = m_fades.find(key);
    if (it == m_fades.end())
        return 1.0f;
    return float(std::clamp(it->second.x, 0.0, 1.0));
}

void CardField::applyCrop(const CardVisual &visual, CardInstance &inst, int row) const
{
    if (!visual.cover)
        return;
    const QSizeF img = m_source->cardImageSize(row);
    const float boxAspect =
        visual.cropAspect > 0.0f ? visual.cropAspect
                                 : (inst.rect[3] != 0.0f ? inst.rect[2] / inst.rect[3] : 1.0f);
    float crop[4];
    coverCrop(img, boxAspect, crop);
    if (visual.cropZoom > 1.0f) {
        const float nw = crop[2] / visual.cropZoom;
        const float nh = crop[3] / visual.cropZoom;
        crop[0] += (crop[2] - nw) * (0.5f + std::clamp(visual.cropShiftX, -1.0f, 1.0f) * 0.5f);
        crop[1] += (crop[3] - nh) * (0.5f + std::clamp(visual.cropShiftY, -1.0f, 1.0f) * 0.5f);
        crop[2] = nw;
        crop[3] = nh;
    }
    setVec4(inst.crop, crop[0], crop[1], crop[2], crop[3]);
}

void CardField::applyFlipPayload(CardInstance &inst, int row) const
{
    const float progress = float(std::clamp(m_flip.x, 0.0, 1.0));
    const float shader = m_flipOpts.shader ? std::clamp(progress / 0.82f, 0.0f, 1.0f) : 0.0f;
    if (shader <= 0.001f)
        return;
    inst.flip[0] = shader;
    inst.flip[1] = std::fmod(float(row), 7.0f) * 1.3f + 0.5f;
    inst.flip[2] = float(m_flipOpts.effect);
    inst.flip[3] = 0.0f;
}

void CardField::resolveTexture(CardRenderNode *node, const LayoutContext &ctx,
                               const CardVisual &visual, CardInstance &inst, int row)
{
    const QString key = m_source->cardKey(row);
    m_wanted.insert(key);
    const bool wantNear = visual.wantNear || std::abs(row - ctx.current) <= 2;
    // The preview texture still holds the last clip's frame until this clip delivers one.
    const bool preview = (row == ctx.current) && m_previewActive && m_previewHasFrame && node->hasPreview();
    uint32_t src = CardTex::None;

    if (preview) {
        inst.misc[0] = CardTex::Preview;
        inst.misc[1] = 0;
        const QRectF uv = node->previewUv();
        setVec4(inst.uv, float(uv.x()), float(uv.y()), float(uv.width()), float(uv.height()));
    } else {
        const TextureTier::Slot *slot = nullptr;
        if (wantNear && (slot = node->nearTier().find(key)))
            src = CardTex::Near;
        else if ((slot = node->farTier().find(key)))
            src = CardTex::Far;
        const int dist = std::abs(row - ctx.current);
        if (wantNear && src != CardTex::Near)
            m_decoder->request(key, m_source->cardFullImage(row), m_source->cardThumb(row),
                               ThumbDecoder::Near, 100 - dist);
        if (src == CardTex::None)
            m_decoder->request(key, m_source->cardThumb(row), ThumbDecoder::Far, 50 - dist);
        inst.misc[0] = src;
        if (slot) {
            inst.misc[1] = uint32_t(slot->layer);
            setVec4(inst.uv, float(slot->uv.x()), float(slot->uv.y()), float(slot->uv.width()),
                    float(slot->uv.height()));
        }
    }

    // Only when the layout left fill unset; projected backfaces set their own.
    if (inst.fill[3] == 0.0f) {
        const QColor fill = m_source->cardFill(row);
        if (fill.isValid())
            setVec4(inst.fill, float(fill.redF()), float(fill.greenF()), float(fill.blueF()), 1.0f);
    }
    inst.params[3] = inst.misc[0] != CardTex::None ? fadeFor(key) : 1.0f;
    applyCrop(visual, inst, row);
}

void CardField::applyMicroAnim(CardInstance &inst, int row, bool projected, const LayoutContext &ctx) const
{
    // Open bloom: the focused card leads, the rest ripple out from it and spring to rest.
    const float e = float(std::clamp(m_entrance.x, 0.0, 1.0));
    if (e < 0.999f) {
        constexpr float span = 0.55f;
        const float phase = std::min(1.0f, float(std::abs(row - ctx.current)) / 12.0f);
        const float local = std::clamp(e * (1.0f + span) - phase * span, 0.0f, 1.0f);
        inst.params[2] *= geom::easeOutCubic(local);
        if (!projected) {
            const float back = local - 1.0f;
            const float overshoot = 1.0f + 2.70158f * back * back * back + 1.70158f * back * back;
            const float scale = 0.92f + 0.08f * overshoot;
            inst.rect[2] *= scale;
            inst.rect[3] *= scale;
        }
    }

    // Hover: brighten (lift the dimming tint) and, on flat cards, a small scale and rise.
    if (row >= 0 && row == m_hoverRow && m_hover.x > 0.001) {
        const float h = float(std::clamp(m_hover.x, 0.0, 1.0));
        inst.tint[3] *= 1.0f - 0.55f * h;
        if (!projected) {
            const float lift = 1.0f + 0.03f * h;
            inst.rect[2] *= lift;
            inst.rect[3] *= lift;
            inst.rect[1] -= 5.0f * h;
        }
    }

    // Press: a quick inward tap the release lets spring back.
    if (row >= 0 && row == m_pressRow && !projected && m_press.x > 0.001) {
        const float p = float(std::clamp(m_press.x, 0.0, 1.0));
        const float shrink = 1.0f - 0.035f * p;
        inst.rect[2] *= shrink;
        inst.rect[3] *= shrink;
    }
}

void CardField::resolve(CardRenderNode *node, const LayoutContext &ctx,
                        std::vector<CardInstance> &instances)
{
    int pendingShadow = -1;
    int pendingShadowRow = -1;
    for (const CardVisual &v : m_visuals) {
        CardInstance inst = v.inst;
        const int row = v.row;
        const bool isCard = v.texture != CardVisual::TextureNone && row >= 0 && row < ctx.count;
        if (isCard)
            resolveTexture(node, ctx, v, inst, row);
        const bool projected = (inst.misc[3] & CardFlag::Projected) != 0;
        if (isCard && !projected) {
            const qint64 cell = cellKey(inst.rect[0], inst.rect[1]);
            const float roll = filterRollFor(cell, inst.rect[0], inst.rect[1]);
            geom::rollInCut(inst, roll);
            if (row == m_flippedRow && m_layout->flipsInPlace())
                applyFlipPayload(inst, row);
            if (pendingShadow >= 0 && pendingShadowRow == row) {
                instances[size_t(pendingShadow)].params[2] *= roll;
                pendingShadow = -1;
            }
        } else if ((inst.misc[3] & CardFlag::Shadow) && !projected) {
            pendingShadow = int(instances.size());
            pendingShadowRow = row;
        }
        applyMicroAnim(inst, row, projected, ctx);
        instances.push_back(inst);
    }
}

void CardField::resolveTransition(CardRenderNode *node, const LayoutContext &ctx)
{
    if (!m_transActive) {
        node->setTransition({}, 1.0f, 0);
        return;
    }
    m_transBuf.clear();
    m_transBuf.reserve(m_transVisuals.size());
    for (const CardVisual &v : m_transVisuals) {
        CardInstance inst = v.inst;
        const int row = v.row;
        if (v.texture != CardVisual::TextureNone && row >= 0 && row < ctx.count)
            resolveTexture(node, ctx, v, inst, row);
        m_transBuf.push_back(inst);
    }
    node->setTransition(std::vector<CardInstance>(m_transBuf), float(std::clamp(m_trans.x, 0.0, 1.0)),
                        0);
}

void CardField::stopPreview()
{
    if (m_previewActive)
        m_preview->stop();
    m_previewActive = false;
    m_previewHasFrame = false;
    m_previewRow = -1;
}

void CardField::schedulePreview()
{
    const int count = m_source ? m_source->cardCount() : 0;
    const QString clip = (m_previewEnabled && m_active && count > 0)
        ? m_source->cardPreviewVideo(std::clamp(m_current, 0, count - 1)) : QString();
    if (clip.isEmpty()) {
        m_previewTimer->stop();
        stopPreview();
        return;
    }
    if (m_previewRow == m_current && (m_previewActive || m_previewTimer->isActive()))
        return;
    stopPreview();
    m_previewRow = m_current;
    m_previewTimer->start(int(std::max(0.0, m_previewDelayMs)));
}

void CardField::startPreview()
{
    const int count = m_source ? m_source->cardCount() : 0;
    if (!m_previewEnabled || !m_active || count == 0 || m_previewRow != m_current)
        return;
    const QString clip = m_source->cardPreviewVideo(std::clamp(m_current, 0, count - 1));
    if (clip.isEmpty())
        return;
    // Hold the decoder off until the scene settles: a clip that starts while the camera is
    // still gliding stalls navigation, and the card is not yet the one being looked at.
    if (m_animating) {
        m_previewTimer->start(int(std::max(120.0, m_previewDelayMs)));
        return;
    }
    m_preview->play(clip, true, 0.0);
    m_previewActive = true;
}

void CardField::publishRects(const QRectF &current, QPointF shear, const QRectF &stage)
{
    // Signals leave sync through the event loop so bound QML never re-enters polish or sync.
    m_pendingRect = current;
    m_pendingShear = shear;
    m_pendingStage = stage;
    if (m_rectQueued)
        return;
    m_rectQueued = true;
    QMetaObject::invokeMethod(this, [this] {
        m_rectQueued = false;
        if (m_pendingRect != m_currentRect || m_pendingShear != m_currentShear) {
            m_currentRect = m_pendingRect;
            m_currentShear = m_pendingShear;
            emit currentRectChanged();
        }
        if (m_pendingStage != m_stageRect) {
            m_stageRect = m_pendingStage;
            emit stageRectChanged();
        }
    }, Qt::QueuedConnection);
}

void CardField::publishVisibleEnd(int end)
{
    m_pendingVisibleEnd = end;
    if (m_endQueued)
        return;
    m_endQueued = true;
    QMetaObject::invokeMethod(this, [this] {
        m_endQueued = false;
        if (m_pendingVisibleEnd != m_visibleEnd) {
            m_visibleEnd = m_pendingVisibleEnd;
            emit visibleEndChanged();
        }
    }, Qt::QueuedConnection);
}

void CardField::publishScrollMetrics(qreal position, qreal extent)
{
    m_pendingScrollPosition = position;
    m_pendingScrollExtent = extent;
    if (m_scrollQueued)
        return;
    m_scrollQueued = true;
    QMetaObject::invokeMethod(this, [this] {
        m_scrollQueued = false;
        if (!qFuzzyCompare(m_scrollPosition + 1.0, m_pendingScrollPosition + 1.0)
                || !qFuzzyCompare(m_scrollExtent + 1.0, m_pendingScrollExtent + 1.0)) {
            m_scrollPosition = m_pendingScrollPosition;
            m_scrollExtent = m_pendingScrollExtent;
            emit scrollMetricsChanged();
        }
    }, Qt::QueuedConnection);
}

QSGNode *CardField::updatePaintNode(QSGNode *old, UpdatePaintNodeData *)
{
    auto *node = static_cast<CardRenderNode *>(old);
    if (!node) {
        node = new CardRenderNode(window());
        m_nearTile = node->nearTileSize();
        m_decoder->setTileSizes(m_nearTile, QSize(256, 160));
    }
    node->beginFrame();
    drainDecoder(node);

    m_wanted.clear();
    if (m_layout && m_source && m_source->cardCount() > 0) {
        LayoutContext ctx = makeContext();
        // The frame goes to the node first: resolve() reads back the part of
        // the preview texture it covers.
        if (m_previewActive && m_preview->active()) {
            const QImage previewImage = m_preview->takeFrame();
            if (!previewImage.isNull()) {
                m_previewHasFrame = true;
                node->setPreviewImage(previewImage);
            }
        }

        m_visuals.clear();
        m_layout->build(ctx, m_visuals);

        std::vector<CardInstance> instances;
        instances.reserve(m_visuals.size());
        resolve(node, ctx, instances);

        publishRects(m_layout->cardRect(ctx.current), m_layout->cardShear(ctx.current),
                     m_layout->stageRect(ctx));
        int visEnd = -1;
        for (const CardVisual &v : m_visuals)
            if (v.texture != CardVisual::TextureNone && v.row > visEnd)
                visEnd = v.row;
        publishVisibleEnd(visEnd);
        publishScrollMetrics(m_layout->scrollPosition(), m_layout->scrollExtent());

        node->setInstances(std::move(instances));
        resolveTransition(node, ctx);
        node->setSandyPass(m_layout->sandyPass());
        const QRectF clip = m_layout->clip(ctx);
        node->setScene(boundingRect(), clip.isValid() ? clip : boundingRect(), float(m_time), 1.0f);
    } else {
        node->setInstances({});
        node->setTransition({}, 1.0f, 0);
        node->setScene(boundingRect(), boundingRect(), float(m_time), 1.0f);
        publishVisibleEnd(-1);
        publishScrollMetrics(0, 0);
        if (m_layout)
            publishRects(m_pendingRect, m_pendingShear, m_layout->stageRect(makeContext()));
    }

    if (m_decoder)
        m_decoder->retain(m_wanted);
    if (m_decoder->hasResults())
        update();
    node->markDirty(QSGNode::DirtyMaterial);
    return node;
}

void CardField::geometryChange(const QRectF &newGeometry, const QRectF &oldGeometry)
{
    QQuickItem::geometryChange(newGeometry, oldGeometry);
    m_params.setViewportWidth(width());
    if (m_layout && m_source) {
        LayoutContext ctx = makeContext();
        m_layout->configure(ctx, true);
    }
    update();
}

void CardField::releaseResources()
{
    QQuickItem::releaseResources();
}

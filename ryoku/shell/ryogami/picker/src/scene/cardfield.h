#pragma once

#include "layout.h"
#include "paramsadapter.h"
#include "spring.h"

#include <QColor>
#include <QElapsedTimer>
#include <QHash>
#include <QImage>
#include <QPointF>
#include <QPointer>
#include <QQuickItem>
#include <QTimer>
#include <QRectF>
#include <QSet>
#include <QSize>
#include <QString>
#include <QVariantMap>

#include <memory>
#include <unordered_map>
#include <vector>

class FrameTicker;
class ThumbDecoder;
class PreviewVideo;
class CardRenderNode;
class CardSource;

class CardField : public QQuickItem
{
    Q_OBJECT
    QML_ELEMENT
    Q_PROPERTY(QObject *source READ source WRITE setSource NOTIFY sourceChanged)
    Q_PROPERTY(QObject *settings READ settings WRITE setSettings NOTIFY settingsChanged)
    Q_PROPERTY(QString mode READ mode WRITE setMode NOTIFY modeChanged)
    Q_PROPERTY(int currentIndex READ currentIndex WRITE setCurrentIndex NOTIFY currentIndexChanged)
    Q_PROPERTY(bool active READ active WRITE setActive NOTIFY activeChanged)
    Q_PROPERTY(QVariantMap palette READ palette WRITE setPalette NOTIFY paletteChanged)
    Q_PROPERTY(bool interactive READ interactive WRITE setInteractive NOTIFY interactiveChanged)
    Q_PROPERTY(int columns READ columns WRITE setColumns NOTIFY columnsChanged)
    Q_PROPERTY(bool activateOnClick READ activateOnClick WRITE setActivateOnClick NOTIFY activateOnClickChanged)
    Q_PROPERTY(int hoveredIndex READ hoveredIndex NOTIFY hoveredIndexChanged)
    Q_PROPERTY(QRectF currentRect READ currentRect NOTIFY currentRectChanged)
    Q_PROPERTY(QPointF currentShear READ currentShear NOTIFY currentRectChanged)
    Q_PROPERTY(QRectF stageRect READ stageRect NOTIFY stageRectChanged)
    Q_PROPERTY(QSizeF barReserve READ barReserve WRITE setBarReserve NOTIFY barReserveChanged)
    Q_PROPERTY(int visibleEnd READ visibleEnd NOTIFY visibleEndChanged)
    Q_PROPERTY(qreal scrollPosition READ scrollPosition NOTIFY scrollMetricsChanged)
    Q_PROPERTY(qreal scrollExtent READ scrollExtent NOTIFY scrollMetricsChanged)
    Q_PROPERTY(int flippedIndex READ flippedIndex NOTIFY flippedIndexChanged)
    Q_PROPERTY(bool flipsInPlace READ flipsInPlace NOTIFY flipsInPlaceChanged)
    Q_PROPERTY(bool moving READ moving NOTIFY movingChanged)

public:
    explicit CardField(QQuickItem *parent = nullptr);
    ~CardField() override;

    QObject *source() const { return m_sourceObj; }
    void setSource(QObject *source);
    QObject *settings() const { return m_settingsObj; }
    void setSettings(QObject *settings);
    QString mode() const { return m_mode; }
    void setMode(const QString &mode);
    int currentIndex() const { return m_current; }
    void setCurrentIndex(int index);
    bool active() const { return m_active; }
    void setActive(bool active);
    QVariantMap palette() const { return m_paletteMap; }
    void setPalette(const QVariantMap &palette);
    bool interactive() const { return m_interactive; }
    void setInteractive(bool interactive);
    bool activateOnClick() const { return m_activateOnClick; }
    void setActivateOnClick(bool activate);
    int columns() const { return m_columns; }
    void setColumns(int columns);
    int hoveredIndex() const { return m_hovered; }
    QRectF currentRect() const { return m_currentRect; }
    QPointF currentShear() const { return m_currentShear; }
    QRectF stageRect() const { return m_stageRect; }
    QSizeF barReserve() const { return m_barReserve; }
    void setBarReserve(const QSizeF &reserve);
    int flippedIndex() const { return m_flippedRow; }
    int visibleEnd() const { return m_visibleEnd; }
    qreal scrollPosition() const { return m_scrollPosition; }
    qreal scrollExtent() const { return m_scrollExtent; }
    bool flipsInPlace() const { return m_layout && m_layout->flipsInPlace(); }
    bool moving() const { return m_animating; }

    Q_INVOKABLE void step(int dx, int dy);
    Q_INVOKABLE void page(int dir);
    Q_INVOKABLE void home();
    Q_INVOKABLE void end();
    Q_INVOKABLE void select(int row);
    Q_INVOKABLE void flip(int row);
    Q_INVOKABLE void unflip();
    Q_INVOKABLE QRectF rectOf(int row) const;
    Q_INVOKABLE bool action(const QString &name);
    Q_INVOKABLE void setScrollPosition(qreal position);
    Q_INVOKABLE void scrollBy(qreal delta);

Q_SIGNALS:
    void sourceChanged();
    void settingsChanged();
    void modeChanged();
    void currentIndexChanged();
    void activeChanged();
    void paletteChanged();
    void interactiveChanged();
    void columnsChanged();
    void hoveredIndexChanged();
    void activateOnClickChanged();
    void currentRectChanged();
    void stageRectChanged();
    void barReserveChanged();
    void flippedIndexChanged();
    void flipsInPlaceChanged();
    void visibleEndChanged();
    void scrollMetricsChanged();
    void movingChanged();

    void activated(int row);
    void contextRequested(int row, QPointF pos);
    void backgroundClicked();
    void flipChanged();

protected:
    QSGNode *updatePaintNode(QSGNode *old, UpdatePaintNodeData *) override;
    void geometryChange(const QRectF &newGeometry, const QRectF &oldGeometry) override;
    void keyPressEvent(QKeyEvent *event) override;
    void mousePressEvent(QMouseEvent *event) override;
    void mouseMoveEvent(QMouseEvent *event) override;
    void mouseReleaseEvent(QMouseEvent *event) override;
    void hoverMoveEvent(QHoverEvent *event) override;
    void hoverLeaveEvent(QHoverEvent *event) override;
    void wheelEvent(QWheelEvent *event) override;
    void releaseResources() override;

private:
    void tick();
    void kick();
    LayoutContext makeContext();

private Q_SLOTS:
    void onSourceChanged();
    void onCardsAppended();
    void onCardUpdated(int row);
    void onSettingChanged();

private:
    void refreshSettings(bool animate);
    void restoreSelection();
    void connectSource();
    void disconnectSource();
    void resetGenerationState();

    void drainDecoder(CardRenderNode *node);
    void resolve(CardRenderNode *node, const LayoutContext &ctx,
                 std::vector<CardInstance> &instances);
    void resolveTexture(CardRenderNode *node, const LayoutContext &ctx, const CardVisual &visual,
                        CardInstance &inst, int row);
    void applyCrop(const CardVisual &visual, CardInstance &inst, int row) const;
    void applyFlipPayload(CardInstance &inst, int row) const;
    // Staggered open bloom plus hover/press settle, folded into one instance.
    void applyMicroAnim(CardInstance &inst, int row, bool projected, const LayoutContext &ctx) const;
    void resolveTransition(CardRenderNode *node, const LayoutContext &ctx);
    float fadeFor(const QString &key) const;
    bool tickFades(double dt);

    float flipPhase(float x, float y) const;
    float filterRollFor(qint64 cell, float x, float y);
    bool advanceFilterSwap(double dt);

    void schedulePreview();
    void startPreview();
    void publishRects(const QRectF &current, QPointF shear, const QRectF &stage);
    void publishVisibleEnd(int end);
    void publishScrollMetrics(qreal position, qreal extent);
    void stopPreview();

    QPointer<QObject> m_sourceObj;
    CardSource *m_source = nullptr;
    QPointer<QObject> m_settingsObj;
    ParamsAdapter m_params;

    QString m_mode = QStringLiteral("slices");
    std::unique_ptr<Layout> m_layout;
    int m_current = 0;
    QString m_currentKey;
    quint64 m_generation = 0;

    bool m_active = false;
    bool m_activateOnClick = false;
    bool m_interactive = true;
    int m_columns = 0;
    Spring m_entrance;
    QVariantMap m_paletteMap;
    ScenePalette m_palette;
    int m_hovered = -1;
    QRectF m_currentRect;
    QPointF m_currentShear;
    QRectF m_stageRect;
    QSizeF m_barReserve;
    int m_visibleEnd = -1;
    qreal m_scrollPosition = 0;
    qreal m_scrollExtent = 0;
    qreal m_pendingScrollPosition = 0;
    qreal m_pendingScrollExtent = 0;
    bool m_scrollQueued = false;
    QPointF m_pointer;
    bool m_pointerInside = false;
    bool m_dragging = false;
    QPointF m_dragLast;

    Spring m_flip;
    int m_flippedRow = -1;
    FlipOptions m_flipOpts;

    // Per-card micro-animation springs, only alive while the ticker runs.
    Spring m_hover;
    int m_hoverRow = -1;
    Spring m_press;
    int m_pressRow = -1;

    MotionProfile m_motion;
    double m_filterMs = 450;

    bool m_previewEnabled = true;
    double m_previewDelayMs = 250;
    bool m_previewActive = false;
    bool m_previewHasFrame = false;
    int m_previewRow = -1;
    QTimer *m_previewTimer = nullptr;
    QRectF m_pendingRect;
    QPointF m_pendingShear;
    QRectF m_pendingStage;
    bool m_rectQueued = false;
    int m_pendingVisibleEnd = -1;
    bool m_endQueued = false;

    std::unique_ptr<FrameTicker> m_ticker;
    std::unique_ptr<ThumbDecoder> m_decoder;
    std::unique_ptr<PreviewVideo> m_preview;
    QElapsedTimer m_clock;
    double m_time = 0;
    bool m_animating = false;
    QSize m_nearTile{1024, 576};

    // Reused per-frame buffers.
    std::vector<CardVisual> m_visuals;
    std::vector<CardVisual> m_transVisuals;
    std::vector<CardInstance> m_transBuf;
    std::unordered_map<QString, Spring> m_fades;

    QHash<qint64, float> m_filterIn;
    float m_filterWave = 10.0f;
    // Watchdog: a bounded swap must terminate, so a stuck roll can never blank a card.
    double m_filterElapsed = 0.0;

    Spring m_trans;
    bool m_transActive = false;
    QSet<QString> m_wanted;
};

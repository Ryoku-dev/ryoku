#pragma once

#include <qcolor.h>
#include <qlist.h>
#include <qobject.h>
#include <qqmlengine.h>
#include <qelapsedtimer.h>
#include <qtimer.h>

class BlobShape;
class BlobInvertedRect;
class BlobWave;

class BlobGroup : public QObject {
    Q_OBJECT
    QML_ELEMENT
    Q_PROPERTY(qreal smoothing READ smoothing WRITE setSmoothing NOTIFY smoothingChanged)
    Q_PROPERTY(QColor color READ color WRITE setColor NOTIFY colorChanged)
    Q_PROPERTY(qreal shadowStrength READ shadowStrength WRITE setShadowStrength NOTIFY shadowStrengthChanged)
    Q_PROPERTY(qreal shadowSize READ shadowSize WRITE setShadowSize NOTIFY shadowSizeChanged)
    Q_PROPERTY(QColor borderColor READ borderColor WRITE setBorderColor NOTIFY borderColorChanged)
    Q_PROPERTY(qreal borderWidth READ borderWidth WRITE setBorderWidth NOTIFY borderWidthChanged)
    Q_PROPERTY(QColor waveColor READ waveColor WRITE setWaveColor NOTIFY waveColorChanged)
    // When true, every rect's corner radii are read as 45-degree chamfer cuts
    // (Lithic) instead of round radii. The smooth-min weld still applies.
    Q_PROPERTY(bool cutCorners READ cutCorners WRITE setCutCorners NOTIFY cutCornersChanged)

public:
    explicit BlobGroup(QObject* parent = nullptr);
    ~BlobGroup() override;

    qreal smoothing() const { return m_smoothing; }

    void setSmoothing(qreal s);

    QColor color() const { return m_color; }

    void setColor(const QColor& c);

    qreal shadowStrength() const { return m_shadowStrength; }
    void setShadowStrength(qreal v);

    qreal shadowSize() const { return m_shadowSize; }
    void setShadowSize(qreal v);

    QColor borderColor() const { return m_borderColor; }
    void setBorderColor(const QColor& c);

    qreal borderWidth() const { return m_borderWidth; }
    void setBorderWidth(qreal v);

    // Ink traced along the wave where sound lifts it; transparent draws none.
    QColor waveColor() const { return m_waveColor; }
    void setWaveColor(const QColor& c);

    bool cutCorners() const { return m_cutCorners; }
    void setCutCorners(bool v);

    void addShape(BlobShape* shape);
    void removeShape(BlobShape* shape);

    void setInvertedRect(BlobInvertedRect* rect);
    void clearInvertedRect(BlobInvertedRect* rect);

    void addWave(BlobWave* wave);
    void removeWave(BlobWave* wave);
    const QList<BlobShape*>& shapes() const { return m_shapes; }
    const QList<BlobWave*>& waves() const { return m_waves; }

    BlobInvertedRect* invertedRect() const { return m_invertedRect; }
    void markDirty();
    void markShapeDirty(BlobShape* source);
    void ensurePhysicsUpdated();
    void requestWaveUpdate();

signals:
    void smoothingChanged();
    void colorChanged();
    void shadowStrengthChanged();
    void shadowSizeChanged();
    void borderColorChanged();
    void borderWidthChanged();
    void waveColorChanged();
    void cutCornersChanged();

private:
    void flushWaveUpdates();
    void markWaveDataDirty(BlobWave* const* waves, int count);

    qreal m_smoothing = 32.0;
    QColor m_color{ 0x44, 0x88, 0xff };
    qreal m_shadowStrength = 0.0;
    qreal m_shadowSize = 0.0;
    QColor m_borderColor{ 0, 0, 0, 0 };
    qreal m_borderWidth = 0.0;
    QColor m_waveColor{ 0, 0, 0, 0 };
    bool m_cutCorners = false;
    QList<BlobShape*> m_shapes;
    BlobInvertedRect* m_invertedRect = nullptr;
    QList<BlobWave*> m_waves;
    bool m_physicsUpdated = false;
    QTimer m_waveTimer;
    QElapsedTimer m_waveClock;
    bool m_updateWavePhysics = false;
};

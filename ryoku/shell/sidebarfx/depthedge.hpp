#pragma once

#include <qquickitem.h>
#include <qqmlintegration.h>

class DepthEdge : public QQuickItem {
    Q_OBJECT
    QML_ELEMENT
    Q_PROPERTY(qreal progress READ progress WRITE setProgress NOTIFY progressChanged)
    Q_PROPERTY(int side READ side WRITE setSide NOTIFY sideChanged)
    Q_PROPERTY(qreal intensity READ intensity WRITE setIntensity NOTIFY intensityChanged)
    Q_PROPERTY(qreal radius READ radius WRITE setRadius NOTIFY radiusChanged)

public:
    explicit DepthEdge(QQuickItem* parent = nullptr);

    qreal progress() const { return m_progress; }
    void setProgress(qreal progress);

    int side() const { return m_side; }
    void setSide(int side);

    qreal intensity() const { return m_intensity; }
    void setIntensity(qreal intensity);

    qreal radius() const { return m_radius; }
    void setRadius(qreal radius);

signals:
    void progressChanged();
    void sideChanged();
    void intensityChanged();
    void radiusChanged();

protected:
    QSGNode* updatePaintNode(QSGNode* oldNode, UpdatePaintNodeData*) override;

private:
    qreal m_progress = 0.0;
    int m_side = 0;
    qreal m_intensity = 1.0;
    qreal m_radius = 0.0;
};

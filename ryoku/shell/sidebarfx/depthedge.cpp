#include "depthedge.hpp"

#include <qmatrix4x4.h>
#include <qsggeometry.h>
#include <qsgmaterial.h>
#include <qsgmaterialshader.h>
#include <qsgnode.h>

#include <algorithm>
#include <cstring>

namespace {

class DepthEdgeShader final : public QSGMaterialShader {
public:
    DepthEdgeShader() {
        setShaderFileName(VertexStage, QStringLiteral(":/shaders/depth.vert.qsb"));
        setShaderFileName(FragmentStage, QStringLiteral(":/shaders/depth.frag.qsb"));
    }

    bool updateUniformData(RenderState& state, QSGMaterial* newMaterial, QSGMaterial*) override;
};

class DepthEdgeMaterial final : public QSGMaterial {
public:
    QSGMaterialType* type() const override {
        static QSGMaterialType type;
        return &type;
    }

    QSGMaterialShader* createShader(QSGRendererInterface::RenderMode) const override {
        return new DepthEdgeShader;
    }

    int compare(const QSGMaterial* other) const override {
        if (this < other)
            return -1;
        if (this > other)
            return 1;
        return 0;
    }

    float progress = 0.0f;
    float intensity = 1.0f;
    float radius = 0.0f;
    float width = 0.0f;
    float height = 0.0f;
    int side = 0;
};

bool DepthEdgeShader::updateUniformData(RenderState& state, QSGMaterial* newMaterial, QSGMaterial*) {
    auto* material = static_cast<DepthEdgeMaterial*>(newMaterial);
    QByteArray* buffer = state.uniformData();
    Q_ASSERT(buffer->size() >= 92);

    if (state.isMatrixDirty()) {
        const QMatrix4x4 matrix = state.combinedMatrix();
        std::memcpy(buffer->data(), matrix.constData(), 64);
    }
    if (state.isOpacityDirty()) {
        const float opacity = state.opacity();
        std::memcpy(buffer->data() + 64, &opacity, sizeof(opacity));
    }

    std::memcpy(buffer->data() + 68, &material->progress, sizeof(material->progress));
    std::memcpy(buffer->data() + 72, &material->intensity, sizeof(material->intensity));
    std::memcpy(buffer->data() + 76, &material->radius, sizeof(material->radius));
    std::memcpy(buffer->data() + 80, &material->width, sizeof(material->width));
    std::memcpy(buffer->data() + 84, &material->height, sizeof(material->height));
    std::memcpy(buffer->data() + 88, &material->side, sizeof(material->side));
    return true;
}

} // namespace

DepthEdge::DepthEdge(QQuickItem* parent)
    : QQuickItem(parent) {
    setFlag(ItemHasContents);
}

void DepthEdge::setProgress(qreal progress) {
    const qreal bounded = std::clamp(progress, 0.0, 1.0);
    if (qFuzzyCompare(m_progress, bounded))
        return;
    m_progress = bounded;
    emit progressChanged();
    update();
}

void DepthEdge::setSide(int side) {
    const int bounded = side == 1 ? 1 : 0;
    if (m_side == bounded)
        return;
    m_side = bounded;
    emit sideChanged();
    update();
}

void DepthEdge::setIntensity(qreal intensity) {
    const qreal bounded = std::max(intensity, 0.0);
    if (qFuzzyCompare(m_intensity, bounded))
        return;
    m_intensity = bounded;
    emit intensityChanged();
    update();
}

void DepthEdge::setRadius(qreal radius) {
    const qreal bounded = std::max(radius, 0.0);
    if (qFuzzyCompare(m_radius, bounded))
        return;
    m_radius = bounded;
    emit radiusChanged();
    update();
}

QSGNode* DepthEdge::updatePaintNode(QSGNode* oldNode, UpdatePaintNodeData*) {
    if (m_progress <= 0.001 || m_intensity <= 0.0 || width() <= 0.0 || height() <= 0.0) {
        delete oldNode;
        return nullptr;
    }

    auto* node = static_cast<QSGGeometryNode*>(oldNode);
    if (!node) {
        node = new QSGGeometryNode;

        auto* geometry = new QSGGeometry(QSGGeometry::defaultAttributes_TexturedPoint2D(), 4);
        geometry->setDrawingMode(QSGGeometry::DrawTriangleStrip);
        node->setGeometry(geometry);
        node->setFlag(QSGNode::OwnsGeometry);

        auto* material = new DepthEdgeMaterial;
        material->setFlag(QSGMaterial::Blending);
        node->setMaterial(material);
        node->setFlag(QSGNode::OwnsMaterial);
    }

    auto* vertices = node->geometry()->vertexDataAsTexturedPoint2D();
    const float itemWidth = static_cast<float>(width());
    const float itemHeight = static_cast<float>(height());
    vertices[0].set(0.0f, 0.0f, 0.0f, 0.0f);
    vertices[1].set(itemWidth, 0.0f, 1.0f, 0.0f);
    vertices[2].set(0.0f, itemHeight, 0.0f, 1.0f);
    vertices[3].set(itemWidth, itemHeight, 1.0f, 1.0f);
    node->markDirty(QSGNode::DirtyGeometry);

    auto* material = static_cast<DepthEdgeMaterial*>(node->material());
    material->progress = static_cast<float>(m_progress);
    material->intensity = static_cast<float>(m_intensity);
    material->radius = static_cast<float>(m_radius);
    material->width = itemWidth;
    material->height = itemHeight;
    material->side = m_side;
    node->markDirty(QSGNode::DirtyMaterial);

    return node;
}

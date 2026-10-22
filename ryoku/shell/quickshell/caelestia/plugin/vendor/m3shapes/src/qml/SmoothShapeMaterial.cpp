#include "SmoothShapeMaterial.hpp"
#include <QMatrix4x4>
#include <QRect>
#include <QSGMaterialShader>
#include <cstring>

const QSGGeometry::AttributeSet& smoothShapeAttributes() {
    static const QSGGeometry::Attribute attributes[] = {
        QSGGeometry::Attribute::createWithAttributeType(
            0, 2, QSGGeometry::FloatType, QSGGeometry::PositionAttribute),
        QSGGeometry::Attribute::createWithAttributeType(
            1, 4, QSGGeometry::UnsignedByteType, QSGGeometry::ColorAttribute),
        QSGGeometry::Attribute::createWithAttributeType(
            2, 2, QSGGeometry::FloatType, QSGGeometry::UnknownAttribute),
    };
    static const QSGGeometry::AttributeSet set = {
        3, sizeof(SmoothVertex), attributes};
    return set;
}

namespace {

// Feather width in DEVICE pixels. ~1px gives a crisp edge; the value is constant
// on screen at any scale because the shader expands after the combined matrix.
constexpr float k_aaWidthPx = 1.0f;

// std140 layout of the uniform block in smoothshape.vert:
//   mat4  qt_Matrix   offset  0, size 64
//   vec2  pixelToNdc  offset 64, size  8
//   float aaWidth     offset 72, size  4
//   float qt_Opacity  offset 76, size  4   -> block size 80
constexpr int k_uboMatrixOffset = 0;
constexpr int k_uboPixelOffset = 64;
constexpr int k_uboAaWidthOffset = 72;
constexpr int k_uboOpacityOffset = 76;
constexpr int k_uboSize = 80;

class SmoothShapeShader : public QSGMaterialShader {
public:
    SmoothShapeShader() {
        setShaderFileName(
            VertexStage, QStringLiteral(":/m3shapes/shaders/smoothshape.vert.qsb"));
        setShaderFileName(FragmentStage,
            QStringLiteral(":/m3shapes/shaders/smoothshape.frag.qsb"));
    }

    bool updateUniformData(RenderState& state, QSGMaterial* /*newMaterial*/,
        QSGMaterial* /*oldMaterial*/) override {
        QByteArray* buf = state.uniformData();
        Q_ASSERT(buf->size() >= k_uboSize);
        char* data = buf->data();

        if (state.isMatrixDirty()) {
            const QMatrix4x4 m = state.combinedMatrix();
            std::memcpy(data + k_uboMatrixOffset, m.constData(), 64);
        }

        // Refresh viewport-derived pixel size every draw: the viewport (and thus
        // the device-pixel mapping) can change without isMatrixDirty() tripping.
        const QRect vp = state.viewportRect();
        const float vpW = vp.width() > 0 ? static_cast<float>(vp.width()) : 1.0f;
        const float vpH = vp.height() > 0 ? static_cast<float>(vp.height()) : 1.0f;
        const float pixelToNdc[2] = {2.0f / vpW, 2.0f / vpH};
        std::memcpy(data + k_uboPixelOffset, pixelToNdc, sizeof(pixelToNdc));
        std::memcpy(data + k_uboAaWidthOffset, &k_aaWidthPx, sizeof(k_aaWidthPx));

        if (state.isOpacityDirty()) {
            const float opacity = state.opacity();
            std::memcpy(data + k_uboOpacityOffset, &opacity, sizeof(opacity));
        }
        return true;
    }
};

} // namespace

SmoothShapeMaterial::SmoothShapeMaterial() {
    // Blending: the feather fades to a transparent (premultiplied) edge.
    // RequiresFullMatrix: the shader needs the true combined matrix and the
    // vertices must stay in local space (no batch pre-transform/merging).
    setFlag(Blending | RequiresFullMatrix, true);
}

QSGMaterialType* SmoothShapeMaterial::type() const {
    static QSGMaterialType type;
    return &type;
}

QSGMaterialShader* SmoothShapeMaterial::createShader(
    QSGRendererInterface::RenderMode /*mode*/) const {
    return new SmoothShapeShader;
}

int SmoothShapeMaterial::compare(const QSGMaterial* /*other*/) const {
    // No per-instance state (colour is per-vertex): all instances are equal.
    return 0;
}

#include "cardrendernode.h"
#include "cardinstancesanitize.h"
#include "gpupoison.h"
#include "previewvideo.h"

#include <QFile>
#include <QMatrix4x4>
#include <QQuickWindow>
#include <rhi/qrhi.h>
#include <rhi/qshader.h>

#include <algorithm>
#include <cstring>
#include <limits>

namespace {

QShader loadShader(const QString &name)
{
    QFile f(name);
    if (!f.open(QIODevice::ReadOnly))
        qFatal("ryogami: missing shader %s", qPrintable(name));
    return QShader::fromSerialized(f.readAll());
}

// Layout of the std140 uniform block shared by card.vert and card.frag.
struct Uniforms {
    float mvp[16];
    float clip[4];
    float time;
    float vis;
    float opacity;
    float pad;
};
static_assert(sizeof(Uniforms) == 96);

constexpr QSize kFarLayer(2048, 2048);
constexpr QSize kFarTile(256, 160);
// Layouts keep up to about a dozen sharp cards resident while moving (Sandy holds its
// transition history too); growing the array copies every layer inside one frame, which
// showed as a 30-140 ms hitch on the first moves after opening.
constexpr int kNearLayers = 12;

}

CardRenderNode::CardRenderNode(QQuickWindow *window, QSize nearLayer)
    : m_window(window)
    , m_near(nearLayer, nearLayer, kNearLayers, 24, 2)
    , m_far(kFarLayer, kFarTile, 1, 4, 1)
{
}

CardRenderNode::~CardRenderNode()
{
    releaseResources();
}

void CardRenderNode::beginFrame()
{
    m_near.beginFrame();
    m_far.beginFrame();
}

void CardRenderNode::setInstances(std::vector<CardInstance> &&instances)
{
    m_instances = std::move(instances);
}

void CardRenderNode::setTransition(std::vector<CardInstance> &&from, float progress, int kind)
{
    m_transitionFrom = std::move(from);
    m_transitionProgress = progress;
    m_transitionKind = kind;
}

void CardRenderNode::setTransitionParams(QPointF origin, const QColor &accent)
{
    m_transitionOrigin = origin;
    m_transitionAccent = accent;
}

void CardRenderNode::setScene(const QRectF &bounds, const QRectF &clip, float time, float vis)
{
    m_bounds = bounds;
    m_clip = clip;
    m_time = time;
    m_vis = vis;
}

void CardRenderNode::setPreviewImage(const QImage &image)
{
    m_previewImage = image;
    const QSize cap = PreviewVideo::kMaxSize;
    if (m_previewImage.width() > cap.width() || m_previewImage.height() > cap.height())
        m_previewImage = m_previewImage.scaled(cap, Qt::KeepAspectRatio, Qt::SmoothTransformation);
    // Half a texel in from the frame's edges: the sampler then never blends
    // with what an earlier, larger frame left in the rest of the texture.
    const QSize s = m_previewImage.size();
    m_previewUv = QRectF(0.5 / cap.width(), 0.5 / cap.height(),
                         (s.width() - 1.0) / cap.width(), (s.height() - 1.0) / cap.height());
}

bool CardRenderNode::ensurePreview(QRhi *rhi, QRhiResourceUpdateBatch *batch)
{
    // One texture for the node's life, sized for the largest frame PreviewVideo
    // delivers; each frame lands in its top-left corner and the instance uv
    // selects the covered part. Recreating it per clip size meant new bindings
    // and pipelines mid-frame on every hover across clips of different
    // aspects, and on the NVIDIA GL driver that is where the field tore.
    bool changed = false;
    if (!m_preview) {
        m_previewUploaded = false;
        m_preview.reset(rhi->newTexture(QRhiTexture::RGBA8, PreviewVideo::kMaxSize));
        if (!m_preview->create()) {
            // A failed texture must not enter the bindings: leave nothing
            // bound and retry next frame.
            m_preview.reset();
            return changed;
        }
        GpuPoison::texture(batch, m_preview.get(), PreviewVideo::kMaxSize);
        changed = true;
        QImage blank(PreviewVideo::kMaxSize, QImage::Format_RGBA8888);
        blank.fill(Qt::black);
        batch->uploadTexture(m_preview.get(), blank);
    }
    if (!m_previewImage.isNull()) {
        QRhiTextureSubresourceUploadDescription frame(m_previewImage);
        frame.setDestinationTopLeft(QPoint(0, 0));
        batch->uploadTexture(m_preview.get(), QRhiTextureUploadDescription({0, 0, frame}));
        m_previewImage = QImage();
        m_previewUploaded = true;
    }
    return changed;
}

void CardRenderNode::configureCardPipeline(QRhiGraphicsPipeline *pipeline)
{
    QRhiGraphicsPipeline::TargetBlend blend;
    blend.enable = true;
    blend.srcColor = QRhiGraphicsPipeline::One;
    blend.dstColor = QRhiGraphicsPipeline::OneMinusSrcAlpha;
    blend.srcAlpha = QRhiGraphicsPipeline::One;
    blend.dstAlpha = QRhiGraphicsPipeline::OneMinusSrcAlpha;
    pipeline->setTargetBlends({blend});
    pipeline->setTopology(QRhiGraphicsPipeline::Triangles);
    pipeline->setShaderStages({
        {QRhiShaderStage::Vertex, loadShader(QStringLiteral(":/shaders/card.vert.qsb"))},
        {QRhiShaderStage::Fragment, loadShader(QStringLiteral(":/shaders/card.frag.qsb"))},
    });

    QRhiVertexInputLayout layout;
    layout.setBindings({QRhiVertexInputBinding(sizeof(CardInstance), QRhiVertexInputBinding::PerInstance)});
    std::vector<QRhiVertexInputAttribute> attributes;
    for (int slot = 0; slot < 15; ++slot) {
        const auto format = slot == 8 ? QRhiVertexInputAttribute::UInt4 : QRhiVertexInputAttribute::Float4;
        attributes.emplace_back(0, slot, format, quint32(slot * 16));
    }
    layout.setAttributes(attributes.cbegin(), attributes.cend());
    pipeline->setVertexInputLayout(layout);
    pipeline->setShaderResourceBindings(m_bindings.get());
}

void CardRenderNode::buildPipeline(QRhi *rhi)
{
    m_pipeline.reset(rhi->newGraphicsPipeline());
    configureCardPipeline(m_pipeline.get());
    m_pipelinePass.reset(renderTarget()->renderPassDescriptor()->newCompatibleRenderPassDescriptor());
    m_pipeline->setRenderPassDescriptor(m_pipelinePass.get());
    m_pipeline->setSampleCount(renderTarget()->sampleCount());
    m_pipelineSamples = renderTarget()->sampleCount();
    if (!m_pipeline->create()) {
        // Drop the failed pipeline so the next frame retries; drawing through
        // it is undefined per backend and can flash garbage over the window.
        m_pipeline.reset();
        m_pipelinePass.reset();
        m_pipelineSamples = 0;
    }
}

void CardRenderNode::buildScenePipeline(QRhi *rhi)
{
    m_scenePipeline.reset(rhi->newGraphicsPipeline());
    configureCardPipeline(m_scenePipeline.get());
    m_scenePipeline->setRenderPassDescriptor(m_transition.renderPassDescriptor());
    m_scenePipeline->setSampleCount(1);
    if (!m_scenePipeline->create())
        m_scenePipeline.reset();
}

bool CardRenderNode::ensureBindings(QRhi *rhi, bool texturesChanged)
{
    if (!m_bindings || texturesChanged) {
        m_bindings.reset(rhi->newShaderResourceBindings());
        const auto stages = QRhiShaderResourceBinding::VertexStage | QRhiShaderResourceBinding::FragmentStage;
        m_bindings->setBindings({
            QRhiShaderResourceBinding::uniformBuffer(0, stages, m_uniformBuffer.get()),
            QRhiShaderResourceBinding::sampledTexture(1, QRhiShaderResourceBinding::FragmentStage,
                                                      m_near.texture(), m_near.mipLevels() > 1 ? m_mipSampler.get() : m_flatSampler.get()),
            QRhiShaderResourceBinding::sampledTexture(2, QRhiShaderResourceBinding::FragmentStage,
                                                      m_far.texture(), m_far.mipLevels() > 1 ? m_mipSampler.get() : m_flatSampler.get()),
            QRhiShaderResourceBinding::sampledTexture(3, QRhiShaderResourceBinding::FragmentStage,
                                                      m_preview.get(), m_flatSampler.get()),
        });
        if (!m_bindings->create()) {
            m_bindings.reset();
            return false;
        }
        m_pipeline.reset();
        m_scenePipeline.reset();
    }
    return true;
}

void CardRenderNode::sanitize(std::vector<CardInstance> &instances, int nearLayers, int farLayers)
{
    const QSize pixels = renderTarget()->pixelSize();
    const float limit = std::max(4096.0f, float(std::max(pixels.width(), pixels.height())) * 8.0f);
    sanitizeCardInstances(instances, nearLayers, farLayers, m_preview != nullptr, limit);
}

quint32 CardRenderNode::updateInstanceBuffer(std::unique_ptr<QRhiBuffer> &buffer, QRhi *rhi,
                                             QRhiResourceUpdateBatch *batch,
                                             const std::vector<CardInstance> &data)
{
    const quint64 wideBytes = quint64(data.size()) * sizeof(CardInstance);
    if (wideBytes == 0 || wideBytes > std::numeric_limits<quint32>::max())
        return 0;
    const quint32 bytes = quint32(wideBytes);
    if (!buffer || buffer->size() < bytes) {
        quint32 capacity = 128 * sizeof(CardInstance);
        while (capacity < bytes && capacity <= std::numeric_limits<quint32>::max() / 2)
            capacity *= 2;
        if (capacity < bytes)
            capacity = bytes;
        if (buffer)
            buffer.release()->deleteLater();
        buffer.reset(rhi->newBuffer(QRhiBuffer::Dynamic, QRhiBuffer::VertexBuffer, capacity));
        if (!buffer->create()) {
            buffer.reset();
            return 0;
        }
        GpuPoison::buffer(batch, buffer.get());
    }
    batch->updateDynamicBuffer(buffer.get(), 0, bytes, data.data());
    return quint32(data.size());
}

void CardRenderNode::prepare()
{
    m_didTransition = false;
    m_uploadedInstanceCount = 0;
    m_uploadedFromCount = 0;
    QRhi *rhi = m_window->rhi();
    if (!rhi)
        return;
    QRhiResourceUpdateBatch *batch = rhi->nextResourceUpdateBatch();

    bool texturesChanged = m_near.commit(rhi, batch);
    texturesChanged |= m_far.commit(rhi, batch);
    texturesChanged |= ensurePreview(rhi, batch);

    // Instances are resolved on the GUI thread against the tiers as they were
    // before this frame's commit. When a grown texture array fails to
    // allocate and a tier falls back to the old size, every reference to a
    // removed layer must drop: sampling an out-of-range array layer is
    // undefined per backend and flashes arbitrary colours through the field.
    sanitize(m_instances, m_near.liveLayers(), m_far.liveLayers());
    sanitize(m_transitionFrom, m_near.liveLayers(), m_far.liveLayers());

    // A mip filter on a single-level texture reads black on GL, so those tiers get their own sampler.
    if (!m_mipSampler) {
        m_mipSampler.reset(rhi->newSampler(QRhiSampler::Linear, QRhiSampler::Linear, QRhiSampler::Linear,
                                           QRhiSampler::ClampToEdge, QRhiSampler::ClampToEdge));
        m_flatSampler.reset(rhi->newSampler(QRhiSampler::Linear, QRhiSampler::Linear, QRhiSampler::None,
                                            QRhiSampler::ClampToEdge, QRhiSampler::ClampToEdge));
        if (!m_mipSampler->create() || !m_flatSampler->create()) {
            m_mipSampler.reset();
            m_flatSampler.reset();
        }
    }
    if (!m_uniformBuffer) {
        m_uniformBuffer.reset(rhi->newBuffer(QRhiBuffer::Dynamic, QRhiBuffer::UniformBuffer, sizeof(Uniforms)));
        if (!m_uniformBuffer->create())
            m_uniformBuffer.reset();
        else
            GpuPoison::buffer(batch, m_uniformBuffer.get());
    }
    if (!m_mipSampler || !m_flatSampler || !m_uniformBuffer) {
        commandBuffer()->resourceUpdate(batch);
        return;
    }

    const bool active = transitionActive();

    m_uploadedInstanceCount = updateInstanceBuffer(m_instanceBuffer, rhi, batch, m_instances);
    if (active)
        m_uploadedFromCount = updateInstanceBuffer(m_fromBuffer, rhi, batch, m_transitionFrom);

    Uniforms u{};
    const QMatrix4x4 mvp = *projectionMatrix() * *matrix();
    std::memcpy(u.mvp, mvp.constData(), sizeof(u.mvp));
    u.clip[0] = float(m_clip.left());
    u.clip[1] = float(m_clip.top());
    u.clip[2] = float(m_clip.right());
    u.clip[3] = float(m_clip.bottom());
    u.time = m_time;
    u.vis = m_vis;
    // Scenes render at full opacity; the composite or direct draw applies the inherited opacity.
    u.opacity = active ? 1.0f : float(inheritedOpacity());
    batch->updateDynamicBuffer(m_uniformBuffer.get(), 0, sizeof(Uniforms), &u);

    if (!ensureBindings(rhi, texturesChanged)) {
        commandBuffer()->resourceUpdate(batch);
        m_uploadedInstanceCount = 0;
        m_uploadedFromCount = 0;
        return;
    }

    QRhiRenderPassDescriptor *pass = renderTarget()->renderPassDescriptor();
    if (!m_pipeline || !m_pipelinePass || !m_pipelinePass->isCompatible(pass)
        || m_pipelineSamples != renderTarget()->sampleCount())
        buildPipeline(rhi);

    bool targetsChanged = false;
    if (active) {
        targetsChanged = m_transition.ensureTargets(rhi, batch, renderTarget()->pixelSize());
        if (targetsChanged || !m_scenePipeline)
            buildScenePipeline(rhi);
        m_transition.prepare(rhi, batch, renderTarget(), m_transitionProgress, m_transitionKind,
                             m_transitionOrigin, m_transitionAccent, float(inheritedOpacity()), targetsChanged);
    }

    commandBuffer()->resourceUpdate(batch);

    // Runs after the node's own batch; it borrows the live preview texture as the incoming layer.
    if (m_sandyPassActive) {
        m_sandy.setPreviewTextures(m_previewUploaded ? m_preview.get() : nullptr, nullptr, m_previewUv);
        m_sandy.prepare(rhi, commandBuffer(), renderTarget(), m_sandyPassData, m_near, m_far, mvp, float(inheritedOpacity()));
    }

    // A recreated target is cleared even when a dependent pipeline failed.
    // That keeps a later retry from ever sampling allocation contents.
    if (active && m_transition.targetsReady()) {
        QRhiCommandBuffer *cb = commandBuffer();
        const QSize sz = m_transition.size();
        const QRhiViewport vp(0, 0, sz.width(), sz.height());
        const QColor clear(0, 0, 0, 0);
        const QRhiDepthStencilClearValue ds(1.0f, 0);
        const bool canDraw = m_scenePipeline && m_transition.ready();

        cb->beginPass(m_transition.targetA(), clear, ds);
        if (canDraw && m_fromBuffer && m_uploadedFromCount > 0) {
            cb->setGraphicsPipeline(m_scenePipeline.get());
            cb->setViewport(vp);
            cb->setShaderResources(m_bindings.get());
            const QRhiCommandBuffer::VertexInput input(m_fromBuffer.get(), 0);
            cb->setVertexInput(0, 1, &input);
            cb->draw(6, m_uploadedFromCount);
        }
        cb->endPass();

        cb->beginPass(m_transition.targetB(), clear, ds);
        if (canDraw && m_instanceBuffer && m_uploadedInstanceCount > 0) {
            cb->setGraphicsPipeline(m_scenePipeline.get());
            cb->setViewport(vp);
            cb->setShaderResources(m_bindings.get());
            const QRhiCommandBuffer::VertexInput input(m_instanceBuffer.get(), 0);
            cb->setVertexInput(0, 1, &input);
            cb->draw(6, m_uploadedInstanceCount);
        }
        cb->endPass();
        m_didTransition = canDraw;
    }
}

void CardRenderNode::render(const RenderState *)
{
    QRhiCommandBuffer *cb = commandBuffer();
    const QSize size = renderTarget()->pixelSize();

    if (m_didTransition && m_transition.ready()) {
        m_transition.render(cb, size);
    } else if (m_uploadedInstanceCount > 0 && m_pipeline && m_instanceBuffer) {
        cb->setGraphicsPipeline(m_pipeline.get());
        cb->setViewport(QRhiViewport(0, 0, size.width(), size.height()));
        cb->setShaderResources();
        const QRhiCommandBuffer::VertexInput input(m_instanceBuffer.get(), 0);
        cb->setVertexInput(0, 1, &input);
        cb->draw(6, m_uploadedInstanceCount);
    }

    if (m_sandyPassActive && m_sandy.active())
        m_sandy.render(cb, size);
}

void CardRenderNode::releaseResources()
{
    m_pipeline.reset();
    m_scenePipeline.reset();
    m_pipelinePass.reset();
    m_bindings.reset();
    m_instanceBuffer.reset();
    m_fromBuffer.reset();
    m_uniformBuffer.reset();
    m_mipSampler.reset();
    m_flatSampler.reset();
    m_preview.reset();
    m_previewUploaded = false;
    m_uploadedInstanceCount = 0;
    m_uploadedFromCount = 0;
    m_didTransition = false;
    m_transition.releaseResources();
    m_sandy.releaseResources();
    m_near.releaseResources();
    m_far.releaseResources();
}

QSGRenderNode::RenderingFlags CardRenderNode::flags() const
{
    return BoundedRectRendering | NoExternalRendering;
}

QRectF CardRenderNode::rect() const
{
    return m_bounds;
}

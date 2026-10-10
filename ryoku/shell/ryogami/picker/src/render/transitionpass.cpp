#include "transitionpass.h"
#include "gpupoison.h"

#include <QFile>
#include <rhi/qrhi.h>
#include <rhi/qshader.h>

namespace {

QShader loadShader(const QString &name)
{
    QFile f(name);
    if (!f.open(QIODevice::ReadOnly))
        qFatal("ryogami: missing shader %s", qPrintable(name));
    return QShader::fromSerialized(f.readAll());
}

// std140 layout of the block shared by transition.vert/.frag.
struct TransUniform {
    float resolution[2];
    float origin[2];
    float progress;
    quint32 kind;
    float opacity;
    float pad;
    float accent[4];
};
static_assert(sizeof(TransUniform) == 48);

}

TransitionPass::~TransitionPass()
{
    releaseResources();
}

bool TransitionPass::ensureTargets(QRhi *rhi, QRhiResourceUpdateBatch *batch, QSize pixelSize)
{
    const QSize px = pixelSize.isEmpty() ? QSize(1, 1) : pixelSize;
    if (m_texA && m_size == px)
        return false;

    releaseTargets();
    m_size = px;
    m_texA.reset(rhi->newTexture(QRhiTexture::RGBA8, px, 1, QRhiTexture::RenderTarget));
    const bool texAOk = m_texA->create();
    m_texB.reset(rhi->newTexture(QRhiTexture::RGBA8, px, 1, QRhiTexture::RenderTarget));
    const bool texBOk = m_texB->create();

    bool ok = texAOk && texBOk;
    if (ok) {
        m_rtA.reset(
            rhi->newTextureRenderTarget(QRhiTextureRenderTargetDescription(QRhiColorAttachment(m_texA.get()))));
        m_rpd.reset(m_rtA->newCompatibleRenderPassDescriptor());
        m_rtA->setRenderPassDescriptor(m_rpd.get());
        ok = m_rtA->create() && m_rpd != nullptr;
    }
    if (ok) {
        m_rtB.reset(
            rhi->newTextureRenderTarget(QRhiTextureRenderTargetDescription(QRhiColorAttachment(m_texB.get()))));
        m_rtB->setRenderPassDescriptor(m_rpd.get());
        ok = m_rtB->create();
    }
    if (!ok) {
        // Full-size render targets can fail on memory pressure or an
        // unsupported format. Begin/draw through the failed objects is
        // undefined per backend and flashes garbage across the window, so
        // drop them and let the caller draw the cards directly; the next
        // frame retries.
        releaseTargets();
        m_size = QSize();
    }
    if (ok) {
        GpuPoison::texture(batch, m_texA.get(), px);
        GpuPoison::texture(batch, m_texB.get(), px);
    }

    // The composite bindings reference the (re)allocated textures.
    m_srb.reset();
    m_pipeline.reset();
    m_pipelinePass.reset();
    m_pipelineSamples = -1;
    return true;
}

void TransitionPass::buildPipeline(QRhi *rhi, QRhiResourceUpdateBatch *batch,
                                   QRhiRenderTarget *mainTarget)
{
    if (!m_uniform) {
        m_uniform.reset(rhi->newBuffer(QRhiBuffer::Dynamic, QRhiBuffer::UniformBuffer, sizeof(TransUniform)));
        if (!m_uniform->create())
            m_uniform.reset();
        else
            GpuPoison::buffer(batch, m_uniform.get());
    }
    if (!m_sampler) {
        m_sampler.reset(rhi->newSampler(QRhiSampler::Linear, QRhiSampler::Linear, QRhiSampler::None,
                                        QRhiSampler::ClampToEdge, QRhiSampler::ClampToEdge));
        if (!m_sampler->create())
            m_sampler.reset();
    }
    if (!m_uniform || !m_sampler || !m_texA || !m_texB)
        return;
    m_srb.reset(rhi->newShaderResourceBindings());
    m_srb->setBindings({
        QRhiShaderResourceBinding::uniformBuffer(0, QRhiShaderResourceBinding::FragmentStage, m_uniform.get()),
        QRhiShaderResourceBinding::sampledTexture(1, QRhiShaderResourceBinding::FragmentStage, m_texA.get(), m_sampler.get()),
        QRhiShaderResourceBinding::sampledTexture(2, QRhiShaderResourceBinding::FragmentStage, m_texB.get(), m_sampler.get()),
    });
    if (!m_srb->create()) {
        m_srb.reset();
        return;
    }

    m_pipeline.reset(rhi->newGraphicsPipeline());
    QRhiGraphicsPipeline::TargetBlend blend;
    blend.enable = true;
    blend.srcColor = QRhiGraphicsPipeline::One;
    blend.dstColor = QRhiGraphicsPipeline::OneMinusSrcAlpha;
    blend.srcAlpha = QRhiGraphicsPipeline::One;
    blend.dstAlpha = QRhiGraphicsPipeline::OneMinusSrcAlpha;
    m_pipeline->setTargetBlends({blend});
    m_pipeline->setTopology(QRhiGraphicsPipeline::Triangles);
    m_pipeline->setShaderStages({
        {QRhiShaderStage::Vertex, loadShader(QStringLiteral(":/shaders/transition.vert.qsb"))},
        {QRhiShaderStage::Fragment, loadShader(QStringLiteral(":/shaders/transition.frag.qsb"))},
    });
    m_pipeline->setVertexInputLayout(QRhiVertexInputLayout());
    m_pipeline->setShaderResourceBindings(m_srb.get());
    m_pipelinePass.reset(mainTarget->renderPassDescriptor()->newCompatibleRenderPassDescriptor());
    m_pipeline->setRenderPassDescriptor(m_pipelinePass.get());
    m_pipeline->setSampleCount(mainTarget->sampleCount());
    m_pipelineSamples = mainTarget->sampleCount();
    if (!m_pipeline->create()) {
        m_pipeline.reset();
        m_pipelinePass.reset();
        m_pipelineSamples = -1;
    }
}

void TransitionPass::prepare(QRhi *rhi, QRhiResourceUpdateBatch *batch, QRhiRenderTarget *mainTarget,
                             float progress, int kind, QPointF origin, const QColor &accent, float opacity,
                             bool targetsChanged)
{
    if (targetsChanged || !m_srb || !m_pipeline || !m_pipelinePass
        || !m_pipelinePass->isCompatible(mainTarget->renderPassDescriptor())
        || m_pipelineSamples != mainTarget->sampleCount())
        buildPipeline(rhi, batch, mainTarget);
    if (!m_uniform)
        return;

    TransUniform u{};
    u.resolution[0] = float(m_size.width());
    u.resolution[1] = float(m_size.height());
    u.origin[0] = float(origin.x());
    u.origin[1] = float(origin.y());
    u.progress = progress;
    u.kind = quint32(kind < 0 ? 0 : kind);
    u.opacity = opacity;
    u.accent[0] = float(accent.redF());
    u.accent[1] = float(accent.greenF());
    u.accent[2] = float(accent.blueF());
    u.accent[3] = 1.0f;
    batch->updateDynamicBuffer(m_uniform.get(), 0, sizeof(TransUniform), &u);
}

void TransitionPass::render(QRhiCommandBuffer *cb, QSize pixelSize)
{
    if (!ready())
        return;
    cb->setGraphicsPipeline(m_pipeline.get());
    cb->setViewport(QRhiViewport(0, 0, pixelSize.width(), pixelSize.height()));
    cb->setShaderResources();
    cb->draw(3);
}

void TransitionPass::releaseTargets()
{
    // Destroy the render targets before the textures they attach.
    m_rtA.reset();
    m_rtB.reset();
    m_rpd.reset();
    m_texA.reset();
    m_texB.reset();
}

void TransitionPass::releaseResources()
{
    m_pipeline.reset();
    m_pipelinePass.reset();
    m_srb.reset();
    m_sampler.reset();
    m_uniform.reset();
    m_pipelineSamples = -1;
    releaseTargets();
    m_size = QSize();
}

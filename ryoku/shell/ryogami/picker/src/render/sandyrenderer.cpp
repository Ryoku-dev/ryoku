#include "sandyrenderer.h"
#include "gpupoison.h"

#include <QColor>
#include <QFile>
#include <QImage>
#include <rhi/qrhi.h>
#include <rhi/qshader.h>

#include <algorithm>
#include <cmath>
#include <cstring>

namespace {

QShader loadShader(const QString &name)
{
    QFile f(name);
    if (!f.open(QIODevice::ReadOnly))
        qFatal("ryogami: missing shader %s", qPrintable(name));
    return QShader::fromSerialized(f.readAll());
}

// Plain vec4 groups keep the std140 layout identical on every backend.
struct SandyUniform {
    float mvp[16];
    float center_res[4];
    float hero_dir_seed[4];
    float prog_time_vis_carry[4];
    float knobA[4];
    float knobB[4];
    float mix0[4];
    float ring0[4];
    float grid_style[4];
    float video0[4];
    float layerIdx[4];
    float layerTier[4];
    float uvRect[4][4];
    float prevUv[4];
};
static_assert(sizeof(SandyUniform) == 320);

QRhiGraphicsPipeline::TargetBlend premultipliedBlend()
{
    QRhiGraphicsPipeline::TargetBlend blend;
    blend.enable = true;
    blend.srcColor = QRhiGraphicsPipeline::One;
    blend.dstColor = QRhiGraphicsPipeline::OneMinusSrcAlpha;
    blend.srcAlpha = QRhiGraphicsPipeline::One;
    blend.dstAlpha = QRhiGraphicsPipeline::OneMinusSrcAlpha;
    return blend;
}

}

SandyRenderer::SandyRenderer() = default;

SandyRenderer::~SandyRenderer()
{
    releaseResources();
}

void SandyRenderer::setPreviewTextures(QRhiTexture *incoming, QRhiTexture *outgoing, const QRectF &incomingUv)
{
    m_prevIncoming = incoming;
    m_prevOutgoing = outgoing;
    m_prevIncomingUv = incomingUv;
}

SandyRenderer::Resolved SandyRenderer::resolve(TextureTier &near, TextureTier &far, const QString &key) const
{
    Resolved r;
    if (key.isEmpty())
        return r;
    const TextureTier::Slot *slot = near.texture() ? near.find(key) : nullptr;
    float tier = 0.0f;
    if (!slot) {
        slot = far.texture() ? far.find(key) : nullptr;
        tier = 1.0f;
    }
    if (!slot)
        return r;
    r.layer = float(slot->layer);
    r.tier = tier;
    r.uv[0] = float(slot->uv.x());
    r.uv[1] = float(slot->uv.y());
    r.uv[2] = float(slot->uv.width());
    r.uv[3] = float(slot->uv.height());
    r.ok = true;
    return r;
}

bool SandyRenderer::ensureStatics(QRhi *rhi, QRhiResourceUpdateBatch *batch)
{
    if (!m_uniform) {
        m_uniform.reset(rhi->newBuffer(QRhiBuffer::Dynamic, QRhiBuffer::UniformBuffer, sizeof(SandyUniform)));
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
    if (!m_dummy) {
        m_dummy.reset(rhi->newTexture(QRhiTexture::RGBA8, QSize(1, 1)));
        if (!m_dummy->create()) {
            m_dummy.reset();
        } else {
            GpuPoison::texture(batch, m_dummy.get(), QSize(1, 1));
            QImage blank(1, 1, QImage::Format_RGBA8888);
            blank.fill(Qt::transparent);
            batch->uploadTexture(m_dummy.get(), blank);
        }
    }
    return m_uniform && m_sampler && m_dummy;
}

void SandyRenderer::ensureFieldBindings(QRhi *rhi, QRhiTexture *nearTex, QRhiTexture *farTex,
                                        QRhiTexture *prevTex, QRhiTexture *prevOutTex)
{
    if (m_fieldBindings && m_boundNear == nearTex && m_boundFar == farTex && m_boundPrev == prevTex
        && m_boundPrevOut == prevOutTex)
        return;
    m_fieldBindings.reset(rhi->newShaderResourceBindings());
    const auto stages = QRhiShaderResourceBinding::VertexStage | QRhiShaderResourceBinding::FragmentStage;
    m_fieldBindings->setBindings({
        QRhiShaderResourceBinding::uniformBuffer(0, stages, m_uniform.get()),
        QRhiShaderResourceBinding::sampledTexture(1, stages, nearTex, m_sampler.get()),
        QRhiShaderResourceBinding::sampledTexture(2, stages, farTex, m_sampler.get()),
        QRhiShaderResourceBinding::sampledTexture(3, stages, prevTex, m_sampler.get()),
        QRhiShaderResourceBinding::sampledTexture(4, stages, prevOutTex, m_sampler.get()),
    });
    if (!m_fieldBindings->create())
        m_fieldBindings.reset();
    m_boundNear = nearTex;
    m_boundFar = farTex;
    m_boundPrev = prevTex;
    m_boundPrevOut = prevOutTex;
    // Pipelines hold a reference to the bindings they were created with, so they rebuild too.
    m_inlinePipeline.reset();
    m_offscreenPipeline.reset();
}

void SandyRenderer::buildFieldPipeline(QRhi *rhi, QRhiRenderPassDescriptor *rp, int samples,
                                       std::unique_ptr<QRhiGraphicsPipeline> &out)
{
    out.reset(rhi->newGraphicsPipeline());
    out->setTargetBlends({premultipliedBlend()});
    out->setTopology(QRhiGraphicsPipeline::Triangles);
    out->setShaderStages({
        {QRhiShaderStage::Vertex, loadShader(QStringLiteral(":/shaders/sandy.vert.qsb"))},
        {QRhiShaderStage::Fragment, loadShader(QStringLiteral(":/shaders/sandy.frag.qsb"))},
    });
    out->setVertexInputLayout(QRhiVertexInputLayout());
    out->setShaderResourceBindings(m_fieldBindings.get());
    out->setRenderPassDescriptor(rp);
    out->setSampleCount(samples);
    if (!out->create())
        out.reset();
}

void SandyRenderer::ensureInlinePipeline(QRhi *rhi, QRhiRenderPassDescriptor *rp, int samples)
{
    if (m_inlinePipeline && m_inlineRpOwned && m_inlineRpOwned->isCompatible(rp) && m_inlineSamples == samples)
        return;
    m_inlineRpOwned.reset(rp->newCompatibleRenderPassDescriptor());
    buildFieldPipeline(rhi, m_inlineRpOwned.get(), samples, m_inlinePipeline);
    m_inlineSamples = samples;
}

void SandyRenderer::ensureBlitBindings(QRhi *rhi, QRhiTexture *tex)
{
    if (m_blitBindings && m_blitBound == tex)
        return;
    m_blitBindings.reset(rhi->newShaderResourceBindings());
    m_blitBindings->setBindings({
        QRhiShaderResourceBinding::sampledTexture(0, QRhiShaderResourceBinding::FragmentStage, tex,
                                                  m_sampler.get()),
    });
    if (!m_blitBindings->create())
        m_blitBindings.reset();
    m_blitBound = tex;
    m_blitPipeline.reset();
}

void SandyRenderer::ensureBlitPipeline(QRhi *rhi, QRhiRenderPassDescriptor *rp, int samples)
{
    if (m_blitPipeline && m_blitRpOwned && m_blitRpOwned->isCompatible(rp) && m_blitSamples == samples)
        return;
    m_blitRpOwned.reset(rp->newCompatibleRenderPassDescriptor());
    m_blitPipeline.reset(rhi->newGraphicsPipeline());
    m_blitPipeline->setTargetBlends({premultipliedBlend()});
    m_blitPipeline->setTopology(QRhiGraphicsPipeline::Triangles);
    m_blitPipeline->setShaderStages({
        {QRhiShaderStage::Vertex, loadShader(QStringLiteral(":/shaders/sandyblit.vert.qsb"))},
        {QRhiShaderStage::Fragment, loadShader(QStringLiteral(":/shaders/sandyblit.frag.qsb"))},
    });
    m_blitPipeline->setVertexInputLayout(QRhiVertexInputLayout());
    m_blitPipeline->setShaderResourceBindings(m_blitBindings.get());
    m_blitPipeline->setRenderPassDescriptor(m_blitRpOwned.get());
    m_blitPipeline->setSampleCount(samples);
    if (!m_blitPipeline->create())
        m_blitPipeline.reset();
    m_blitSamples = samples;
}

bool SandyRenderer::ensureOffscreen(QRhi *rhi, QRhiResourceUpdateBatch *batch, QSize size)
{
    if (m_offscreen && m_offSize == size)
        return true;
    releaseOffscreen();
    m_offSize = size;
    m_offscreen.reset(rhi->newTexture(QRhiTexture::RGBA8, size, 1, QRhiTexture::RenderTarget));
    if (!m_offscreen->create()) {
        releaseOffscreen();
        return false;
    }
    QRhiColorAttachment attachment(m_offscreen.get());
    QRhiTextureRenderTargetDescription desc(attachment);
    m_offRT.reset(rhi->newTextureRenderTarget(desc));
    m_offRp.reset(m_offRT->newCompatibleRenderPassDescriptor());
    m_offRT->setRenderPassDescriptor(m_offRp.get());
    if (!m_offRp || !m_offRT->create()) {
        releaseOffscreen();
        return false;
    }
    GpuPoison::texture(batch, m_offscreen.get(), size);
    return true;
}

void SandyRenderer::releaseOffscreen()
{
    m_offscreenPipeline.reset();
    m_offRT.reset();
    m_offRp.reset();
    m_offscreen.reset();
    m_blitPipeline.reset();
    m_blitRpOwned.reset();
    m_blitBindings.reset();
    m_blitBound = nullptr;
    m_blitSamples = -1;
    m_offSize = QSize();
}

void SandyRenderer::drawField(QRhiCommandBuffer *cb) const
{
    if (m_halfDraw) {
        const uint32_t first = m_vertexCount / 2u;
        cb->draw(m_vertexCount - first, 1, first, 0);
    } else {
        cb->draw(m_vertexCount, 1, 0, 0);
    }
}

void SandyRenderer::prepare(QRhi *rhi, QRhiCommandBuffer *cb, QRhiRenderTarget *mainTarget,
                            const SandyPass &pass, TextureTier &near, TextureTier &far,
                            const QMatrix4x4 &mvp, float opacity)
{
    m_hasPass = false;
    if (!rhi)
        return;

    Resolved rA = resolve(near, far, pass.keyA);
    Resolved rB = resolve(near, far, pass.keyB);
    if (!rB.ok)
        rB = rA;
    Resolved rB2 = resolve(near, far, pass.keyB2);
    if (!rB2.ok)
        rB2 = rB;
    Resolved rB3 = resolve(near, far, pass.keyB3);
    if (!rB3.ok)
        rB3 = rB2;
    if (!rA.ok)
        return;

    m_hasPass = true;
    m_scaled = pass.resScale < 0.999f;
    const uint32_t gx = uint32_t(std::max(pass.grid[0], 1.0f));
    const uint32_t gy = uint32_t(std::max(pass.grid[1], 1.0f));
    m_vertexCount = gx * gy * 12u;
    m_halfDraw = pass.swirl >= 0.55f || pass.swapStyle >= 16.5f;

    QRhiResourceUpdateBatch *batch = rhi->nextResourceUpdateBatch();
    if (!ensureStatics(rhi, batch)) {
        cb->resourceUpdate(batch);
        m_hasPass = false;
        return;
    }

    SandyUniform u{};
    QMatrix4x4 useMvp;
    if (m_scaled) {
        QMatrix4x4 ortho;
        ortho.ortho(0.0f, pass.resolution[0], pass.resolution[1], 0.0f, -1.0f, 1.0f);
        useMvp = rhi->clipSpaceCorrMatrix() * ortho;
    } else {
        useMvp = mvp;
    }
    std::memcpy(u.mvp, useMvp.constData(), sizeof(u.mvp));
    u.center_res[0] = pass.center[0];
    u.center_res[1] = pass.center[1];
    u.center_res[2] = pass.resolution[0];
    u.center_res[3] = pass.resolution[1];
    u.hero_dir_seed[0] = pass.hero[0];
    u.hero_dir_seed[1] = pass.hero[1];
    u.hero_dir_seed[2] = pass.dir;
    u.hero_dir_seed[3] = pass.seed;
    u.prog_time_vis_carry[0] = pass.progress;
    u.prog_time_vis_carry[1] = pass.time;
    u.prog_time_vis_carry[2] = std::clamp(opacity, 0.0f, 1.0f);
    u.prog_time_vis_carry[3] = pass.carry;
    u.knobA[0] = pass.strands;
    u.knobA[1] = pass.twist;
    u.knobA[2] = pass.orbit;
    u.knobA[3] = pass.turbulence;
    u.knobB[0] = pass.waist;
    u.knobB[1] = pass.front;
    u.knobB[2] = pass.fan;
    u.knobB[3] = pass.arc;
    u.mix0[0] = pass.bcut;
    u.mix0[1] = pass.bmix;
    u.mix0[2] = pass.swapLoop;
    u.mix0[3] = pass.swirl;
    u.ring0[0] = pass.wave;
    u.ring0[1] = pass.ringSpin;
    u.ring0[2] = pass.ringWave;
    u.ring0[3] = pass.ringSoft;
    u.grid_style[0] = pass.grid[0];
    u.grid_style[1] = pass.grid[1];
    u.grid_style[2] = pass.swapStyle;
    u.grid_style[3] = pass.ringSize;
    u.video0[0] = pass.videoIn;
    u.video0[1] = pass.videoOut;
    u.layerIdx[0] = rA.layer;
    u.layerIdx[1] = rB.layer;
    u.layerIdx[2] = rB2.layer;
    u.layerIdx[3] = rB3.layer;
    u.layerTier[0] = rA.tier;
    u.layerTier[1] = rB.tier;
    u.layerTier[2] = rB2.tier;
    u.layerTier[3] = rB3.tier;
    std::memcpy(u.uvRect[0], rA.uv, sizeof(rA.uv));
    std::memcpy(u.uvRect[1], rB.uv, sizeof(rB.uv));
    std::memcpy(u.uvRect[2], rB2.uv, sizeof(rB2.uv));
    std::memcpy(u.uvRect[3], rB3.uv, sizeof(rB3.uv));
    u.prevUv[0] = float(m_prevIncomingUv.x());
    u.prevUv[1] = float(m_prevIncomingUv.y());
    u.prevUv[2] = float(m_prevIncomingUv.width());
    u.prevUv[3] = float(m_prevIncomingUv.height());
    batch->updateDynamicBuffer(m_uniform.get(), 0, sizeof(u), &u);

    QRhiTexture *prevTex = m_prevIncoming ? m_prevIncoming : m_dummy.get();
    QRhiTexture *prevOutTex = m_prevOutgoing ? m_prevOutgoing : m_dummy.get();
    ensureFieldBindings(rhi, near.texture(), far.texture(), prevTex, prevOutTex);
    if (!m_fieldBindings) {
        cb->resourceUpdate(batch);
        m_hasPass = false;
        return;
    }

    if (m_scaled) {
        const QSize mainPx = mainTarget->pixelSize();
        const int ow = std::max(1, int(std::ceil(mainPx.width() * pass.resScale)));
        const int oh = std::max(1, int(std::ceil(mainPx.height() * pass.resScale)));
        if (!ensureOffscreen(rhi, batch, QSize(ow, oh))) {
            cb->resourceUpdate(batch);
            m_hasPass = false;
            return;
        }
        if (!m_offscreenPipeline)
            buildFieldPipeline(rhi, m_offRp.get(), 1, m_offscreenPipeline);
        ensureBlitBindings(rhi, m_offscreen.get());
        if (m_blitBindings)
            ensureBlitPipeline(rhi, mainTarget->renderPassDescriptor(), mainTarget->sampleCount());

        cb->beginPass(m_offRT.get(), QColor(0, 0, 0, 0), QRhiDepthStencilClearValue(1.0f, 0), batch);
        if (m_offscreenPipeline && m_fieldBindings) {
            cb->setGraphicsPipeline(m_offscreenPipeline.get());
            cb->setViewport(QRhiViewport(0, 0, ow, oh));
            cb->setShaderResources(m_fieldBindings.get());
            drawField(cb);
        }
        cb->endPass();
        if (!m_offscreenPipeline || !m_fieldBindings || !m_blitPipeline || !m_blitBindings)
            m_hasPass = false;
    } else {
        releaseOffscreen();
        ensureInlinePipeline(rhi, mainTarget->renderPassDescriptor(), mainTarget->sampleCount());
        cb->resourceUpdate(batch);
    }
}

void SandyRenderer::render(QRhiCommandBuffer *cb, QSize pixelSize)
{
    if (!m_hasPass)
        return;
    cb->setViewport(QRhiViewport(0, 0, pixelSize.width(), pixelSize.height()));
    if (m_scaled) {
        if (!m_blitPipeline || !m_blitBindings)
            return;
        cb->setGraphicsPipeline(m_blitPipeline.get());
        cb->setShaderResources(m_blitBindings.get());
        cb->draw(3, 1, 0, 0);
    } else {
        if (!m_inlinePipeline || !m_fieldBindings)
            return;
        cb->setGraphicsPipeline(m_inlinePipeline.get());
        cb->setShaderResources(m_fieldBindings.get());
        drawField(cb);
    }
}

void SandyRenderer::releaseResources()
{
    m_hasPass = false;
    releaseOffscreen();
    m_inlinePipeline.reset();
    m_inlineRpOwned.reset();
    m_inlineSamples = -1;
    m_fieldBindings.reset();
    m_boundNear = nullptr;
    m_boundFar = nullptr;
    m_boundPrev = nullptr;
    m_boundPrevOut = nullptr;
    m_uniform.reset();
    m_sampler.reset();
    m_dummy.reset();
}

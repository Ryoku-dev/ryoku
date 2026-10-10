#pragma once

#include <QColor>
#include <QPointF>
#include <QSize>

#include <memory>

class QRhi;
class QRhiBuffer;
class QRhiCommandBuffer;
class QRhiGraphicsPipeline;
class QRhiRenderPassDescriptor;
class QRhiRenderTarget;
class QRhiResourceUpdateBatch;
class QRhiSampler;
class QRhiShaderResourceBindings;
class QRhiTexture;
class QRhiTextureRenderTarget;

class TransitionPass
{
public:
    ~TransitionPass();

    // True when the targets were recreated and the offscreen card pipeline must rebuild.
    bool ensureTargets(QRhi *rhi, QRhiResourceUpdateBatch *batch, QSize pixelSize);

    QRhiTextureRenderTarget *targetA() const { return m_rtA.get(); }
    QRhiTextureRenderTarget *targetB() const { return m_rtB.get(); }
    QRhiRenderPassDescriptor *renderPassDescriptor() const { return m_rpd.get(); }
    QSize size() const { return m_size; }
    bool ready() const { return m_pipeline && m_rtA && m_rtB && m_texA && m_texB; }
    bool targetsReady() const { return m_rtA && m_rtB && m_texA && m_texB; }

    // Call in prepare(), before the main render pass begins.
    void prepare(QRhi *rhi, QRhiResourceUpdateBatch *batch, QRhiRenderTarget *mainTarget,
                 float progress, int kind, QPointF origin, const QColor &accent, float opacity,
                 bool targetsChanged);

    void render(QRhiCommandBuffer *cb, QSize pixelSize);

    void releaseResources();

private:
    void buildPipeline(QRhi *rhi, QRhiResourceUpdateBatch *batch, QRhiRenderTarget *mainTarget);
    void releaseTargets();

    QSize m_size;
    std::unique_ptr<QRhiTexture> m_texA;
    std::unique_ptr<QRhiTexture> m_texB;
    std::unique_ptr<QRhiTextureRenderTarget> m_rtA;
    std::unique_ptr<QRhiTextureRenderTarget> m_rtB;
    std::unique_ptr<QRhiRenderPassDescriptor> m_rpd;

    std::unique_ptr<QRhiBuffer> m_uniform;
    std::unique_ptr<QRhiSampler> m_sampler;
    std::unique_ptr<QRhiShaderResourceBindings> m_srb;
    std::unique_ptr<QRhiGraphicsPipeline> m_pipeline;
    std::unique_ptr<QRhiRenderPassDescriptor> m_pipelinePass;
    int m_pipelineSamples = 0;
};

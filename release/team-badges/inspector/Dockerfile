# syntax=docker/dockerfile:1
ARG BASE_IMAGE
FROM ${BASE_IMAGE}
ENV PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1 PIP_DISABLE_PIP_VERSION_CHECK=1
ENV LIBGL_ALWAYS_SOFTWARE=1 GALLIUM_DRIVER=llvmpipe LP_NUM_THREADS=2 OMP_NUM_THREADS=2
RUN apt-get update && apt-get install -y --no-install-recommends \
    xvfb libgl1 libegl1 libgl1-mesa-dri libglib2.0-0 libdbus-1-3 \
    libx11-xcb1 libxkbcommon-x11-0 libxcb-cursor0 libxcb-icccm4 \
    libxcb-keysyms1 libxcb-shape0 libxcb-xinerama0 libxcb-render-util0 \
    libxcb-randr0 libasound2t64 libpulse0 libgssapi-krb5-2 fontconfig \
    && rm -rf /var/lib/apt/lists/*
COPY requirements-worker.txt /opt/ryoku-inspector/requirements-worker.txt
RUN python -m pip install --no-cache-dir -r /opt/ryoku-inspector/requirements-worker.txt
COPY policy.py worker.py ProfileStore.qml Quickshell.qml Ambient.qml prepare_renderer.py /opt/ryoku-inspector/
# private_renderer is an explicit local build context, never fetched from a URL
COPY --from=private_renderer /renderer /opt/ryoku-inspector/renderer
COPY --from=private_renderer /LICENSE /usr/share/licenses/ryoku-inspector/LICENSE.Mika
COPY --from=private_renderer /LICENSE.Ryoku /usr/share/licenses/ryoku-inspector/LICENSE.Ryoku
COPY --from=private_renderer /NOTICE.Ryoku /usr/share/licenses/ryoku-inspector/NOTICE.Ryoku
COPY --from=private_renderer /LICENSE.Noto /usr/share/licenses/ryoku-inspector/LICENSE.Noto
COPY --from=private_renderer /LICENSE.MaterialSymbols /usr/share/licenses/ryoku-inspector/LICENSE.MaterialSymbols
RUN python /opt/ryoku-inspector/prepare_renderer.py \
    && chmod -R a+rX /opt/ryoku-inspector /usr/share/licenses/ryoku-inspector \
    && mkdir -p /input /work
WORKDIR /work
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/python", "/opt/ryoku-inspector/worker.py"]

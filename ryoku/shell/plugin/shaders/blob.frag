#version 440

layout(location = 0) in vec2 qt_TexCoord0;
layout(location = 0) out vec4 fragColor;

layout(std140, binding = 0) uniform buf {
    mat4 qt_Matrix;
    float qt_Opacity;
    float paddedX;
    float paddedY;
    float paddedW;
    float paddedH;
    float smoothFactor;
    int rectCount;
    int myIndex;
    vec4 color;
    int hasInverted;
    float invertedRadius;
    float shadowStrength;
    float shadowSize;
    vec4 invertedOuter;
    vec4 invertedInner;
    vec4 rectData[80];
    vec4 borderColor;
    float borderWidth;
    float cutCorners;
    vec4 waveInfo[4];
    vec4 waveData[64];
    vec4 waveColor;
};

float sdRoundedBox(vec2 p, vec2 center, vec2 halfSize, float radius) {
    vec2 d = abs(p - center) - halfSize + vec2(radius);
    return length(max(d, vec2(0.0))) + min(max(d.x, d.y), 0.0) - radius;
}

float sdRoundedBox4(vec2 p, vec2 center, vec2 halfSize, vec4 r) {
    // r = (topRight, bottomRight, bottomLeft, topLeft)
    p -= center;
    r.xy = (p.x > 0.0) ? r.xy : r.wz;
    r.x  = (p.y > 0.0) ? r.y : r.x;
    vec2 q = abs(p) - halfSize + r.x;
    return min(max(q.x, q.y), 0.0) + length(max(q, 0.0)) - r.x;
}

// A box whose corners are 45-degree chamfer cuts instead of round arcs: the
// intersection of the box with four diagonal half-planes, one per corner. r is
// the cut leg (tr, br, bl, tl). The smooth-min weld still blends cut shapes.
float sdCutBox4(vec2 p, vec2 center, vec2 halfSize, vec4 r) {
    p -= center;
    vec2 q = abs(p);
    float cut = (p.x > 0.0) ? ((p.y > 0.0) ? r.y : r.x) : ((p.y > 0.0) ? r.z : r.w);
    float box = max(q.x - halfSize.x, q.y - halfSize.y);
    // diagonal corner plane: q.x + q.y <= halfSize.x + halfSize.y - cut
    float diag = (q.x + q.y - (halfSize.x + halfSize.y - cut)) * 0.70710678;
    return max(box, diag);
}

// Dispatch: cut corners for Lithic, round for the other designs.
float sdCornerBox4(vec2 p, vec2 center, vec2 halfSize, vec4 r) {
    return cutCorners > 0.5 ? sdCutBox4(p, center, halfSize, r)
                            : sdRoundedBox4(p, center, halfSize, r);
}

float sdTrapezoid(vec2 p, float r1, float r2, float he) {
    vec2 k1 = vec2(r2, he);
    vec2 k2 = vec2(r2 - r1, 2.0 * he);
    p.x = abs(p.x);
    vec2 ca = vec2(p.x - min(p.x, p.y < 0.0 ? r1 : r2), abs(p.y) - he);
    vec2 cb = p - k1 + k2 * clamp(dot(k1 - p, k2) / dot(k2, k2), 0.0, 1.0);
    float s = cb.x < 0.0 && ca.y < 0.0 ? -1.0 : 1.0;
    return s * sqrt(min(dot(ca, ca), dot(cb, cb)));
}

float sdBox(vec2 p, vec2 center, vec2 halfSize) {
    vec2 d = abs(p - center) - halfSize;
    return length(max(d, vec2(0.0))) + min(max(d.x, d.y), 0.0);
}

float smin(float a, float b, float k) {
    // Circular smooth min — the blend fillet is a true circular arc of radius k,
    // tangent to both surfaces (not a polynomial/squircle curve). Deviates from
    // min(a, b) only in the corner region where BOTH a < k and b < k (unlike the
    // cubic it replaced, which deviated over the whole band |a - b| < k). Always
    // <= min(a, b); max blend depth at a == b is (sqrt(2) - 1) * k. It is C1 but
    // not C2 at the support boundary, so a circular-arc fillet shows the usual
    // line-meets-arc curvature step — by design, that is the "circular" look.
    return max(k, min(a, b)) - length(max(vec2(k) - vec2(a, b), vec2(0.0)));
}

float smax(float a, float b, float k) {
    // Circular smooth max — dual of smin: -smin(-a, -b, k). Always >= max(a, b).
    return min(-k, max(a, b)) + length(max(vec2(a, b) + vec2(k), vec2(0.0)));
}

float smaxSharpA(float a, float b, float k) {
    // Circular smax variant that keeps a's boundary sharp (no inward rounding at
    // a = 0). Used for the frame outer edge so it always fills to the edges.
    float sm = min(-k, max(a, b)) + length(max(vec2(a, b) + vec2(k), vec2(0.0)));
    float blend = (sm - max(a, b)) * smoothstep(0.0, k * 0.5, -a);
    return max(a, b) + blend;
}

float waveControlPoint(int w, int i) {
    int bounded = clamp(i, 0, 63);
    int component = bounded % 4;
    return waveData[w * 16 + bounded / 4][component];
}

bool waveStepped(int w) {
    return waveInfo[w].z < 0.0;
}

int waveCount(int w) {
    return clamp(int(abs(waveInfo[w].z) + 0.5), 2, 64);
}

float waveHeight(int w, float x) {
    int count = waveCount(w);
    float span = waveInfo[w].y - waveInfo[w].x;
    if (waveInfo[w].w < 0.0 || span <= 0.0 || x < waveInfo[w].x || x > waveInfo[w].y)
        return 0.0;

    float position = (x - waveInfo[w].x) / span * float(count - 1);
    int point = int(floor(position));
    float t = position - float(point);
    float p0 = waveControlPoint(w, point - 1);
    float p1 = waveControlPoint(w, point);
    float p2 = waveControlPoint(w, point + 1);
    float p3 = waveControlPoint(w, point + 2);
    float t2 = t * t;
    float t3 = t2 * t;
    return max(0.0, 0.5 * ((2.0 * p1) + (-p0 + p2) * t
        + (2.0 * p0 - 5.0 * p1 + 4.0 * p2 - p3) * t2
        + (-p0 + 3.0 * p1 - 3.0 * p2 + p3) * t3));
}

float waveBlocks(int w, vec2 p, float top, float bottom) {
    int count = waveCount(w);
    float span = waveInfo[w].y - waveInfo[w].x;
    if (span <= 0.0)
        return 1e10;
    float pitch = span / float(count);
    float gap = clamp(pitch * 0.24, 2.0, 6.0);
    float halfW = max(0.5, (pitch - gap) * 0.5);
    int nearest = int(floor((p.x - waveInfo[w].x) / pitch));
    float d = 1e10;
    for (int j = nearest - 1; j <= nearest + 1; j++) {
        if (j < 0 || j >= count)
            continue;
        float h = waveControlPoint(w, j);
        if (h < 0.5)
            continue;
        float r = min(2.0, min(halfW, h * 0.5));
        vec2 c = vec2(waveInfo[w].x + (float(j) + 0.5) * pitch, (top + bottom + h) * 0.5);
        vec2 halfSize = vec2(halfW, (bottom + h - top) * 0.5);
        d = min(d, sdRoundedBox4(p, c, halfSize, vec4(0.0, r, r, 0.0)));
    }
    return d;
}

int waveSlot(int rectIndex) {
    for (int w = 0; w < 4; ++w) {
        if (waveInfo[w].w == float(rectIndex))
            return w;
    }
    return -1;
}

float waveLift(float x) {
    for (int w = 0; w < 4; ++w) {
        if (waveInfo[w].w < 0.0 || x < waveInfo[w].x || x > waveInfo[w].y)
            continue;
        if (!waveStepped(w))
            return waveHeight(w, x);
        int count = waveCount(w);
        float pitch = (waveInfo[w].y - waveInfo[w].x) / float(count);
        if (pitch <= 0.0)
            return 0.0;
        return waveControlPoint(w, clamp(int(floor((x - waveInfo[w].x) / pitch)), 0, count - 1));
    }
    return 0.0;
}

void main() {
    vec2 pixel = vec2(paddedX, paddedY) + qt_TexCoord0 * vec2(paddedW, paddedH);

    // Phase 1: compute per-rect SDF, track owner. We can't smin yet because excluded
    // pairs need to skip the smooth blend, which requires pairwise pass below.
    float dArr[16];
    int owner = -2;
    float minDist = 1e10;

    for (int i = 0; i < rectCount; i++) {
        vec4 rect = rectData[i * 5];         // cx, cy, hw, hh
        vec4 props = rectData[i * 5 + 1];    // excludeMask(int bits), offsetX, offsetY, minEig
        vec4 invDm = rectData[i * 5 + 2];    // inverse deform matrix
        vec4 sh = rectData[i * 5 + 3];       // screenHalfX, screenHalfY, sinkWeight, taper
        vec4 radii = rectData[i * 5 + 4];    // effective per-corner radii (tr, br, bl, tl)

        // Offset center for asymmetric deformation
        vec2 center = rect.xy + props.yz;

        // AABB early-out: skip rects far from this pixel
        vec2 extent = sh.xy + vec2(smoothFactor * 1.5);
        if (abs(pixel.x - center.x) > extent.x || abs(pixel.y - center.y) > extent.y) {
            dArr[i] = 1e10;
            continue;
        }

        // Apply pre-computed inverse deformation to the evaluation point
        mat2 invDeform = mat2(invDm.xy, invDm.zw);
        vec2 transformedPixel = center + invDeform * (pixel - center);

        float d;
        int w = waveSlot(i);
        if (w >= 0 && waveStepped(w)) {
            d = min(sdCornerBox4(transformedPixel, center, rect.zw, radii),
                waveBlocks(w, transformedPixel, center.y, center.y + rect.w));
        } else if (w >= 0) {
            float height = waveHeight(w, pixel.x);
            vec2 waveCenter = center + vec2(0.0, height * 0.5);
            vec2 waveHalfSize = rect.zw + vec2(0.0, height * 0.5);
            d = sdCornerBox4(transformedPixel, waveCenter, waveHalfSize, radii);
            if (transformedPixel.y > center.y) {
                float slope = (waveHeight(w, pixel.x + 1.0) - waveHeight(w, pixel.x - 1.0)) * 0.5;
                d /= sqrt(1.0 + slope * slope);
            }
        } else if (sh.w != 0.0) {
            float topHalf = rect.z + min(sh.w, 0.0);
            float bottomHalf = rect.z - max(sh.w, 0.0);
            float maxRadius = max(min(min(topHalf, bottomHalf), rect.w) - 0.001, 0.0);
            float radius = clamp(max(radii.y, radii.z), 0.0, maxRadius);
            d = sdTrapezoid(
                    transformedPixel - center,
                    topHalf - radius,
                    bottomHalf - radius,
                    rect.w - radius)
                - radius;
        } else {
            d = sdCornerBox4(transformedPixel, center, rect.zw, radii);
        }

        // Use pre-computed minimum eigenvalue for SDF correction
        d *= max(props.w, 0.01);

        // Scale SDF on the axis facing a nearby border to narrow the smin blend zone
        // in that direction only, without reducing k (which would cause sharp edges).
        if (hasInverted != 0) {
            vec2 screenHalf = sh.xy;

            float distY0 = (center.y + screenHalf.y) - (invertedInner.y - invertedInner.w);
            float distY1 = (invertedInner.y + invertedInner.w) - (center.y - screenHalf.y);
            float distX0 = (center.x + screenHalf.x) - (invertedInner.x - invertedInner.z);
            float distX1 = (invertedInner.x + invertedInner.z) - (center.x - screenHalf.x);

            // 0 = far from border, 1 = at border (max compression)
            float yProx = 1.0 - min(
                smoothstep(0.0, smoothFactor, distY0),
                smoothstep(0.0, smoothFactor, distY1)
            );
            float xProx = 1.0 - min(
                smoothstep(0.0, smoothFactor, distX0),
                smoothstep(0.0, smoothFactor, distX1)
            );

            // Smooth axis weights: gradient-based at corners, face-based inside.
            vec2 q = abs(pixel - center) - screenHalf;
            vec2 qp = max(q, vec2(0.0));
            float cornerLen = length(qp);

            // Gradient direction in corner region (smooth 90-degree rotation)
            float gradX = qp.x / max(cornerLen, 0.001);
            float gradY = qp.y / max(cornerLen, 0.001);

            // Smooth face weights for inside/edge (no hard branch)
            float faceY = smoothstep(-4.0, 4.0, q.y - q.x);
            float faceX = 1.0 - faceY;

            // Blend: gradient in corner region, face-based inside
            float t = smoothstep(0.0, 2.0, cornerLen);
            float xWeight = mix(faceX, gradX, t);
            float yWeight = mix(faceY, gradY, t);

            float boost = 3.0;
            float scale = 1.0 + (xProx * xWeight + yProx * yWeight) * boost;
            d *= scale;
        }

        dArr[i] = d;
        if (d < smoothFactor && d < minDist) {
            minDist = d;
            owner = i;
        }
    }

    // Phase 2: hard-min baseline over all rects.
    float mergedSdf = 1e10;
    for (int i = 0; i < rectCount; i++) {
        mergedSdf = min(mergedSdf, dArr[i]);
    }

    // Phase 3: pair-wise smin contributions, skipping excluded pairs. Pair smin <= min,
    // so taking the min over all non-excluded pair smins gives the smoothly-merged SDF.
    for (int i = 0; i < rectCount; i++) {
        if (dArr[i] >= 1e9)
            continue;
        int excludeMask = floatBitsToInt(rectData[i * 5 + 1].x);
        for (int j = i + 1; j < rectCount; j++) {
            if (dArr[j] >= 1e9)
                continue;
            if ((excludeMask & (1 << j)) != 0)
                continue;
            // Circular smin deviates from min only where BOTH dArr are < smoothFactor.
            if (max(dArr[i], dArr[j]) >= smoothFactor)
                continue;
            mergedSdf = min(mergedSdf, smin(dArr[i], dArr[j], smoothFactor));
        }
    }

    float frameDist = 1e10;
    if (hasInverted != 0) {
        float dOuter = sdBox(pixel, invertedOuter.xy, invertedOuter.zw) - 1.0;
        float dInner = sdRoundedBox(pixel, invertedInner.xy, invertedInner.zw, invertedRadius);

        // Border sinks: track the opposite rect edge, clamped to border thickness
        float innerTop = invertedInner.y - invertedInner.w;
        float innerBot = invertedInner.y + invertedInner.w;
        float innerLeft = invertedInner.x - invertedInner.z;
        float innerRight = invertedInner.x + invertedInner.z;
        float outerTop = invertedOuter.y - invertedOuter.w;
        float outerBot = invertedOuter.y + invertedOuter.w;
        float outerLeft = invertedOuter.x - invertedOuter.z;
        float outerRight = invertedOuter.x + invertedOuter.z;

        float sinkValue = 0.0;
        for (int i = 0; i < rectCount; i++) {
            vec4 rect = rectData[i * 5];
            vec4 sinkProps = rectData[i * 5 + 1];
            vec2 sinkSh = rectData[i * 5 + 3].xy;
            // 0 = sinks off for this rect (a melting popout must slide in
            // flush, not dig a pocket that snaps back at zero size)
            float sinkW = rectData[i * 5 + 3].z;
            if (sinkW <= 0.0)
                continue;

            // Screen-space center (with offset) and pre-computed AABB half-extents
            vec2 ctr = rect.xy + sinkProps.yz;

            // Sink onset / residual overlap: how far a rect must penetrate the border before
            // the inner wall recedes to form its pocket. Too low and the wall recedes faster
            // than the junction can stay convex, denting the inner edge inward near the rect's
            // (squared) corners; too high and the rect nestles too deep before the wall yields.
            // Tuned between the old cubic blend depth (k/6, too shallow) and the circular blend
            // depth ((sqrt2-1)k): half the circular smin gap-closing distance, (2-sqrt2)k/2.
            float preOff = smoothFactor * (2.0 - sqrt(2.0)) * 0.5;

            // Top border: track rect's BOTTOM edge, only within border thickness
            float topPen = clamp(innerTop - (ctr.y + sinkSh.y) - preOff, 0.0, innerTop - outerTop);

            // Bottom border: track rect's TOP edge
            float botPen = clamp((ctr.y - sinkSh.y) - innerBot - preOff, 0.0, outerBot - innerBot);

            // Left border: track rect's RIGHT edge
            float leftPen = clamp(innerLeft - (ctr.x + sinkSh.x) - preOff, 0.0, innerLeft - outerLeft);

            // Right border: track rect's LEFT edge
            float rightPen = clamp((ctr.x - sinkSh.x) - innerRight - preOff, 0.0, outerRight - innerRight);

            // Lateral distance from pixel to rect's extent along each edge
            float hLat = max(abs(pixel.x - ctr.x) - sinkSh.x, 0.0);
            float vLat = max(abs(pixel.y - ctr.y) - sinkSh.y, 0.0);

            // Perpendicular proximity: full strength in border, fade inside inner area
            float topZone = 1.0 - smoothstep(innerTop, innerTop + smoothFactor, pixel.y);
            float botZone = smoothstep(innerBot - smoothFactor, innerBot, pixel.y);
            float leftZone = 1.0 - smoothstep(innerLeft, innerLeft + smoothFactor, pixel.x);
            float rightZone = smoothstep(innerRight - smoothFactor, innerRight, pixel.x);

            float s = smoothFactor * 2.0;
            float sink = max(
                max(topPen * smoothstep(s, 0.0, hLat) * topZone,
                    botPen * smoothstep(s, 0.0, hLat) * botZone),
                max(leftPen * smoothstep(s, 0.0, vLat) * leftZone,
                    rightPen * smoothstep(s, 0.0, vLat) * rightZone)
            );
            sinkValue = max(sinkValue, sink);
        }

        dInner -= sinkValue;

        // The circular smax fillet has radius kFrame; when it exceeds the border thickness
        // it can't complete inside the border, so the sharp outer-box term bleeds onto the
        // inner edge and bulges the inner corners (worst when thickness < smoothFactor — the
        // default border is thinner than the blend radius). Clamp kFrame to the thinnest side
        // so the inner edge stays a clean constant-radius arc. Each inner corner is bounded by
        // its thinner adjacent side, so the global min is correct for every corner.
        float minThick = min(min(innerTop - outerTop, outerBot - innerBot),
                             min(innerLeft - outerLeft, outerRight - innerRight));
        float kFrame = clamp(min(smoothFactor, minThick - 1.0), 1.0, smoothFactor);
        float dFrame = smaxSharpA(dOuter, -dInner, kFrame);
        frameDist = dFrame;

        mergedSdf = smin(mergedSdf, dFrame, smoothFactor);
        if (dFrame < minDist) {
            owner = -1;
        }
    }

    float fw = fwidth(mergedSdf);
    float bodyAlpha = 1.0 - smoothstep(-fw, fw, mergedSdf);

    // Frame contact shadow: a soft falloff inward from the border, by distance
    // from the border itself, so it stays uniform along each edge. The pill and
    // popouts are opaque shapes drawn over it (the frame swelling open, not panels
    // on top), so they neither cast a halo nor cut the shadow off.
    float shadowAlpha = 0.0;
    if (shadowStrength > 0.0 && shadowSize > 0.0 && myIndex == -1 && frameDist > 0.0) {
        float t = clamp(1.0 - frameDist / shadowSize, 0.0, 1.0);
        shadowAlpha = shadowStrength * t * t;
    }

    // Owner gate: a renderer paints its own pixels plus the shared blend zone
    // (mergedSdf <= smoothFactor) to prevent seams; the shadow is added only by
    // the border renderer.
    bool ownPixel = (owner == myIndex) || (mergedSdf <= smoothFactor);
    if (!ownPixel && shadowAlpha <= 0.0)
        discard;

    float bodyA = ownPixel ? bodyAlpha : 0.0;
    // Body color over the (black) shadow, premultiplied.
    float outA = bodyA + shadowAlpha * (1.0 - bodyA);
    // 1-2px outline in borderColor along the silhouette rim (a band just inside sdf = 0)
    vec3 bodyRgb = (borderWidth > 0.0 && borderColor.a > 0.0)
        ? mix(color.rgb, borderColor.rgb, borderColor.a * smoothstep(-borderWidth - fw, -borderWidth + fw, mergedSdf))
        : color.rgb;
    if (waveColor.a > 0.0) {
        for (int w = 0; w < 4; ++w) {
            int rectIndex = int(waveInfo[w].w);
            if (rectIndex < 0 || rectIndex >= rectCount
                    || pixel.x < waveInfo[w].x || pixel.x > waveInfo[w].y)
                continue;
            float height = waveLift(pixel.x);
            float lift = smoothstep(0.75, 5.0, height);
            float waveD = dArr[rectIndex];
            if (lift > 0.0 && waveD < 1e9) {
                float bandBottom = rectData[rectIndex * 5].y
                    + rectData[rectIndex * 5 + 1].z + rectData[rectIndex * 5].w;
                float below = smoothstep(bandBottom, bandBottom + 2.0, pixel.y);
                float onWave = 1.0 - smoothstep(0.5, 2.0, waveD - mergedSdf);
                float trace = smoothstep(-1.5 - fw, -1.5 + fw, mergedSdf);
                if (waveStepped(w))
                    trace *= smoothstep(bandBottom + height - 3.0, bandBottom + height - 1.5, pixel.y);
                bodyRgb = mix(bodyRgb, waveColor.rgb, waveColor.a * lift * below * onWave * trace);
            }
        }
    }
    fragColor = vec4(bodyRgb * bodyA, outA) * qt_Opacity;
}

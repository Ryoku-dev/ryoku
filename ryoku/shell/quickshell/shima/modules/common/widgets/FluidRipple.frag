/*
 * Copyright (C) 2021 The Android Open Source Project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Ported to QtQuick/Quickshell by Gemini CLI.
 */

#version 440

layout(location = 0) in vec2 qt_TexCoord0;
layout(location = 0) out vec4 fragColor;

layout(std140, binding = 0) uniform buf {
    mat4 qt_Matrix;
    float qt_Opacity;
    float progress;
    float aspect;
    float sparkleIntensity;
    float glowIntensity;
    float ringWidth;
    vec2 center;
    vec4 color;
} ubuf;

// Improved noise for the "Sparkle" effect
float hash(vec2 p) {
    vec3 p3  = fract(vec3(p.xyx) * .1031);
    p3 += dot(p3, p3.yzx + 33.33);
    return fract((p3.x + p3.y) * p3.z);
}

void main() {
    vec2 uv = qt_TexCoord0;
    vec2 dvec = uv - ubuf.center;
    dvec.x *= ubuf.aspect;
    
    float d = length(dvec);
    float p = ubuf.progress;
    float rw = ubuf.ringWidth;

    // 1. Expansion mask (AOSP style) — ring width configurable
    float ring = smoothstep(p - rw, p, d) * (1.0 - smoothstep(p, p + rw, d));
    
    // 2. Sparkle/Noise layer — intensity configurable
    float noise = hash(uv * 250.0 + p * 0.1);
    float sparkles = pow(noise, 18.0) * ring * 5.0 * ubuf.sparkleIntensity;
    
    // 3. Soft Glow — intensity configurable
    float glow = exp(-pow(d - p, 2.0) * 60.0) * 0.4 * ubuf.glowIntensity;

    // Fade out as progress increases
    float alpha = (1.0 - p) * ubuf.qt_Opacity;
    
    // Combine components
    vec3 baseColor = ubuf.color.rgb;
    vec3 finalRGB = baseColor * (ring + sparkles + glow);
    
    fragColor = vec4(finalRGB, alpha * (ring + glow * 0.5 + sparkles * 0.2));
}

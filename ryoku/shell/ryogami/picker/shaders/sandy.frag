#version 440

layout(location = 0) in vec4 v_col;
layout(location = 1) in vec2 v_uv;
layout(location = 2) flat in float v_tex_mix;
layout(location = 3) flat in float v_li;
layout(location = 4) flat in float v_li2;
layout(location = 5) flat in float v_bmix;
layout(location = 6) flat in float v_vid;
layout(location = 7) flat in float v_vido;

layout(location = 0) out vec4 fragColor;

layout(std140, binding = 0) uniform buf {
    mat4 mvp;
    vec4 center_res;
    vec4 hero_dir_seed;
    vec4 prog_time_vis_carry;
    vec4 knobA;
    vec4 knobB;
    vec4 mix0;
    vec4 ring0;
    vec4 grid_style;
    vec4 video0;
    vec4 layerIdx;
    vec4 layerTier;
    vec4 uvRect[4];
    vec4 prevUv;
};

layout(binding = 1) uniform sampler2DArray nearTex;
layout(binding = 2) uniform sampler2DArray farTex;
layout(binding = 3) uniform sampler2D prevTex;
layout(binding = 4) uniform sampler2D prevOutTex;

vec3 sampleLayer(vec2 p, int which)
{
    vec4 r = uvRect[which];
    vec2 uv = r.xy + p * r.zw;
    if (layerTier[which] < 0.5)
        return textureLod(nearTex, vec3(uv, layerIdx[which]), 0.0).rgb;
    return textureLod(farTex, vec3(uv, layerIdx[which]), 0.0).rgb;
}

void main()
{
    vec3 rgb = v_col.rgb;
    if (v_tex_mix > 0.003) {
        vec3 t = sampleLayer(v_uv, int(v_li + 0.5));
        if (v_bmix < 0.997) {
            vec3 t2 = sampleLayer(v_uv, int(v_li2 + 0.5));
            t = mix(t2, t, v_bmix);
        }
        if (v_vid > 0.003) {
            vec3 vt = textureLod(prevTex, prevUv.xy + clamp(v_uv, vec2(0.0), vec2(1.0)) * prevUv.zw, 0.0).rgb;
            t = mix(t, vt, v_vid);
        }
        if (v_vido > 0.003) {
            vec3 vo = textureLod(prevOutTex, clamp(v_uv, vec2(0.0), vec2(1.0)), 0.0).rgb;
            t = mix(t, vo, v_vido);
        }
        rgb = mix(rgb, t, v_tex_mix);
    }
    fragColor = vec4(rgb * v_col.a, v_col.a);
}

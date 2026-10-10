#version 440

// Each grid cell is a six-vertex quad drawn twice: A outgoing, B incoming.

layout(std140, binding = 0) uniform buf {
    mat4 mvp;
    vec4 center_res;           // center.xy, resolution.xy
    vec4 hero_dir_seed;        // hero.xy, dir, seed
    vec4 prog_time_vis_carry;  // progress, time, vis, carry
    vec4 knobA;                // strands, twist, orbit, turbulence
    vec4 knobB;                // waist, front, fan, arc
    vec4 mix0;                 // bcut, bmix, swap_loop, swirl
    vec4 ring0;                // wave_flag, ring_spin, ring_wave, ring_soft
    vec4 grid_style;           // grid.x, grid.y, swap_style, ring_size
    vec4 video0;               // video_in, video_out, unused, unused
    vec4 layerIdx;             // array-layer index for A, B, B2, B3
    vec4 layerTier;            // tier per layer: 0 near, 1 far
    vec4 uvRect[4];            // per-layer sub-rect origin.xy, size.zw
    vec4 prevUv;               // live preview sub-rect origin.xy, size.zw
};

layout(binding = 1) uniform sampler2DArray nearTex;
layout(binding = 2) uniform sampler2DArray farTex;
layout(binding = 3) uniform sampler2D prevTex;
layout(binding = 4) uniform sampler2D prevOutTex;

layout(location = 0) out vec4 v_col;
layout(location = 1) out vec2 v_uv;
layout(location = 2) flat out float v_tex_mix;
layout(location = 3) flat out float v_li;
layout(location = 4) flat out float v_li2;
layout(location = 5) flat out float v_bmix;
layout(location = 6) flat out float v_vid;
layout(location = 7) flat out float v_vido;

// Layers resolve to the tier the CPU found the image in: near when resident, else the far tile.
vec3 sampleLayer(vec2 p, int which)
{
    vec4 r = uvRect[which];
    vec2 uv = r.xy + p * r.zw;
    if (layerTier[which] < 0.5)
        return textureLod(nearTex, vec3(uv, layerIdx[which]), 0.0).rgb;
    return textureLod(farTex, vec3(uv, layerIdx[which]), 0.0).rgb;
}

float hash11(float n)
{
    return fract(sin(n * 12.9898) * 43758.5453);
}

vec2 quad_off(uint corner)
{
    vec2 off = vec2(-0.5, -0.5);
    if (corner == 1u || corner == 3u)
        off = vec2(0.5, -0.5);
    if (corner == 2u || corner == 5u)
        off = vec2(-0.5, 0.5);
    if (corner == 4u)
        off = vec2(0.5, 0.5);
    return off;
}

void emit(vec2 p, vec2 sz, uint corner, vec3 col, float a, vec2 uvc, float tex_mix,
          float li, float li2, float bm, float vid, float vido)
{
    vec2 off = quad_off(corner);
    vec2 q = p + off * sz;
    gl_Position = mvp * vec4(q, 0.0, 1.0);
    v_col = vec4(col, clamp(a, 0.0, 1.0) * clamp(prog_time_vis_carry.z, 0.0, 1.0));
    v_uv = uvc + off * (vec2(1.15, 1.15) / grid_style.xy);
    v_tex_mix = tex_mix;
    v_li = li;
    v_li2 = li2;
    v_bmix = bm;
    v_vid = vid;
    v_vido = vido;
}

void main()
{
    vec2 center = center_res.xy;
    vec2 resolution = center_res.zw;
    vec2 hero = hero_dir_seed.xy;
    float dir = hero_dir_seed.z;
    float seed = hero_dir_seed.w;
    float progress = prog_time_vis_carry.x;
    float time = prog_time_vis_carry.y;
    float carry = prog_time_vis_carry.w;
    float strands = knobA.x;
    float twist = knobA.y;
    float orbit = knobA.z;
    float turbulence = knobA.w;
    float waist = knobB.x;
    float front = knobB.y;
    float fan = knobB.z;
    float arc = knobB.w;
    float bcut = mix0.x;
    float bmix = mix0.y;
    float swap_loop = mix0.z;
    float swirl = mix0.w;
    float wave_flag = ring0.x;
    float ring_spin = ring0.y;
    float ring_wave = ring0.z;
    float ring_soft = ring0.w;
    vec2 grid = grid_style.xy;
    float swap_style = grid_style.z;
    float ring_size = grid_style.w;
    float video_in = video0.x;
    float video_out = video0.y;

    uint vi = uint(gl_VertexIndex);
    uint pid = vi / 6u;
    uint corner = vi % 6u;
    uint gwu = uint(grid.x + 0.5);
    uint ng = gwu * uint(grid.y + 0.5);
    bool role_b = pid >= ng;
    uint g = pid % max(ng, 1u);
    uint gx = g % max(gwu, 1u);
    uint gy = g / max(gwu, 1u);
    vec2 uvp = (vec2(float(gx), float(gy)) + vec2(0.5, 0.5)) / grid;
    vec2 dc = uvp - vec2(0.5);
    float seedh = float(g) * 0.618034 + (role_b ? 31.7 : 0.0);
    float h = hash11(seedh);
    float h2 = hash11(seedh + 47.0);
    float h3 = hash11(seedh + 91.0);
    vec2 home = center + dc * 2.0 * hero;
    vec2 cell = 2.0 * hero / grid;

    uint style = uint(clamp(swap_style, 0.0, 17.0) + 0.5);
    float ang01 = fract(atan(dc.y, dc.x) * 0.15915494 + 1.0 + (h - 0.5) * 0.02);
    float strand_lin = clamp(uvp.y + (h - 0.5) * 0.07, 0.0, 0.999);
    float strand_base = strand_lin;
    if (style == 1u || style == 13u || style == 17u)
        strand_base = ang01;
    float strand = floor(strand_base * max(strands, 2.0));
    bool odd = (uint(strand) % 2u) == 1u;
    float pos_front = (dir > 0.0) ? uvp.x : (1.0 - uvp.x);
    float sh = hash11(strand * 7.31 + seed * 53.0);
    float sh2 = hash11(strand * 3.77 + seed * 29.0);
    float nd = mix(clamp(max(abs(dc.x), abs(dc.y)) * 2.0, 0.0, 1.0),
                   clamp(length(dc) * 1.4142136, 0.0, 1.0), 0.25);
    float rim = pow(nd, 1.5);
    vec2 sink = center + (vec2(h2, h3) - 0.5) * hero * 0.12;
    vec2 ring_axes = vec2(hero.x * 0.72, hero.y * 0.82) * clamp(ring_size, 0.25, 3.0);
    float ra17 = atan(dc.y, dc.x);
    float ang17 = ra17 + time * 1.8 * ring_spin * dir;
    float wob17 = (sin(ra17 * 3.0 + time * 1.7) * 0.07
        + sin(ra17 * 5.0 - time * 2.3 + hash11(float(g) * 0.618034 + 12.9) * 6.2831853) * 0.05)
        * ring_wave;
    float rr17 = 1.0 + wob17 + (hash11(float(g) * 0.618034 + 12.9) - 0.5) * 0.12;
    vec2 ring_a = center + vec2(cos(ang17), sin(ang17)) * ring_axes * rr17;
    vec2 ring_b = center + vec2(cos(ang17 - 0.9 * dir), sin(ang17 - 0.9 * dir)) * ring_axes * rr17;
    float start17 = (rim * 0.80 + h * 0.20) * 0.60;
    float raw17 = (progress - start17) / 0.40;
    float local17 = clamp(raw17, 0.0, 1.0);
    float gather17 = 0.30;
    float scatter17 = 0.62;
    float entry17 = ra17 + (h - 0.5) * 0.04;
    float spin17 = 6.2831853 * (0.55 + sh2 * 0.35) * dir;
    vec2 axes17 = ring_axes * rr17;
    vec2 ring_curve17 = home;
    float ramt17 = 0.0;
    float cell17 = 1.0;
    if (raw17 > 0.0 && raw17 < 1.0) {
        if (local17 < gather17) {
            float t17 = local17 / gather17;
            float e17 = t17 * t17 * (3.0 - 2.0 * t17);
            vec2 ringp17 = center + vec2(cos(entry17), sin(entry17)) * axes17;
            vec2 tangent17 = normalize(vec2(-sin(entry17) * axes17.x, cos(entry17) * axes17.y));
            vec2 waypoint17 = mix(home, ringp17, 0.55) + tangent17 * hero.y * 0.14 * dir;
            ring_curve17 = mix(mix(home, waypoint17, e17), mix(waypoint17, ringp17, e17), e17);
            ramt17 = sin(1.5707963 * e17);
            cell17 = 1.0 - smoothstep(0.03, 0.68, e17);
        } else if (local17 < scatter17) {
            float t17 = (local17 - gather17) / (scatter17 - gather17);
            float a17 = entry17 + spin17 * t17;
            float orbit_wobble17 = 1.0 + sin(t17 * 6.2831853 * (1.0 + sh * 2.0) + h * 6.2831853) * 0.05;
            ring_curve17 = center + vec2(cos(a17), sin(a17)) * axes17 * orbit_wobble17;
            ramt17 = 1.0;
            cell17 = 0.0;
        } else {
            float t17 = (local17 - scatter17) / (1.0 - scatter17);
            float e17 = t17 * t17 * (3.0 - 2.0 * t17);
            float exit_ang17 = entry17 + spin17;
            vec2 ringp17 = center + vec2(cos(exit_ang17), sin(exit_ang17)) * axes17;
            vec2 tangent17 = normalize(vec2(-sin(exit_ang17) * axes17.x, cos(exit_ang17) * axes17.y));
            vec2 waypoint17 = mix(ringp17, home, 0.45) + tangent17 * hero.y * 0.11 * dir;
            ring_curve17 = mix(mix(ringp17, waypoint17, e17), mix(waypoint17, home, e17), e17);
            ramt17 = sin(1.5707963 * (1.0 - e17));
            cell17 = smoothstep(0.70, 0.98, e17);
        }
        float activity17 = sin(3.14159265 * local17);
        ring_curve17 += vec2(sin(local17 * 9.0 + h * 6.2831853),
                             cos(local17 * 7.0 + h2 * 6.2831853)) * hero.y * 0.025 * activity17;
    }
    float dust17 = 1.0 - cell17;
    float grain17 = 1.15 + ramt17 * 1.25;
    float exit_x = (dir > 0.0) ? (-hero.x * 0.4) : (resolution.x + hero.x * 0.4);
    float entry_x = (dir > 0.0) ? (resolution.x + hero.x * 0.4) : (-hero.x * 0.4);
    vec2 neck = vec2(center.x + (h2 - 0.5) * hero.x * 0.06, center.y + hero.y);
    float hx = home.x + (h2 - 0.5) * hero.x * 0.25;
    float xn = (hx - center.x) / max(hero.x, 1.0);
    vec2 heap = vec2(hx, center.y + hero.y * (1.0 - exp(-xn * xn * 2.8) * (0.15 + h3 * 0.55)));
    float ground = center.y + hero.y * 0.92;
    float oa0 = h * 6.2831853;
    float osw = (2.0 + sh2 * 1.6) * dir;
    vec2 ring_ax = vec2(hero.x, hero.y * 0.88) * (1.15 + h3 * 0.40);
    vec2 orb_a1 = center + vec2(cos(oa0 + osw * 0.5), sin(oa0 + osw * 0.5)) * ring_ax;
    vec2 orb_a2 = center + vec2(cos(oa0 + osw), sin(oa0 + osw)) * ring_ax;
    float ob0 = h2 * 6.2831853;
    float obw = ob0 + (1.6 + sh * 1.4) * dir;
    vec2 ring_bx = vec2(hero.x, hero.y * 0.88) * (1.15 + h * 0.40);
    vec2 orb_b0 = center + vec2(cos(ob0), sin(ob0)) * ring_bx;
    vec2 orb_b1 = center + vec2(cos(obw), sin(obw)) * ring_bx;
    vec2 rv = dc + (vec2(h, h2) - 0.5) * 0.02;
    vec2 dcn = rv / max(length(rv), 1e-4);
    vec2 farp = center + dcn * length(resolution) * 0.62;
    float pa = (strand + 0.5) / max(strands, 2.0) * 6.2831853;
    vec2 bloom_w = center + vec2(cos(pa + 1.1 * dir), sin(pa + 1.1 * dir) * 0.85) * hero.y * 1.3;
    vec2 bloom_out = center + vec2(cos(pa + 2.2 * dir), sin(pa + 2.2 * dir) * 0.85) * length(resolution) * 0.55;
    vec2 bloom_bw = center + vec2(cos(pa - 1.1 * dir), sin(pa - 1.1 * dir) * 0.85) * hero.y * 1.3;
    vec2 bloom_b0 = center + vec2(cos(pa - 2.2 * dir), sin(pa - 2.2 * dir) * 0.85) * length(resolution) * 0.55;

    vec3 col;
    float tt;
    vec2 p0;
    vec2 p2;
    float fade;
    vec2 szf;
    float li = 0.0;
    float li2 = 0.0;
    float bm = 1.0;
    float extra_mid = 0.0;
    vec2 ring_pos = vec2(0.0, 0.0);
    float swg = 0.0;
    float vid = 0.0;
    float vido = 0.0;

    float sh3 = hash11(strand * 9.13 + seed * 71.0);
    float hc = hash11(float(g) * 0.618034 + 12.9);
    float bmixc = clamp(bmix, 0.0, 1.0);
    bool loop_style = swap_loop > 0.5;
    float wk = clamp(pos_front * 0.75 + hc * 0.25, 0.0, 0.999);
    float arch_t = clamp((bmixc * 1.111 - wk * 0.58) / 0.42, 0.0, 1.0);
    bool has_wave = wave_flag > 0.5;
    float cswitch = smoothstep(0.4, 0.6, arch_t);
    float cring = smoothstep(hc * 0.7, hc * 0.7 + 0.3, bmixc);
    float cmix = has_wave ? cswitch : cring;
    if (style == 17u && swirl < 0.001) {
        float w17 = clamp((progress - 0.30) / 0.50, 0.0, 1.0);
        cmix = smoothstep(hc * 0.55, hc * 0.55 + 0.45, w17);
    }
    float solid_kill = (progress >= 0.999) ? 0.0 : 1.0;

    if (!role_b) {
        col = sampleLayer(uvp, 0);
        float lum = dot(col, vec3(0.299, 0.587, 0.114));
        if (video_out > 0.001) {
            vec3 vcol = textureLod(prevOutTex, uvp, 0.0).rgb;
            col = mix(col, vcol, video_out);
        }
        vido = video_out;
        float ma = (1.35 + front) / 0.90;
        float dep = pos_front;
        vec2 fa = vec2(0.55, 0.95);
        p0 = home;
        p2 = vec2(exit_x, center.y + (sh - 0.5) * resolution.y * fan);
        if (style == 1u) {
            dep = rim;
            p2 = sink;
            fa = vec2(0.70, 0.98);
        } else if (style == 2u) {
            dep = uvp.y;
            p2 = neck;
            fa = vec2(0.72, 0.985);
        } else if (style == 3u) {
            dep = uvp.y;
            p2 = heap;
            fa = vec2(0.82, 0.98);
        } else if (style == 6u) {
            p2 = vec2(exit_x, ground + (sh - 0.5) * hero.y * 0.12);
        } else if (style == 7u) {
            dep = clamp(length((uvp - vec2(0.5, 1.0)) * vec2(1.0, 0.8)) * 0.95, 0.0, 1.0);
            p2 = vec2(center.x + (h2 - 0.5) * hero.x * 0.12, resolution.y + hero.y * 0.6);
            fa = vec2(0.75, 0.98);
        } else if (style == 8u) {
            dep = rim;
            p2 = orb_a2;
            fa = vec2(0.70, 0.97);
        } else if (style == 10u) {
            dep = rim;
            p2 = farp;
            fa = vec2(0.55, 0.92);
        } else if (style == 11u) {
            dep = odd ? (1.0 - pos_front) : pos_front;
            p2 = vec2(odd ? entry_x : exit_x, home.y + (sh - 0.5) * hero.y * 0.3);
        } else if (style == 13u) {
            dep = 1.0 - rim;
            p2 = bloom_out;
            fa = vec2(0.60, 0.95);
        } else if (style == 16u) {
            p2 = vec2(exit_x, center.y + (sh - 0.5) * resolution.y * 0.9);
        } else if (style == 17u) {
            dep = h3;
            p2 = ring_a;
            fa = vec2(0.75, 0.98);
        }
        float pw = 1.0;
        if (style == 2u || style == 3u || style == 7u || style == 10u || style == 13u || style == 17u)
            pw = 0.55;
        tt = clamp(progress / pw * ma - dep * front - h * 0.2 - lum * 0.15 + carry, 0.0, 1.0);
        float ease0 = tt * tt * (3.0 - 2.0 * tt);
        fade = (1.0 - smoothstep(fa.x, fa.y, tt)) * solid_kill;
        fade = fade * (1.0 - smoothstep(0.05, 0.55, swirl));
        szf = mix(cell * 1.15, vec2(2.0, 2.0), vec2(ease0, ease0));
        col = col * (1.0 + ease0 * 0.4);
        if (style == 17u) {
            fade = 0.0;
            szf = vec2(0.0, 0.0);
        }
    } else {
        vec3 cb_new = sampleLayer(uvp, 1);
        vec3 cb_old = sampleLayer(uvp, 2);
        vec3 cb_base = cb_old;
        if (bcut < 0.999) {
            vec3 cb_o3 = sampleLayer(uvp, 3);
            cb_base = mix(cb_o3, cb_old, smoothstep(hc * 0.7, hc * 0.7 + 0.3, bcut));
        }
        col = mix(cb_base, cb_new, cmix);
        if (video_in > 0.001) {
            vec3 vcol = textureLod(prevTex, uvp, 0.0).rgb;
            col = mix(col, vcol, video_in * cmix);
        }
        float lum = dot(cb_old, vec3(0.299, 0.587, 0.114));
        float mb = (1.32 + front) / 0.86;
        float dep = pos_front;
        vec2 ab = vec2(0.02, 0.18);
        p0 = vec2(entry_x, center.y + (sh - 0.5) * resolution.y * fan);
        p2 = home;
        if (style == 1u) {
            dep = rim;
            p0 = sink;
        } else if (style == 2u) {
            dep = 1.0 - uvp.y;
            p0 = neck + vec2(0.0, 4.0);
        } else if (style == 3u) {
            dep = 1.0 - uvp.y;
            p0 = heap;
            ab = vec2(0.03, 0.15);
        } else if (style == 6u) {
            p0 = vec2(entry_x, ground + (sh2 - 0.5) * hero.y * 0.12);
        } else if (style == 7u) {
            dep = 1.0 - uvp.y;
            p0 = vec2(center.x + (h3 - 0.5) * hero.x * 0.12, -hero.y * 0.6);
            ab = vec2(0.03, 0.16);
        } else if (style == 8u) {
            dep = rim;
            p0 = orb_b0;
            ab = vec2(0.05, 0.25);
        } else if (style == 10u) {
            dep = rim;
            p0 = farp;
            ab = vec2(0.03, 0.18);
        } else if (style == 11u) {
            dep = odd ? (1.0 - pos_front) : pos_front;
            p0 = vec2(odd ? exit_x : entry_x, home.y + (sh2 - 0.5) * hero.y * 0.3);
        } else if (style == 13u) {
            dep = rim;
            p0 = bloom_b0;
            ab = vec2(0.03, 0.20);
        } else if (style == 16u) {
            p0 = vec2(entry_x, center.y + (sh2 - 0.5) * resolution.y * 0.9);
        } else if (style == 17u) {
            dep = h2;
            p0 = ring_b;
        }
        float pb = progress;
        if (style == 2u || style == 3u || style == 7u || style == 10u || style == 13u || style == 17u)
            pb = (progress - 0.35) / 0.55;
        tt = clamp((pb - 0.04) * mb - dep * front - h * 0.2 - (1.0 - lum) * 0.12, 0.0, 1.0);
        if (style == 17u)
            tt = 1.0 - ramt17;
        float ease0 = tt * tt * (3.0 - 2.0 * tt);
        fade = smoothstep(ab.x, ab.y, tt) * solid_kill;
        szf = mix(vec2(2.0, 2.0), cell * 1.15, vec2(ease0, ease0));
        col = col * (1.0 + (1.0 - ease0) * 0.4);
        if (style == 17u) {
            fade = solid_kill;
            szf = mix(vec2(grain17, grain17), cell * 1.15, vec2(cell17, cell17));
            extra_mid = max(extra_mid, dust17);
        }
        li = 1.0;
        li2 = 2.0;
        bm = cmix;
        vid = video_in * cmix;
        if (swirl > 0.001) {
            swg = smoothstep(hc * 0.35, hc * 0.35 + 0.65, swirl);
            float ba = atan(dc.y, dc.x);
            float ang = ba + time * 1.8 * ring_spin * dir;
            float wob = (sin(ba * 3.0 + time * 1.7) * 0.07
                + sin(ba * 5.0 - time * 2.3 + hc * 6.2831853) * 0.05) * ring_wave;
            float rr = 1.0 + wob + (hc - 0.5) * 0.12;
            ring_pos = center + vec2(cos(ang), sin(ang)) * ring_axes * rr;
            fade = max(fade, swg * solid_kill / max(ring_soft, 0.34));
            extra_mid = max(extra_mid, swg);
        }
    }

    float ease = tt * tt * (3.0 - 2.0 * tt);
    float mid = sin(3.14159265 * ease);

    vec2 waistp;
    if (style == 1u) {
        vec2 rel = home - center;
        float rl = length(rel);
        float raa = atan(rel.y, rel.x);
        float spin = (0.5 + sh2 * 0.9) * dir * (0.4 + fan);
        waistp = center
            + vec2(cos(raa + spin), sin(raa + spin)) * rl * (0.22 + sh * 0.22) * (0.45 + waist * 0.4)
            + (vec2(h2, h3) - 0.5) * hero.y * 0.05;
    } else if (style == 2u) {
        float side = sign(dc.x + (h - 0.5) * 0.2);
        if (role_b) {
            waistp = center + vec2(side * hero.x * (0.7 + sh * 0.5), hero.y * (0.45 + sh2 * 0.35));
        } else {
            waistp = center + vec2(side * hero.x * (0.9 + sh * 0.5), (sh2 - 0.6) * hero.y * 0.5);
        }
        waistp += (vec2(h2, h3) - 0.5) * hero.y * 0.12 * waist;
    } else if (style == 3u) {
        float side = sign(dc.x + (h - 0.5) * 0.2);
        if (role_b) {
            waistp = vec2(mix(heap.x, home.x, 0.3) + side * hero.x * (0.15 + sh * 0.25),
                          center.y - hero.y * (0.75 + sh2 * 0.5));
        } else {
            waistp = vec2(home.x + side * hero.x * (0.5 + sh * 0.7),
                          home.y - hero.y * (0.45 + sh2 * 0.55));
        }
    } else if (style == 6u) {
        waistp = vec2(mix(p0.x, p2.x, 0.35 + sh2 * 0.3),
                      ground - hero.y * (0.05 + sh * 0.08) * waist);
    } else if (style == 7u) {
        if (role_b) {
            waistp = vec2(center.x + (sh2 - 0.5) * hero.x * 0.15, resolution.y * 0.08);
        } else {
            waistp = vec2(center.x + (sh - 0.5) * hero.x * 0.15, resolution.y * 0.94);
        }
    } else if (style == 8u) {
        if (role_b) {
            waistp = orb_b1;
        } else {
            waistp = orb_a1;
        }
        waistp += (vec2(h2, h3) - 0.5) * hero.y * 0.18 * waist;
    } else if (style == 10u) {
        vec2 tang = vec2(-dcn.y, dcn.x) * (sh - 0.5) * hero.y * 0.9 * dir;
        waistp = mix(p0, p2, 0.5) + tang;
    } else if (style == 11u) {
        waistp = vec2(mix(p0.x, p2.x, 0.3 + sh2 * 0.4),
                      home.y + (h2 - 0.5) * hero.y * 0.3 * waist);
    } else if (style == 13u) {
        if (role_b) {
            waistp = bloom_bw;
        } else {
            waistp = bloom_w;
        }
        waistp += (vec2(h2, h3) - 0.5) * hero.y * 0.10 * waist;
    } else if (style == 16u) {
        waistp = vec2(mix(p0.x, p2.x, 0.25 + sh2 * 0.5),
                      center.y + (sh - 0.5) * resolution.y * 0.45);
    } else if (style == 17u) {
        if (role_b) {
            waistp = mix(p0, p2, 0.45) + (vec2(h2, h3) - 0.5) * hero.y * 0.25 * waist;
        } else {
            waistp = mix(p0, p2, 0.55) + (vec2(h2, h3) - 0.5) * hero.y * 0.25 * waist;
        }
    } else {
        waistp = vec2(mix(p0.x, p2.x, 0.3 + sh2 * 0.4),
                      center.y + ((sh - 0.5) * hero.y * 0.3 + (h2 - 0.5) * hero.y * 0.05) * waist);
    }

    vec2 q0 = mix(p0, waistp, ease);
    vec2 q1 = mix(waistp, p2, ease);
    vec2 guide = mix(q0, q1, ease);
    if (style == 17u && role_b)
        guide = ring_curve17;

    float turb_mul = 1.0;
    float orb_mul = 1.0;
    if (style == 2u) { turb_mul = 1.4; orb_mul = 1.6; }
    if (style == 3u) { turb_mul = 1.3; orb_mul = 1.5; }
    if (style == 6u) { turb_mul = 0.4; orb_mul = 0.3; }
    if (style == 7u) { turb_mul = 1.2; orb_mul = 1.2; }
    if (style == 8u) { turb_mul = 1.2; orb_mul = 1.4; }
    if (style == 10u) { turb_mul = 1.2; orb_mul = 1.3; }
    if (style == 11u) { turb_mul = 1.25; orb_mul = 1.25; }
    if (style == 13u) { turb_mul = 1.2; orb_mul = 1.4; }
    if (style == 16u) { turb_mul = 1.6; orb_mul = 0.5; }
    if (style == 17u) { turb_mul = 0.7; orb_mul = 0.6; }
    guide += vec2(sin(ease * (4.0 + sh3 * 5.0) * 3.14159265 + h * 6.2831853 + time * 0.5),
                  cos(ease * (3.0 + sh2 * 4.0) * 3.14159265 + h2 * 6.2831853 + time * 0.42))
        * hero.y * (0.05 + h3 * 0.07) * mid * turbulence * turb_mul;

    float orb_r = (6.0 + h2 * 22.0) * orbit * orb_mul * mid;
    float orb_ang = ease * 6.2831853 * (1.5 + sh3 * 1.8) * twist
        + uvp.x * (7.0 + sh3 * 7.0) * twist
        + h2 * 1.3
        + time * 0.6;

    vec2 arch_off = vec2(0.0, 0.0);
    if (has_wave && loop_style && role_b) {
        float damp = 1.0 - mid * 0.6;
        float arch_r = hero.y * (0.22 + h2 * 0.22 + h3 * 0.10) * arc * damp;
        float theta = arch_t * 6.2831853;
        vec2 cu = normalize(vec2(dir * 0.85, -1.0));
        vec2 cw = vec2(cu.y, -cu.x) * dir;
        arch_off = (cw * sin(theta) + cu * (1.0 - cos(theta))) * arch_r;
    }
    vec2 style_off = vec2(0.0, 0.0);
    if (style == 6u) {
        style_off.y = -abs(sin(ease * 3.14159265 * (3.0 + sh3 * 3.0) + h * 3.0))
            * hero.y * (0.10 + h3 * 0.08) * mid;
    }
    if (style == 16u) {
        float lr = hero.y * (0.25 + sh2 * 0.25) * mid;
        float la = ease * 6.2831853 * (1.0 + sh3 * 1.2) * dir + h * 6.2831853;
        style_off = vec2(cos(la), sin(la)) * lr;
    }
    vec2 p = guide + vec2(cos(orb_ang), sin(orb_ang)) * orb_r + arch_off + style_off;
    if (swg > 0.001) {
        float ring_ang = atan(ring_pos.y - center.y, ring_pos.x - center.x);
        vec2 ring_tangent = normalize(vec2(-sin(ring_ang) * ring_axes.x, cos(ring_ang) * ring_axes.y));
        vec2 ring_waypoint = mix(p, ring_pos, 0.55)
            + ring_tangent * hero.y * 0.13 * clamp(ring_size, 0.25, 3.0) * dir;
        p = mix(mix(p, ring_waypoint, swg), mix(ring_waypoint, ring_pos, swg), swg);
        float depth_ring = sin(ring_ang) * 0.5 + 0.5;
        float gs = 2.2 * (1.0 + (max(ring_soft, 1.0) - 1.0) * 0.9) * mix(0.82, 1.18, depth_ring);
        szf = mix(szf, vec2(gs, gs), vec2(swg, swg));
        col = col * (1.0 + swg * mix(0.12, 0.42, depth_ring));
    }

    float corner_mid = (has_wave && loop_style && role_b) ? sin(3.14159265 * arch_t) : 0.0;
    szf = mix(szf, vec2(2.4, 2.4), vec2(corner_mid * 0.8));
    col = col * (1.0 + corner_mid * 0.35);
    float tex_mix = 1.0 - smoothstep(0.03, 0.3, max(mid, max(corner_mid * 0.9, extra_mid)));
    emit(p, szf, corner, col, fade, uvp, tex_mix, li, li2, bm, vid, vido);
}

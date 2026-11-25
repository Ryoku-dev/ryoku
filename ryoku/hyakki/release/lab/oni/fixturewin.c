/* SPDX-License-Identifier: GPL-3.0-only */
/* oni-fixture: one xdg-toplevel that shows a fixed picture, for the Oni lab.
 *
 *   oni-fixture APP_ID TITLE WIDTH HEIGHT PICTURE.xrgb
 *       PICTURE is WIDTH*HEIGHT XRGB8888 pixels (fixture.py build). The
 *       window asks for that size; when the compositor configures another,
 *       the picture sits at the top-left on its paper colour. Prints
 *       "ready WxH" once the first frame is committed and exits when the
 *       compositor closes the window.
 *   oni-fixture --globals
 *       prints every global the compositor advertises as one JSON object,
 *       {"globals": {"interface": version, ...}}, then exits: the census of
 *       capture protocols (ext-image-copy-capture, ext foreign toplevel
 *       sources, wlr screencopy) the lab records.
 *
 * Built on first use by window.sh with wayland-scanner from the system's
 * xdg-shell.xml. It reads no input and binds no seat. */
#define _GNU_SOURCE
#include <errno.h>
#include <fcntl.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mman.h>
#include <sys/stat.h>
#include <unistd.h>
#include <wayland-client.h>
#include "xdg-shell-client-protocol.h"

#define PAPER 0xFFF4F1EAu

static struct wl_compositor *compositor;
static struct wl_shm *shm;
static struct xdg_wm_base *wm_base;
static struct wl_surface *surface;
static struct xdg_surface *xsurface;
static struct xdg_toplevel *toplevel;
static struct wl_buffer *buffer;
static uint32_t *picture;
static int pic_w, pic_h, want_w, want_h, buf_w, buf_h;
static int running = 1, globals_only, globals_n, announced;

static void die(const char *what) {
    fprintf(stderr, "oni-fixture: %s: %s\n", what, strerror(errno));
    exit(1);
}

static void wm_ping(void *data, struct xdg_wm_base *base, uint32_t serial) {
    (void)data;
    xdg_wm_base_pong(base, serial);
}
static const struct xdg_wm_base_listener wm_listener = {.ping = wm_ping};

static void registry_global(void *data, struct wl_registry *reg, uint32_t name, const char *iface, uint32_t version) {
    (void)data;
    if (globals_only) {
        printf("%s\"%s\": %u", globals_n++ ? ", " : "", iface, version);
        return;
    }
    if (strcmp(iface, wl_compositor_interface.name) == 0)
        compositor = wl_registry_bind(reg, name, &wl_compositor_interface, version < 4 ? version : 4);
    else if (strcmp(iface, wl_shm_interface.name) == 0)
        shm = wl_registry_bind(reg, name, &wl_shm_interface, 1);
    else if (strcmp(iface, xdg_wm_base_interface.name) == 0) {
        /* version 1: no configure_bounds or wm_capabilities events to handle */
        wm_base = wl_registry_bind(reg, name, &xdg_wm_base_interface, 1);
        xdg_wm_base_add_listener(wm_base, &wm_listener, NULL);
    }
}
static void registry_remove(void *data, struct wl_registry *reg, uint32_t name) {
    (void)data, (void)reg, (void)name;
}
static const struct wl_registry_listener registry_listener = {registry_global, registry_remove};

static void buffer_release(void *data, struct wl_buffer *b) {
    (void)data, (void)b;
}
static const struct wl_buffer_listener buffer_listener = {buffer_release};

static void make_buffer(int w, int h) {
    size_t stride = (size_t)w * 4, size = stride * (size_t)h;
    int fd = memfd_create("oni-fixture", MFD_CLOEXEC);
    if (fd < 0)
        die("memfd_create");
    if (ftruncate(fd, (off_t)size) < 0)
        die("ftruncate");
    uint32_t *px = mmap(NULL, size, PROT_READ | PROT_WRITE, MAP_SHARED, fd, 0);
    if (px == MAP_FAILED)
        die("mmap");
    for (int y = 0; y < h; y++)
        for (int x = 0; x < w; x++)
            px[(size_t)y * w + x] = (x < pic_w && y < pic_h) ? picture[(size_t)y * pic_w + x] : PAPER;
    struct wl_shm_pool *pool = wl_shm_create_pool(shm, fd, (int32_t)size);
    if (buffer)
        wl_buffer_destroy(buffer);
    buffer = wl_shm_pool_create_buffer(pool, 0, w, h, (int32_t)stride, WL_SHM_FORMAT_XRGB8888);
    wl_buffer_add_listener(buffer, &buffer_listener, NULL);
    wl_shm_pool_destroy(pool);
    munmap(px, size);
    close(fd);
    buf_w = w, buf_h = h;
}

static void draw(void) {
    int w = want_w > 0 ? want_w : pic_w, h = want_h > 0 ? want_h : pic_h;
    if (!buffer || w != buf_w || h != buf_h)
        make_buffer(w, h);
    wl_surface_attach(surface, buffer, 0, 0);
    wl_surface_damage_buffer(surface, 0, 0, w, h);
    wl_surface_commit(surface);
    if (!announced) {
        printf("ready %dx%d\n", w, h);
        fflush(stdout);
        announced = 1;
    }
}

static void xsurface_configure(void *data, struct xdg_surface *xs, uint32_t serial) {
    (void)data;
    xdg_surface_ack_configure(xs, serial);
    draw();
}
static const struct xdg_surface_listener xsurface_listener = {xsurface_configure};

static void toplevel_configure(void *data, struct xdg_toplevel *t, int32_t w, int32_t h, struct wl_array *states) {
    (void)data, (void)t, (void)states;
    want_w = w, want_h = h;
}
static void toplevel_close(void *data, struct xdg_toplevel *t) {
    (void)data, (void)t;
    running = 0;
}
/* designated: bound at version 1, the compositor never sends the later events */
static const struct xdg_toplevel_listener toplevel_listener = {.configure = toplevel_configure, .close = toplevel_close};

static void load(const char *path) {
    int fd = open(path, O_RDONLY | O_CLOEXEC);
    if (fd < 0)
        die(path);
    struct stat st;
    if (fstat(fd, &st) < 0)
        die("fstat");
    size_t want = (size_t)pic_w * (size_t)pic_h * 4;
    if ((size_t)st.st_size != want) {
        fprintf(stderr, "oni-fixture: %s holds %lld bytes, not %dx%dx4\n", path, (long long)st.st_size, pic_w, pic_h);
        exit(1);
    }
    picture = malloc(want);
    if (!picture)
        die("malloc");
    for (size_t got = 0; got < want;) {
        ssize_t n = read(fd, (char *)picture + got, want - got);
        if (n <= 0)
            die("read");
        got += (size_t)n;
    }
    close(fd);
}

int main(int argc, char **argv) {
    globals_only = argc == 2 && strcmp(argv[1], "--globals") == 0;
    if (!globals_only && argc != 6) {
        fprintf(stderr, "usage: oni-fixture APP_ID TITLE WIDTH HEIGHT PICTURE.xrgb | oni-fixture --globals\n");
        return 2;
    }
    if (!globals_only) {
        pic_w = atoi(argv[3]), pic_h = atoi(argv[4]);
        if (pic_w <= 0 || pic_h <= 0 || pic_w > 4096 || pic_h > 4096) {
            fprintf(stderr, "oni-fixture: bad size %sx%s\n", argv[3], argv[4]);
            return 2;
        }
        load(argv[5]);
    }
    struct wl_display *display = wl_display_connect(NULL);
    if (!display)
        die("wl_display_connect");
    struct wl_registry *reg = wl_display_get_registry(display);
    if (globals_only)
        printf("{\"globals\": {");
    wl_registry_add_listener(reg, &registry_listener, NULL);
    wl_display_roundtrip(display);
    if (globals_only) {
        printf("}}\n");
        wl_display_disconnect(display);
        return 0;
    }
    if (!compositor || !shm || !wm_base) {
        fprintf(stderr, "oni-fixture: the compositor lacks wl_compositor, wl_shm or xdg_wm_base\n");
        return 1;
    }
    surface = wl_compositor_create_surface(compositor);
    xsurface = xdg_wm_base_get_xdg_surface(wm_base, surface);
    xdg_surface_add_listener(xsurface, &xsurface_listener, NULL);
    toplevel = xdg_surface_get_toplevel(xsurface);
    xdg_toplevel_add_listener(toplevel, &toplevel_listener, NULL);
    xdg_toplevel_set_app_id(toplevel, argv[1]);
    xdg_toplevel_set_title(toplevel, argv[2]);
    xdg_toplevel_set_min_size(toplevel, pic_w, pic_h);
    wl_surface_commit(surface);
    while (running && wl_display_dispatch(display) != -1)
        ;
    if (buffer)
        wl_buffer_destroy(buffer);
    xdg_toplevel_destroy(toplevel);
    xdg_surface_destroy(xsurface);
    wl_surface_destroy(surface);
    wl_display_disconnect(display);
    free(picture);
    return 0;
}

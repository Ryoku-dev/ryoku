//go:build linux

#include <wayland-client.h>
#include "proto_wlr_layer_shell_unstable_v1.h"
#include "proto_wlr_foreign_toplevel_management_unstable_v1.h"
#include "proto_xdg_output_unstable_v1.h"
#include "proto_viewporter.h"
#include "proto_fractional_scale_v1.h"
#include "_cgo_export.h"

const struct wl_registry_listener oni_registry_listener = {
	.global = (void (*)(void *, struct wl_registry *, uint32_t, const char *, uint32_t))oni_onGlobal,
	.global_remove = oni_onGlobalRemove,
};

const struct wl_output_listener oni_output_listener = {
	.geometry = (void (*)(void *, struct wl_output *, int32_t, int32_t, int32_t, int32_t, int32_t, const char *, const char *, int32_t))oni_onOutputGeometry,
	.mode = oni_onOutputMode,
	.done = oni_onOutputDone,
	.scale = oni_onOutputScale,
	.name = (void (*)(void *, struct wl_output *, const char *))oni_onOutputName,
	.description = (void (*)(void *, struct wl_output *, const char *))oni_onOutputDescription,
};

const struct zxdg_output_v1_listener oni_xdg_output_listener = {
	.logical_position = oni_onXdgOutputPosition,
	.logical_size = oni_onXdgOutputSize,
	.done = oni_onXdgOutputDone,
	.name = (void (*)(void *, struct zxdg_output_v1 *, const char *))oni_onXdgOutputName,
	.description = (void (*)(void *, struct zxdg_output_v1 *, const char *))oni_onXdgOutputDescription,
};

const struct wl_seat_listener oni_seat_listener = {
	.capabilities = oni_onSeatCapabilities,
	.name = (void (*)(void *, struct wl_seat *, const char *))oni_onSeatName,
};

const struct wl_pointer_listener oni_pointer_listener = {
	.enter = oni_onPointerEnter,
	.leave = oni_onPointerLeave,
	.motion = oni_onPointerMotion,
	.button = oni_onPointerButton,
	.axis = oni_onPointerAxis,
	.frame = oni_onPointerFrame,
	.axis_source = oni_onPointerAxisSource,
	.axis_stop = oni_onPointerAxisStop,
	.axis_discrete = oni_onPointerAxisDiscrete,
};

const struct wl_surface_listener oni_surface_listener = {
	.enter = oni_onSurfaceEnter,
	.leave = oni_onSurfaceLeave,
};

const struct wl_callback_listener oni_callback_listener = {
	.done = oni_onCallbackDone,
};

const struct wl_buffer_listener oni_buffer_listener = {
	.release = oni_onBufferRelease,
};

const struct zwlr_layer_surface_v1_listener oni_layer_surface_listener = {
	.configure = oni_onLayerConfigure,
	.closed = oni_onLayerClosed,
};

const struct wp_fractional_scale_v1_listener oni_fractional_scale_listener = {
	.preferred_scale = oni_onPreferredScale,
};

const struct zwlr_foreign_toplevel_manager_v1_listener oni_toplevel_manager_listener = {
	.toplevel = oni_onToplevel,
	.finished = oni_onToplevelManagerFinished,
};

const struct zwlr_foreign_toplevel_handle_v1_listener oni_toplevel_listener = {
	.title = (void (*)(void *, struct zwlr_foreign_toplevel_handle_v1 *, const char *))oni_onToplevelTitle,
	.app_id = (void (*)(void *, struct zwlr_foreign_toplevel_handle_v1 *, const char *))oni_onToplevelAppID,
	.output_enter = oni_onToplevelOutputEnter,
	.output_leave = oni_onToplevelOutputLeave,
	.state = oni_onToplevelState,
	.done = oni_onToplevelDone,
	.closed = oni_onToplevelClosed,
	.parent = oni_onToplevelParent,
};

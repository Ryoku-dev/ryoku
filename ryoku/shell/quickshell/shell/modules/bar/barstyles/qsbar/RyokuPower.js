.pragma library

// Backlight and battery shell commands, built on the standard Linux tools Ryoku
// ships (brightnessctl, upower). Device detection is /sys-anchored so
// Quickshell's working directory cannot affect the selected backlight.

// Prefer ryoku-hw-backlight -- the ground-truth selector the media keys and the
// OSD daemon also use, which names the backlight on the connected internal panel
// rather than a phantom nvidia_*/acpi_video device an Intel+dGPU laptop exposes
// (#221). Fall back to the /sys name list when the helper is off PATH (a dev
// checkout), so behaviour there is unchanged.
var backlightDeviceCmd = "BL=$(ryoku-hw-backlight 2>/dev/null); if [ -z \"$BL\" ]; then BL=$(ls -1 /sys/class/backlight 2>/dev/null | head -n1); for C in /sys/class/backlight/amdgpu_bl* /sys/class/backlight/intel_backlight /sys/class/backlight/acpi_video*; do [ -e \"$C\" ] && { BL=\"${C##*/}\"; break; }; done; fi; [ -n \"$BL\" ] || exit 0; "

var backlightDetectCmd = backlightDeviceCmd + "echo \"$BL\""

var brightnessPercentCmd = backlightDeviceCmd + "brightnessctl -d \"$BL\" -m 2>/dev/null | cut -d',' -f4 | tr -d '%' | awk '{print int($1)}'"

function shellQuote(value) {
    return "'" + String(value).replace(/'/g, "'\\''") + "'"
}

// Relative +5%/5%- steps ramp finely near the bottom of the range; absolute
// "<n>%" steps pass straight through. Single-flock so concurrent taps serialise.
function brightnessSetCmd(step) {
    var qStep = shellQuote(step)
    return "RUNTIME_DIR=${XDG_RUNTIME_DIR:-/tmp}; exec 9>\"$RUNTIME_DIR/ryoku-qsbar-brightness.lock\"; flock -n 9 || exit 0; " +
        backlightDeviceCmd +
        "STEP=" + qStep + "; " +
        "CURRENT=$(brightnessctl -d \"$BL\" -m 2>/dev/null | cut -d',' -f4 | tr -d '%' | awk '{print int($1)}'); " +
        "if [ \"$STEP\" = '+5%' ]; then if [ \"${CURRENT:-0}\" -lt 5 ]; then TARGET=$((CURRENT + 1)); else TARGET=$((CURRENT + 5)); fi; [ \"$TARGET\" -gt 100 ] && TARGET=100; STEP=\"$TARGET%\"; " +
        "elif [ \"$STEP\" = '5%-' ]; then if [ \"${CURRENT:-0}\" -le 5 ]; then TARGET=$((CURRENT - 1)); else TARGET=$((CURRENT - 5)); fi; [ \"$TARGET\" -lt 1 ] && TARGET=1; STEP=\"$TARGET%\"; fi; " +
        "brightnessctl -d \"$BL\" set \"$STEP\" >/dev/null 2>&1"
}

var batteryDataCmd =
    "BAT_PATH=$(upower -e 2>/dev/null | grep BAT | head -n1); [ -n \"$BAT_PATH\" ] || exit 0; " +
    "INFO=$(upower -i \"$BAT_PATH\" 2>/dev/null); [ -n \"$INFO\" ] || exit 0; " +
    "BAT=${BAT_PATH##*/}; BAT=${BAT#battery_}; " +
    "PCT=$(printf '%s\\n' \"$INFO\" | awk '/percentage/ { gsub(\"%\", \"\", $2); print int($2); exit }'); " +
    "STATE=$(printf '%s\\n' \"$INFO\" | awk '/state/ { print $2; exit }'); " +
    "RATE=$(printf '%s\\n' \"$INFO\" | awk '/energy-rate/ { rounded=sprintf(\"%.1f\", $2); sub(/\\.0$/, \"\", rounded); print rounded; exit }'); " +
    "SIZE=$(printf '%s\\n' \"$INFO\" | awk '/energy-full:/ { printf \"%d Wh\", $2; exit }'); " +
    "TIME=$(printf '%s\\n' \"$INFO\" | awk '/time to (empty|full)/ { value=$4; unit=$5; if (unit ~ /^minute/) printf \"%dm\", int(value); else { hours=int(value); minutes=int((value-hours)*60); if (minutes>0) printf \"%dh %dm\", hours, minutes; else printf \"%dh\", hours } exit }'); " +
    "LABEL=$(case \"$STATE\" in charging) echo 'Time to full';; *) echo 'Time left';; esac); " +
    "SYS=/sys/class/power_supply/$BAT; " +
    "FULL=$(cat \"$SYS/charge_full\" 2>/dev/null || cat \"$SYS/energy_full\" 2>/dev/null || echo 0); " +
    "DESIGN=$(cat \"$SYS/charge_full_design\" 2>/dev/null || cat \"$SYS/energy_full_design\" 2>/dev/null || echo 0); " +
    "HEALTH=$(awk -v f=\"$FULL\" -v d=\"$DESIGN\" 'BEGIN{ if(d>0){ h=f*100/d; if(h>100) h=100; printf \"%d%%\", h } }'); " +
    "CYC=$(cat \"$SYS/cycle_count\" 2>/dev/null || echo 0); " +
    "printf '%s|%s|%s|%s|%s|%s|%s|%s|%s\\n' \"$BAT\" \"$PCT\" \"$STATE\" \"$LABEL\" \"$TIME\" \"$RATE\" \"$SIZE\" \"$HEALTH\" \"$CYC\""

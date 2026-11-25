Hyakki starts nothing at install. Optional programs, each used only once
the feature that names it is turned on (from release/optional.json):

chromium: the browser Hands drives in its own profile; any Chromium-family browser on PATH also works
weston: the headless parent of the separate desktop Hands runs apps on
niri: the separate desktop Hands runs apps on, and the maximize quick command
grim: screenshots when Ryoku's capture tool is absent, and Hands looking at its separate desktop
wtype: types dictation, rewrites and Hands keys into the focused window
wl-clipboard: reads the highlighted text for a rewrite in place, and a pasted picture in the app
tesseract-ocr: reads words on screen for guides, kept walkthroughs and select-and-ask
tesseract-ocr-eng: reads words on screen for guides, kept walkthroughs and select-and-ask
espeak-ng: Oni's system voice
pipewire: hears the microphone and plays Oni's voice and sounds
pulseaudio-utils: the microphone list, the volume quick commands and the PulseAudio capture and playback fallback
uv: sets up Hindsight, the optional memory engine, when asked

On runit, "hyakki on" and "hyakki off" print the sv steps for the dormant
services in /usr/share/hyakki/runit; they change no service themselves.
Run "hyakki off" before removing the package.

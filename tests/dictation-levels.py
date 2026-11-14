#!/usr/bin/env python3
"""Speech variation must start quiet and stay visible on hot microphones."""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]

HARNESS = '''import QtQuick
import Quickshell
import shell.services
ShellRoot {
    Timer {
      interval: 50
      running: true
      onTriggered: {
        function feed(rms) { Dictation.apply({state: "recording", level: rms}); }
        function last() { return Dictation.levels[Dictation.levels.length - 1]; }
        function require(ok, reason) {
            if (!ok) throw new Error(reason);
        }
        try {
            Dictation.apply({state: "idle", enabled: false, notice: "off", attentionMs: 1});
            require(Dictation.showingOff, "a disabled shortcut needs its off notice");
            Dictation.apply({state: "idle", enabled: true, notice: "off", attentionMs: 2});
            require(!Dictation.showingOff, "turning voice typing on must dismiss the off notice");
            Dictation.apply({state: "idle"});
            for (let i = 0; i < 32; i++) feed(0.5);
            require(Math.max.apply(null, Dictation.levels) < 0.01,
                    "constant hot input must start at rest");
            require(Math.abs(Dictation.decibels + 6.0206) < 0.01,
                    "the dB reading must remain absolute");
            feed(0.05);
            feed(0.5);
            require(last() < 0.01, "one quiet buffer must not turn a steady input into a peak");
            for (const rms of [0.6, 0.75, 0.9]) feed(rms);
            require(last() > 0.75, "speech above a hot baseline needs visible movement");
            for (let i = 0; i < 16; i++) feed(0.5);
            require(last() < 0.01, "the trace must settle after speech stops");
            Dictation.apply({state: "idle"});
            feed(0.9);
            require(Math.max.apply(null, Dictation.levels) < 0.01,
                    "a new recording must not inherit the previous waveform");
            Dictation.apply({state: "idle"});
            for (const rms of [0.001, 0.01, 0.1, 0.5, 0.5, 0.5, 0.5, 0.5]) feed(rms);
            for (let i = 0; i < 16; i++) feed(0.5);
            require(Math.max.apply(null, Dictation.levels) < 0.01,
                    "capture startup ramp must not inflate a constant input");
            Dictation.apply({state: "idle"});
            for (let i = 0; i < 16; i++) feed(0.001);
            feed(0.01);
            require(last() > 0.75, "quiet microphones must retain speech sensitivity");
            for (let i = 0; i < 32; i++) feed(0);
            require(last() < 0.01, "digital silence must settle at rest");
            require(Dictation.levels.every(v => isFinite(v) && v >= 0 && v <= 1),
                    "the trace must stay inside its display range");
            console.log("PASS microphone response");
        } catch (error) {
            console.log("FAIL " + error.message);
        }
        Qt.quit();
      }
    }
}
'''


class MicrophoneResponse(unittest.TestCase):
    def test_quiet_speech_and_session_transitions(self):
        if not shutil.which('qs'):
            self.skipTest('needs qs')
        with tempfile.TemporaryDirectory(prefix='ryoku-meter-') as directory:
            home = Path(directory)
            runtime = home / 'run'
            runtime.mkdir(mode=0o700)
            scene = home / 'headroom.qml'
            scene.write_text(HARNESS)
            env = dict(os.environ, QT_QPA_PLATFORM='offscreen', HOME=str(home),
                       XDG_RUNTIME_DIR=str(runtime), XDG_CONFIG_HOME=str(home / 'config'),
                       XDG_DATA_HOME=str(home / 'data'), XDG_CACHE_HOME=str(home / 'cache'),
                       QML_IMPORT_PATH=str(ROOT / 'ryoku/shell/quickshell')
                       + ':' + os.environ.get('QML_IMPORT_PATH', ''))
            for key in ('__QUICKSHELL_CRASH_INFO_FD', '__QUICKSHELL_CRASH_DUMP_FD'):
                env.pop(key, None)
            result = subprocess.run(['qs', '-p', str(scene)], env=env,
                                    capture_output=True, text=True, timeout=30)
            output = result.stdout + result.stderr
            self.assertIn('PASS microphone response', output, output[-2000:])
            self.assertNotIn('FAIL ', output, output[-2000:])


if __name__ == '__main__':
    unittest.main()

import { resolve } from 'node:path';

// Integration check: needs Quickshell, ryoku-shell icons and an installed icon theme.
const root = resolve(import.meta.dir, '..');
for (let attempt = 1; attempt <= 3; attempt++) {
    const proc = Bun.spawn(['qs', '-p', resolve(import.meta.dir, 'fixtures/shell-icons'), '--no-color'], {
        env: {
            ...process.env,
            QT_QPA_PLATFORM: process.env.QT_QPA_PLATFORM || 'offscreen',
            RYOKU_SHELL_DIR: '',
            QML_IMPORT_PATH: [resolve(root, 'ryoku/shell/quickshell'), process.env.QML_IMPORT_PATH,
                resolve(process.env.HOME!, '.local/lib/qt6/qml')].filter(Boolean).join(':'),
        },
        stdout: 'pipe', stderr: 'pipe',
    });
    const timer = setTimeout(() => proc.kill(), 20000);
    const [stdout, stderr, code] = await Promise.all([
        new Response(proc.stdout).text(), new Response(proc.stderr).text(), proc.exited,
    ]);
    clearTimeout(timer);
    const log = stdout + stderr;
    if (code !== 0 || !log.includes('ICONS_SMOKE_PASS') || /decodeErrors=[1-9]/.test(log)) {
        console.error(log);
        throw new Error(`Icon runtime check failed on attempt ${attempt}`);
    }
    console.log(`Attempt ${attempt}: ${log.split('\n').find(line => line.includes('ICONS_SMOKE_PASS'))}`);
}

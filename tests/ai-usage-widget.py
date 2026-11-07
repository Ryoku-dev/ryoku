#!/usr/bin/env python3
"""Exercise the shipped QML expressions and compile real components in isolation."""
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
SHIPPED = ROOT / 'ryoku/shell/quickshell/inir/defaults/widgets/ai-usage'
WIDGET = SHIPPED if SHIPPED.exists() else ROOT / 'ai-usage'
IMPORTS = (ROOT / 'ryoku/shell/quickshell' if SHIPPED.exists() else
           Path(os.environ.get('RYOKU_QML_IMPORTS', Path.home() / '.config/quickshell')))


def function(source, name):
    start = source.index('    function ' + name + '(')
    end = source.index('\n    }', start) + len('\n    }')
    body = source[start:end]
    body = re.sub(r': (?:string|var|int|double|bool)(?=[,)])', '', body)
    return re.sub(r'\): (?:string|var|int|double|bool|void) \{', ') {', body)


def binding(source, name):
    match = re.search(r'^    readonly property \w+ ' + re.escape(name) + r': \{\n',
                      source, re.M)
    if not match:
        raise AssertionError('Missing binding: ' + name)
    end = source.index('\n    }', match.end())
    return source[match.end():end]


def node(script, timezone='Asia/Kolkata'):
    result = subprocess.run(['node', '-e', script], capture_output=True, text=True,
                            env=dict(os.environ, TZ=timezone), timeout=15)
    if result.returncode:
        raise AssertionError(result.stdout + result.stderr)


SETUP = '''
const assert = require('node:assert/strict');
const RealDate = Date;
let clock = new RealDate(2026, 10, 7, 12).getTime();
global.Date = class extends RealDate {
    constructor(...args) { super(...(args.length ? args : [clock])); }
    static now() { return clock; }
};
const Qt = {formatDate: (d,format) => {
    const value=[d.getFullYear(), String(d.getMonth()+1).padStart(2,'0'),
        String(d.getDate()).padStart(2,'0')].join('-');
    return format==='yyyy-MM' ? value.slice(0,7) : value;
}};
'''


class WidgetTest(unittest.TestCase):
    def test_trends_and_heatmap_fixed_calendar_windows(self):
        page = (WIDGET / 'IrisCompact.qml').read_text()
        activity = (WIDGET / 'components/AiUsageActivityCard.qml').read_text()
        script = SETUP + function(page, 'historyWeight') + '\n' + function(activity, 'dayWeight')
        script += '\nfunction trend() {\n' + binding(page, 'trend') + '\n}'
        script += '\nfunction heatmap() {\n' + binding(activity, 'map') + '\n}'
        script += '''
const usageModel = {days:[], hours:[]}, usage = usageModel;
const root = {hourly:false, periodDays:7, historyWeight};
const card = {dayWeight};
for (const date of [[2026,2,8],[2026,10,1],[2026,10,7]]) {
    clock = new RealDate(...date, 12).getTime();
    for (let weekday=0; weekday<7; weekday++) {
        clock += 86400000;
        const today = new Date();
        const yesterday = new Date(clock); yesterday.setDate(yesterday.getDate()-1);
        usage.days = [{date:Qt.formatDate(yesterday),tokens:2,cacheRead:3},
            {date:Qt.formatDate(today),tokens:7,cacheRead:11}];
        for (const count of [7,30,45]) {
            root.periodDays = count === 45 ? 0 : count;
            const result = trend();
            assert.equal(result.buckets.length,count);
            assert.equal(result.buckets.at(-1).date,Qt.formatDate(today));
            assert.equal(result.total,23);
            assert.equal(result.buckets.filter(x=>x.tokens===0).length,count-2);
        }
        const map = heatmap();
        assert.equal(map.weeks.length,9);
        assert(map.weeks.every(week=>week.length===7 && week.every(Boolean)));
        const cells=map.weeks.flat();
        assert.equal(new Set(cells.map(x=>x.date)).size,63);
        assert.equal(cells.at(-1).date,Qt.formatDate(today));
        const start=new Date(clock);start.setDate(start.getDate()-62);
        assert.equal(cells[0].date,Qt.formatDate(start));
        assert.equal(cells.filter(x=>x.value===0).length,61);
        assert.equal(map.active,2);
    }
}
usage.days=[];
assert.deepEqual(heatmap().weeks,[]);
'''
        for timezone in ('Asia/Kolkata', 'America/New_York'):
            node(script, timezone)

    def test_cache_hours_normalize_both_units(self):
        source = (WIDGET / 'UsageModel.qml').read_text()
        names = ('empty', 'pct', 'normalizeWindows', 'shortName', 'normalizePeriods', 'kmg', 'parse')
        script = SETUP + '\n'.join(function(source, name) for name in names)
        script += '\nconst root = {' + ','.join(names) + '}; const switchTimer={stop(){}};'
        script += '''
for (const start of [1794054600,1794054600000]) {
    root.parse(JSON.stringify({_hours:[{start,tokens:10,cacheRead:20}]}),clock/1000);
    assert.equal(root.hours[0].start,1794054600000);
    assert.equal(root.hours[0].tokens+root.hours[0].cacheRead,30);
}
'''
        node(script)

    def test_agent_fallback_uses_only_supported_available_tools(self):
        source = (WIDGET / 'IrisCompact.qml').read_text()
        script = SETUP + '\nfunction activeTool() {\n' + binding(source, 'activeTool') + '\n}'
        script += '''
const availability={ready:true};
const root={tool:'claude',hasAnyAgent:false,toolAvailable:false,agentOptions:[]};
for (const open of [false,true]) for (const codex of [false,true]) {
    root.agentOptions=[...(open?[{value:'opencode'}]:[]),...(codex?[{value:'codex'}]:[])];
    root.hasAnyAgent=root.agentOptions.length>0;
    for (const tool of ['claude','opencode','codex']) {
        root.tool=tool;
        root.toolAvailable=root.agentOptions.some(x=>x.value===tool);
        const expected=root.toolAvailable?tool:root.agentOptions[0]?.value??'opencode';
        assert.equal(activeTool(),expected);
    }
}
'''
        node(script)

    def test_tile_reads_its_model_and_actual_quota_window(self):
        source = (WIDGET / 'AiUsageTile.qml').read_text()
        expression = re.search(r'        StyledText \{\n            text: (.*?)\n            font \{',
                               source, re.S).group(1)
        script = SETUP + '\nfunction metric() { return (' + expression + '); }'
        script += '''
const root={};
const usage={has:false,hasPlan:false,windows:[],today:123,kmg:n=>String(n)};
assert.equal(metric(),'—');
usage.has=true;
assert.equal(metric(),'123 tokens today');
usage.hasPlan=true;
usage.windows=[{pct:48,label:'30d'}];
assert.equal(metric(),'48% · 30d');
usage.windows=[];
assert.equal(metric(),'123 tokens today');
'''
        node(script)

    def test_packaged_collector_lookup_preserves_environment_override(self):
        source = (WIDGET / 'UsageModel.qml').read_text()
        expression = re.search(
            r'    readonly property string collectorPath: ([\s\S]+?)\n    readonly property var collectorCommand:',
            source).group(1).strip()
        script = '''const assert=require('node:assert/strict');
let override='';
const Quickshell={env:name=>name==='RYOKU_AI_USAGE_COLLECTOR'?override:'/fixture/home'};
const collectorPath=()=>''' + expression + ''';
assert.equal(collectorPath(),'opencode-usage');
override='/fixture/custom-collector';
assert.equal(collectorPath(),override);
'''
        node(script)

    def test_estimated_spending_is_codex_only_and_checks_month_and_coverage(self):
        source = (WIDGET / 'IrisCompact.qml').read_text()
        script = SETUP + '\nfunction monthly() {\n' + binding(source, 'monthly') + '\n}'
        script += '\nfunction spendHelp() {\n' + binding(source, 'spendHelp') + '\n}'
        script += '''
const root={activeTool:'codex',monthLabel:()=>'Nov'};
const usageModel={plan:'plus',days:[{date:'2026-11-07',cost:99}],spending:{
    kind:'api-equivalent',month:'2026-11',amount:1.23,partial:true,price_date:'2026-11-07'}};
root.monthly=monthly();
assert.equal(root.monthly.current,1.23);
assert.equal(root.monthly.available,true);
assert.equal(root.monthly.partial,true);
assert(spendHelp().includes('Not a bill'));
assert(spendHelp().includes('Plus'));
usageModel.spending.amount=null;
assert.equal(monthly().available,false);
usageModel.spending.amount=NaN;
assert.equal(monthly().available,false);
usageModel.spending.amount=1.23;
usageModel.spending.month='2026-10';
assert.equal(monthly().available,false);
root.activeTool='opencode';
assert.equal(monthly().current,99);
assert.equal(monthly().available,undefined);
'''
        node(script)

    @unittest.skipUnless(shutil.which('qs') and (IMPORTS / 'inir').exists(),
                         'Quickshell and Ryoku imports required for component compilation')
    def test_real_qml_components_compile_without_starting_services(self):
        with tempfile.TemporaryDirectory(prefix='ai-usage-qml-') as directory:
            work = Path(directory)
            for name in ('home', 'config', 'cache', 'runtime'):
                (work / name).mkdir(mode=0o700)
            for name in ('inir', 'shell', 'stage'):
                if (IMPORTS / name).exists():
                    (work / name).symlink_to(IMPORTS / name, target_is_directory=True)
            paths = [str(IMPORTS), str(Path.home() / '.local/lib/qt6/qml')]
            if SHIPPED.exists():
                (work / 'Ryoku').mkdir()
                (work / 'Ryoku/Ui').symlink_to(ROOT / 'ryoku/ui', target_is_directory=True)
            targets = list(WIDGET.rglob('*.qml'))
            platform = os.environ.get('AI_USAGE_QML_PLATFORM', 'offscreen')
            if SHIPPED.exists():
                targets.append(ROOT / 'ryoku/shell/quickshell/inir/services/CustomWidgets.qml')
                if platform == 'wayland':
                    targets.append(ROOT / 'ryoku/shell/quickshell/inir/modules/background/widgets/WidgetManagerPanel.qml')
            uris = json.dumps([path.as_uri() for path in targets])
            (work / 'shell.qml').write_text('''import QtQuick
import Quickshell
ShellRoot {
 Timer { interval: 50; running: true; onTriggered: {
    let failures=0;
    for (const path of ''' + uris + ''') {
        const component=Qt.createComponent(path, Component.PreferSynchronous);
        if (component.status!==Component.Ready) {
            console.log('FAIL:',path,component.errorString()); failures++;
        }
    }
    console.log(failures ? 'AI-USAGE-COMPILE-FAIL' : 'AI-USAGE-COMPILE-PASS');
    Qt.quit();
 } }
}''')
            env = dict(os.environ, HOME=str(work / 'home'), XDG_CONFIG_HOME=str(work / 'config'),
                       XDG_CACHE_HOME=str(work / 'cache'), XDG_STATE_HOME=str(work / 'state'),
                       XDG_RUNTIME_DIR=str(work / 'runtime'), QT_QPA_PLATFORM=platform,
                       QML_IMPORT_PATH=os.pathsep.join([str(work)] + paths),
                       QML2_IMPORT_PATH=os.pathsep.join([str(work)] + paths),
                       QS_DISABLE_CRASH_HANDLER='1')
            if platform == 'wayland':
                display = Path(os.environ.get('WAYLAND_DISPLAY', 'wayland-0'))
                if not display.is_absolute():
                    display = Path(os.environ['XDG_RUNTIME_DIR']) / display
                env['WAYLAND_DISPLAY'] = str(display)
            result = subprocess.run(['qs', '-p', str(work / 'shell.qml')], env=env,
                                    capture_output=True, text=True, timeout=20)
            output = result.stdout + result.stderr
            self.assertEqual(result.returncode, 0, output)
            self.assertIn('AI-USAGE-COMPILE-PASS', output)
            self.assertNotIn('FAIL:', output)


if __name__ == '__main__':
    unittest.main(verbosity=2)

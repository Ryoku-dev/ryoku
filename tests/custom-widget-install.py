#!/usr/bin/env python3
"""Check shared widget installation and the unchanged shell cache readers."""
import importlib.util
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
LOADER = ROOT / 'ryoku/shell/quickshell/inir/services/CustomWidgets.qml'


def functions(source, names):
    result = []
    for name in names:
        start = source.index('    function ' + name + '(')
        end = source.index('\n    }', start) + len('\n    }')
        body = re.sub(r': (?:string|var|bool)(?=[,)])', '', source[start:end])
        result.append(re.sub(r'\): (?:void|bool|var) \{', ') {', body))
    return '\n'.join(result)


def load_test(name):
    spec = importlib.util.spec_from_file_location(name, ROOT / 'tests' / (name + '.py'))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class SharedWidgetTest(unittest.TestCase):
    def run_node(self, script, *args):
        result = subprocess.run(['node', '-e', script, *map(str, args)],
                                capture_output=True, text=True, timeout=15)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        return result.stdout

    def module_catalog_script(self):
        loader = LOADER.read_text()
        options = (ROOT / 'ryoku/shell/quickshell/inir/modules/iris/settings/IrisOptions.qml').read_text()
        host = (ROOT / 'ryoku/shell/quickshell/inir/modules/iris/bar/IrisCustomModule.qml').read_text()
        script = functions(loader, ('_parseShipped', 'isInstalled'))
        script += '\n' + functions(options, ('choicesOf',))
        script += '\nconst assert=require("node:assert/strict");'
        script += '\nconst root={ready:true,widgets:[],shippedWidgets:[],widgetId:"ai-usage"};'
        script += '\nconst CustomWidgets=root, IrisStyle={ryokuFrontend:false};'
        if 'readonly property var irisModules:' in loader:
            binding = load_test('ai-usage-widget').binding(loader, 'irisModules')
            script += '\nObject.defineProperty(root,"irisModules",{get:()=>{' + binding + '\n}});'
        row = next(line.strip().rstrip(',') for line in options.splitlines()
                   if 'path: "iris.bar.rightModules"' in line)
        script += '\nconst spec=' + row + ';'
        expression = re.search(r'readonly property var entry: ([\s\S]+?)\n    readonly property string sourcePath:', host).group(1)
        script += '\nconst selected=()=>(' + expression + ');'
        scan = subprocess.run(['bash', str(ROOT / 'ryoku/shell/quickshell/inir/scripts/scan-widgets.sh'),
                               str(ROOT / 'ryoku/shell/quickshell/inir/defaults/widgets')],
                              capture_output=True, text=True, check=True)
        script += '\n_parseShipped(' + json.dumps(scan.stdout) + ');'
        return script

    def test_bundled_module_is_offered_and_loadable_on_a_fresh_profile(self):
        script = self.module_catalog_script() + '''
assert.deepEqual(root.widgets,[]);
assert.equal(isInstalled('ai-usage'),false);
assert.equal(spec.showIf(),true);
assert.deepEqual(spec.fallback,[]);
assert.deepEqual(choicesOf(spec).map(choice=>choice.value),['custom:ai-usage']);
const entry=selected();
assert.equal(entry.id,'ai-usage');
assert(entry.irisQmlPath.includes('/defaults/widgets/ai-usage/IrisCompact.qml'));
assert(require('node:fs').existsSync(new URL(entry.irisQmlPath)));
'''
        self.run_node(script)

    def test_installed_modules_override_bundled_defaults_without_duplicates(self):
        script = self.module_catalog_script() + '''
const custom={id:'ai-usage',name:'My AI usage',irisQmlPath:'file:///custom/AI.qml'};
const other={id:'other',name:'Other',irisQmlPath:'file:///custom/Other.qml'};
root.widgets=[custom,other,{id:'desktop-only',irisQmlPath:''}];
assert.deepEqual(choicesOf(spec).map(choice=>choice.value),['custom:ai-usage','custom:other']);
assert.equal(selected(),custom);
assert.equal(isInstalled('ai-usage'),true);
root.widgets=[{id:'ai-usage',irisQmlPath:''}];
assert.deepEqual(choicesOf(spec),[]);
assert.equal(selected(),null);
root.widgets=[];
assert.equal(selected().id,'ai-usage');
assert.equal(isInstalled('ai-usage'),false);
'''
        self.run_node(script)

    @unittest.skipUnless(shutil.which('qs'), 'Quickshell required for typed-list runtime check')
    def test_bundled_catalog_reacts_to_installed_copies_in_qml(self):
        source = LOADER.read_text()
        binding = load_test('ai-usage-widget').binding(source, 'irisModules')
        manifest = json.loads((ROOT / 'ryoku/shell/quickshell/inir/defaults/widgets/ai-usage/widget.json').read_text())
        entries = json.dumps([{'id': 'ai-usage', 'dir': '/bundled/ai-usage', 'manifest': manifest}])
        qml = '''import QtQuick
import Quickshell
ShellRoot {
    id: root
    property list<var> widgets: []
    property list<var> shippedWidgets: []
    readonly property var irisModules: {''' + binding + '''
    }
''' + functions(source, ('_parseShipped',)) + '''
    Component.onCompleted: _parseShipped(''' + json.dumps(entries) + ''')
    Timer {
        interval: 30; running: true; repeat: true
        property int phase: 0
        onTriggered: {
            const expected = phase === 1 ? "file:///custom/AI.qml" : "file:///bundled/ai-usage/IrisCompact.qml";
            if (root.irisModules.length !== 1 || root.irisModules[0].irisQmlPath !== expected) {
                console.log("BUNDLED-MODULE-RUNTIME-FAIL", phase, JSON.stringify(root.irisModules));
                Qt.quit(); return;
            }
            if (phase === 0) root.widgets = [{id:"ai-usage",irisQmlPath:"file:///custom/AI.qml"}];
            else if (phase === 1) root.widgets = [];
            else { console.log("BUNDLED-MODULE-RUNTIME-PASS"); Qt.quit(); }
            phase++;
        }
    }
}'''
        with tempfile.TemporaryDirectory(prefix='bundled-module-qml-') as directory:
            work = Path(directory)
            runtime = work / 'runtime'
            runtime.mkdir(mode=0o700)
            (work / 'shell.qml').write_text(qml)
            env = dict(os.environ, HOME=str(work / 'home'), XDG_CONFIG_HOME=str(work / 'config'),
                       XDG_CACHE_HOME=str(work / 'cache'), XDG_STATE_HOME=str(work / 'state'),
                       XDG_RUNTIME_DIR=str(runtime), QT_QPA_PLATFORM='offscreen', QS_DISABLE_CRASH_HANDLER='1')
            result = subprocess.run(['qs', '-p', str(work / 'shell.qml')], env=env,
                                    capture_output=True, text=True, timeout=10)
            output = result.stdout + result.stderr
            self.assertEqual(result.returncode, 0, output)
            self.assertIn('BUNDLED-MODULE-RUNTIME-PASS', output)
            self.assertNotIn('BUNDLED-MODULE-RUNTIME-FAIL', output)

    def test_original_example_api_and_install_guard(self):
        script = functions(LOADER.read_text(), ('installShipped', 'installExample'))
        script += '''
const assert=require('node:assert/strict');
const root={installShipped};
const _installShippedProcess={widgetId:'unchanged',running:false};
installExample();
assert.equal(_installShippedProcess.widgetId,'example-widget');
assert.equal(_installShippedProcess.running,true);
installShipped('ai-usage');
assert.equal(_installShippedProcess.widgetId,'example-widget');
for(const id of ['', '../other', 'x/y', '$(touch injected)', 'x";exit 1', null]) {
    _installShippedProcess.running=false;
    installShipped(id);
    assert.equal(_installShippedProcess.running,false);
    assert.equal(_installShippedProcess.widgetId,'example-widget');
}
installShipped('ai-usage');
assert.equal(_installShippedProcess.widgetId,'ai-usage');
assert.equal(_installShippedProcess.running,true);
'''
        self.run_node(script)

    def install_command(self, source, target):
        text = LOADER.read_text()
        start = text.index('        command: ', text.index('id: _installShippedProcess'))
        end = text.index('\n        stdout:', start)
        expression = text[start:end].strip().removeprefix('command: ')
        script = '''const root={shippedWidgetsDir:process.argv[1],widgetsDir:process.argv[2]};
const _installShippedProcess={widgetId:'ai-usage'};
process.stdout.write(JSON.stringify(''' + expression + '));'
        return json.loads(self.run_node(script, source, target))

    def test_install_handles_literal_paths_and_preserves_other_widgets(self):
        with tempfile.TemporaryDirectory(prefix='widget-install-') as directory:
            work = Path(directory)
            source = work / 'defaults " $(touch injected)'
            target = work / "user's widgets"
            (source / 'ai-usage').mkdir(parents=True)
            (source / 'ai-usage/widget.json').write_text('{}')
            (source / 'ai-usage/.hidden').write_text('asset')
            (target / 'other-widget').mkdir(parents=True)
            (target / 'other-widget/keep').write_text('user content')
            command = self.install_command(source, target)
            first = subprocess.run(command, cwd=work, capture_output=True, text=True)
            self.assertEqual(first.returncode, 0, first.stderr)
            self.assertIn('done', first.stdout)
            self.assertEqual((target / 'ai-usage/.hidden').read_text(), 'asset')
            (target / 'ai-usage/widget.json').write_text('user edit')
            second = subprocess.run(command, cwd=work, capture_output=True, text=True)
            self.assertEqual(second.returncode, 0, second.stderr)
            self.assertIn('exists', second.stdout)
            self.assertEqual((target / 'ai-usage/widget.json').read_text(), 'user edit')
            self.assertEqual((target / 'other-widget/keep').read_text(), 'user content')
            self.assertFalse((work / 'injected').exists())
            self.assertEqual(list(target.glob('.install-*')), [])

    def test_failed_copy_does_not_publish_a_partial_widget(self):
        with tempfile.TemporaryDirectory(prefix='widget-install-fail-') as directory:
            work = Path(directory)
            source, target, tools = work / 'defaults', work / 'widgets', work / 'bin'
            (source / 'ai-usage').mkdir(parents=True)
            tools.mkdir()
            (tools / 'cp').write_text('#!/bin/sh\nexit 9\n')
            (tools / 'cp').chmod(0o755)
            result = subprocess.run(self.install_command(source, target), capture_output=True,
                                    env=dict(os.environ, PATH=str(tools) + os.pathsep + os.environ['PATH']))
            self.assertNotEqual(result.returncode, 0)
            self.assertFalse((target / 'ai-usage').exists())
            self.assertEqual(list(target.glob('.install-*')), [])

    def test_existing_dangling_symlink_is_preserved(self):
        with tempfile.TemporaryDirectory(prefix='widget-install-link-') as directory:
            work = Path(directory)
            source, target = work / 'defaults', work / 'widgets'
            (source / 'ai-usage').mkdir(parents=True)
            target.mkdir()
            (target / 'ai-usage').symlink_to(work / 'missing')
            result = subprocess.run(self.install_command(source, target), capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertTrue((target / 'ai-usage').is_symlink())

    def test_collector_output_remains_readable_by_other_shell_widgets(self):
        fixture = load_test('codex-usage-regression').CodexUsageTest()
        fixture.setUp()
        self.addCleanup(fixture.doCleanups)
        plus = fixture.collect()
        fixture.set_limits('free', 43200)
        free = fixture.collect()
        contract = load_test('ai-usage-cache')
        self.assertEqual(contract.main(), 0)
        opencode = json.loads(contract.CACHE.read_text())
        source = (ROOT / 'ryoku/shell/quickshell/shell/services/AiUsage.qml').read_text()
        names = ('emptyTool', 'percent', 'windowLabel', 'windowFromCache', 'windowsFromCache',
                 'legacyWindows', 'codexBuckets', 'generalBucket', 'usageText', 'rateText',
                 'parseLegacy', 'parseCodex')
        script = functions(source, names) + '\nconst root={' + ','.join(names) + '};'
        script += "\nconst I18n={tr:text=>text}; const assert=require('node:assert/strict');"
        script += '\nconst data=' + json.dumps([plus, free, opencode]) + ';'
        script += '''
for(const [index,minutes] of [[0,[300,10080]],[1,[43200]]]) {
    const result=root.parseCodex(data[index],true);
    assert.equal(result.has,true);
    assert.equal(result.fresh,true);
    assert.equal(result.today,data[index]._today_tokens);
    assert.deepEqual(result.windows.map(x=>x.minutes),minutes);
    assert.equal(result.pct5h,index===0?25:0);
}
const result=root.parseLegacy('opencode',data[2],true);
assert.equal(result.has,true);
assert.equal(result.today,data[2]._today_tokens);
assert(result.models.length>0);
assert.equal(result.models[0].name,data[2]._models[0].name);
'''
        self.run_node(script)


if __name__ == '__main__':
    unittest.main(verbosity=2)

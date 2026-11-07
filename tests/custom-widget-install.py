#!/usr/bin/env python3
"""Check shared widget installation and the unchanged shell cache readers."""
import importlib.util
import json
import os
from pathlib import Path
import re
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
        result.append(re.sub(r'\): (?:void|bool) \{', ') {', body))
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

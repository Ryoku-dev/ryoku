#!/usr/bin/env python3
"""Regression cases for Codex rollout replay and account quota windows."""
import contextlib
from datetime import datetime
import io
import json
import os
from pathlib import Path
import re
import runpy
import signal
import subprocess
import sys
import tempfile
import time
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1]
COLLECTOR = Path(os.environ.get('CODEX_USAGE_TEST_COLLECTOR', ROOT / 'ryoku/rashin/usage/codex-usage'))


class CodexUsageTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='codex-usage-test-')
        self.addCleanup(self.temp.cleanup)
        self.home = Path(self.temp.name)
        self.sessions = self.home / 'codex/sessions'
        self.archive = self.home / 'codex/archived_sessions'
        self.sessions.mkdir(parents=True)
        self.archive.mkdir()
        self.path = self.sessions / 'rollout-test.jsonl'
        self.cache = self.home / 'usage.json'
        self.activity = self.home / 'activity.json'
        self.now = int(time.time())
        self.env = dict(os.environ, HOME=str(self.home), CODEX_HOME=str(self.sessions.parent),
                        CODEX_USAGE_DISABLE_RPC='1', CODEX_USAGE_NOW=str(self.now),
                        CODEX_USAGE_CACHE_FILE=str(self.cache),
                        CODEX_USAGE_ACTIVITY_FILE=str(self.activity),
                        CODEX_USAGE_STATE_DIR=str(self.home / 'spending'),
                        CODEX_USAGE_DISABLE_PRICES='1')
        self.set_limits('plus', 300, 10080)
        self.records = [self.meta('one'), self.event(1000, 200, 300, -120),
                        self.event(3000, 900, 1200, -60)]
        self.write(self.path, self.records)

    def stamp(self, offset):
        return time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime(self.now + offset))

    def meta(self, sid):
        return {'timestamp': self.stamp(-180), 'type': 'session_meta',
                'payload': {'id': sid, 'timestamp': self.stamp(-180), 'cwd': '/test'}}

    def event(self, total, cached, output, offset):
        return {'timestamp': self.stamp(offset), 'type': 'event_msg',
                'payload': {'type': 'token_count', 'info': {'total_token_usage': {
                    'input_tokens': total - output, 'output_tokens': output,
                    'cached_input_tokens': cached, 'total_tokens': total}}}}

    def write(self, path, records):
        path.write_text(''.join(json.dumps(row) + '\n' for row in records))

    def set_limits(self, plan, primary, secondary=None):
        def window(minutes):
            return {'usedPercent': 25, 'windowDurationMins': minutes,
                    'resetsAt': self.now + minutes * 60} if minutes else None
        self.env['CODEX_USAGE_RATE_LIMITS_JSON'] = json.dumps({
            'source': 'rpc', 'rateLimits': {'limitId': 'codex', 'planType': plan,
                                           'primary': window(primary), 'secondary': window(secondary)}})

    def collect(self):
        subprocess.run(['python3', str(COLLECTOR)], env=self.env, check=True,
                       capture_output=True, text=True, timeout=10)
        return json.loads(self.cache.read_text())

    def assert_totals(self, cache, inclusive=3000, uncached=2100):
        self.assertEqual(cache['_today_tokens'], inclusive)
        self.assertEqual(cache['_rate_per_hour'], inclusive)
        self.assertEqual(sum(x['tokens'] for x in cache['_recent_days']), inclusive)
        self.assertEqual(cache['_periods']['all']['tokens'], uncached)
        self.assertEqual(sum(x['tokens'] for x in cache['_sessions']), uncached)
        self.assertEqual(sum(x['tokens'] + x['cacheRead'] for x in cache['_days']), inclusive)
        self.assertEqual(sum(x['tokens'] + x['cacheRead'] for x in cache['_hours']), inclusive)

    def test_repeat_snapshot_and_unchanged_refresh(self):
        self.write(self.path, self.records + [self.event(3000, 900, 1200, -50)])
        self.assert_totals(self.collect())
        self.assert_totals(self.collect())

    def test_archive_move_and_new_append(self):
        self.assert_totals(self.collect())
        target = self.archive / self.path.name
        self.path.rename(target)
        self.assert_totals(self.collect())
        self.write(target, self.records + [self.event(4000, 1200, 1600, -10)])
        self.assert_totals(self.collect(), 4000, 2800)

    def test_truncation_then_replay_and_append(self):
        self.assert_totals(self.collect())
        self.write(self.path, self.records[:2])
        cache = self.collect()
        self.assertEqual(cache['_today_tokens'], 3000)
        self.assertEqual(cache['_recent_days'][-1]['tokens'], 3000)
        self.write(self.path, self.records + [self.event(4000, 1200, 1600, -10)])
        self.assert_totals(self.collect(), 4000, 2800)

    def test_duplicate_rollout_and_shared_prefix(self):
        self.write(self.archive / self.path.name, self.records)
        self.assert_totals(self.collect())
        self.write(self.path, self.records + [self.event(4000, 1200, 1600, -10)])
        self.assert_totals(self.collect(), 4000, 2800)

    def test_separate_sessions_with_equal_counters_are_distinct(self):
        self.write(self.archive / 'second.jsonl', [self.meta('two')] + self.records[1:])
        self.assert_totals(self.collect(), 6000, 4200)

    def test_counter_reset_is_new_usage(self):
        self.write(self.path, self.records + [self.event(500, 100, 200, -10)])
        self.assert_totals(self.collect(), 3500, 2500)

    def test_schema_three_state_is_rebuilt_without_replay_inflation(self):
        self.activity.write_text(json.dumps({'schemaVersion': 3,
            'files': {}, 'events': [[self.now - 60, 9000]], 'dailyTokens': {}}))
        self.assert_totals(self.collect())
        self.assert_totals(self.collect())
        self.assertEqual(json.loads(self.activity.read_text())['schemaVersion'], 4)

    def test_yesterday_replay_after_hour_events_are_pruned(self):
        self.collect()
        self.env['CODEX_USAGE_NOW'] = str(self.now + 86400)
        self.collect()
        self.path.rename(self.archive / self.path.name)
        os.utime(self.archive / self.path.name, (self.now + 86400, self.now + 86400))
        cache = self.collect()
        self.assertEqual(sum(x['tokens'] for x in cache['_recent_days']), 3000)
        self.assertEqual(cache['_today_tokens'], 0)

    def test_plus_windows_use_minutes_percent_and_unix_seconds(self):
        cache = self.collect()
        self.assertEqual([x['minutes'] for x in cache['_windows']], [300, 10080])
        self.assertEqual([x['utilization'] for x in cache['_windows']], [0.25, 0.25])
        self.assertEqual(cache['_windows'][0]['reset'], self.now + 300 * 60)
        self.assertEqual(cache['5h-utilization'], '0.2500')
        self.assertEqual(cache['7d-utilization'], '0.2500')

    def test_free_window_does_not_restore_old_paid_windows(self):
        self.records[-1]['payload']['rate_limits'] = {
            'limit_id': 'codex', 'plan_type': 'plus',
            'primary': {'used_percent': 20, 'window_minutes': 300, 'resets_at': self.now + 3000},
            'secondary': {'used_percent': 10, 'window_minutes': 10080, 'resets_at': self.now + 90000}}
        self.write(self.path, self.records)
        self.collect()
        self.set_limits('free', 43200)
        cache = self.collect()
        self.assertEqual(cache['_plan'], 'free')
        self.assertEqual([x['minutes'] for x in cache['_windows']], [43200])
        self.assertEqual(cache['_windows'][0]['label'], '30d')
        self.assertEqual(cache['5h-utilization'], '')
        self.assertEqual(cache['7d-utilization'], '')

    def test_weekly_only_plan_does_not_gain_five_hour_window(self):
        self.set_limits('pro', None, 10080)
        cache = self.collect()
        self.assertEqual([x['minutes'] for x in cache['_windows']], [10080])
        self.assertEqual(cache['5h-utilization'], '')

    def test_rollout_fallback_free_window(self):
        self.env.pop('CODEX_USAGE_RATE_LIMITS_JSON')
        self.records[-1]['payload']['rate_limits'] = {
            'limit_id': 'codex', 'plan_type': 'free',
            'primary': {'used_percent': 48, 'window_minutes': 43200,
                        'resets_at': self.now + 2500000}}
        self.write(self.path, self.records)
        cache = self.collect()
        self.assertEqual(cache['_source'], 'rollout')
        self.assertEqual([x['minutes'] for x in cache['_windows']], [43200])
        self.assertEqual(cache['_windows'][0]['utilization'], 0.48)

    def test_rollout_fallback_ignores_previous_plan_windows(self):
        paid = dict(self.records[1])
        paid['payload'] = dict(paid['payload'], rate_limits={
            'limit_id': 'codex', 'plan_type': 'plus',
            'primary': {'used_percent': 20, 'window_minutes': 300, 'resets_at': self.now + 3000},
            'secondary': {'used_percent': 10, 'window_minutes': 10080, 'resets_at': self.now + 90000}})
        self.records[1] = paid
        self.test_rollout_fallback_free_window()

    def test_actual_qml_window_parser_and_reset_countdown(self):
        # Execute the shipped function bodies rather than a Python translation.
        source = (ROOT / 'ryoku/shell/quickshell/inir/defaults/widgets/ai-usage/UsageModel.qml').read_text()
        functions = []
        for name in ('pct', 'normalizeWindows', 'resetIn'):
            start = source.index('    function ' + name + '(')
            end = source.index('\n    }', start) + len('\n    }')
            function = source[start:end]
            function = re.sub(r': (?:string|var|int|double)(?=[,)])', '', function)
            function = re.sub(r'\): (?:string|var|int) \{', ') {', function)
            functions.append(function)
        script = '\n'.join(functions) + '''
const assert = require('node:assert/strict');
const root = {pct, tick: 0, windowTokens: '', weekTokens: '', pct5h: 25, percent7d: 10,
              reset5hTs: 123, reset7dTs: 456};
const free = normalizeWindows({_windows: [{minutes:43200, utilization:0.48, reset:12345}]});
assert.deepEqual(free, [{label:'30d', caption:'', pct:48, resetTs:12345}]);
assert.deepEqual(normalizeWindows({_windows: []}), []);
assert.deepEqual(normalizeWindows({_plan:'free'}), []);
assert.deepEqual(normalizeWindows({_windows: [{minutes:300, utilization:0.25},
    {minutes:10080, utilization:0.1}]}).map(x=>[x.label,x.pct]), [['5h',25],['7d',10]]);
assert.deepEqual(normalizeWindows({_windows:[{minutes:60,utilization:0}]}).map(x=>x.label), ['1h']);
assert.deepEqual(normalizeWindows({'5h-utilization':'0','7d-utilization':'0.1'}).map(x=>x.label), ['5h','7d']);
Date.now = () => 1000000;
assert.equal(resetIn(0), '');
assert.equal(resetIn(999), 'reset due');
assert.equal(resetIn(1060), 'resets in 1 min');
assert.equal(resetIn(1000+30*86400), 'resets in 30 days');
'''
        subprocess.run(['node', '-e', script], check=True, capture_output=True, text=True)

    def rpc_fixture(self, mode='ok'):
        tools = self.home / 'rpc-bin'
        tools.mkdir(exist_ok=True)
        fake = tools / 'codex'
        fake.write_text('''#!/usr/bin/env python3
import json,os,sys
from pathlib import Path
trace=Path(os.environ['RPC_TRACE'])
record={'pid':os.getpid(),'args':sys.argv[1:],'methods':[]}
trace.write_text(json.dumps(record))
for line in sys.stdin:
    request=json.loads(line)
    method=request['method']
    record['methods'].append(method)
    trace.write_text(json.dumps(record))
    if 'id' not in request: continue
    mode=os.environ.get('RPC_TEST_MODE')
    if mode=='initialize_eof' and method=='initialize': sys.exit(0)
    if (mode=='initialize_error' and method=='initialize') or (mode=='limits_error' and method=='account/rateLimits/read'):
        reply={'id':request['id'],'error':{'code':-1,'message':os.environ['OPENAI_API_KEY']}}
    elif method=='account/read':
        reply={'id':request['id'],'result':{'account':{'type':os.environ.get('RPC_ACCOUNT_TYPE','chatgpt'),'planType':'plus',
            'email':'private@example.invalid','access_token':os.environ['OPENAI_API_KEY']}}}
    elif method=='account/rateLimits/read':
        reply={'id':request['id'],'result':{'rateLimits':{'limitId':'codex','planType':'plus',
            'primary':{'usedPercent':25,'windowDurationMins':300,
            'resetsAt':int(os.environ['CODEX_USAGE_NOW'])+18000},
            'access_token':os.environ['OPENAI_API_KEY']}}}
    else: reply={'id':request['id'],'result':{}}
    print(json.dumps(reply),flush=True)
''')
        fake.chmod(0o755)
        env = dict(self.env, PATH=str(tools) + os.pathsep + os.environ['PATH'],
                   RPC_TRACE=str(self.home / 'rpc-trace.json'), RPC_TEST_MODE=mode,
                   OPENAI_API_KEY='FAKE-CREDENTIAL-MUST-NOT-LEAK')
        env.pop('CODEX_USAGE_DISABLE_RPC', None)
        env.pop('CODEX_USAGE_RATE_LIMITS_JSON', None)
        return env

    def test_rpc_uses_private_stdio_and_publishes_no_credentials(self):
        self.env = self.rpc_fixture()
        data = self.collect()
        trace = json.loads((self.home / 'rpc-trace.json').read_text())
        self.assertEqual(trace['methods'], ['initialize', 'initialized', 'account/read',
                                            'account/rateLimits/read'])
        self.assertEqual(trace['args'][-2:], ['--listen', 'stdio://'])
        self.assertNotIn(self.env['OPENAI_API_KEY'], json.dumps(trace))
        self.assertNotIn(self.env['OPENAI_API_KEY'], self.cache.read_text())
        self.assertNotIn('private@example.invalid', self.cache.read_text())
        self.assertEqual(data['_source'], 'rpc')
        self.assertEqual(data['_windows'][0]['utilization'], 0.25)

    def test_rpc_failure_reaps_child_and_redacts_provider_errors(self):
        module = runpy.run_path(str(COLLECTOR))
        namespace = module['fetch_rpc'].__globals__
        for mode in ('initialize_error', 'initialize_eof', 'limits_error'):
            with self.subTest(mode=mode):
                env = self.rpc_fixture(mode)
                namespace['ENV'] = env
                errors = io.StringIO()
                with patch.dict(os.environ, {'CODEX_USAGE_DISABLE_RPC': '0'}), contextlib.redirect_stderr(errors):
                    self.assertIsNone(module['fetch_rpc']())
                self.assertNotIn(env['OPENAI_API_KEY'], errors.getvalue())
                trace = json.loads((self.home / 'rpc-trace.json').read_text())
                try:
                    os.kill(trace['pid'], 0)
                except ProcessLookupError:
                    continue
                # A regression must not leave the test's fake server behind.
                os.kill(trace['pid'], signal.SIGKILL)
                os.waitpid(trace['pid'], 0)
                self.fail('RPC child survived a failed request')

    def test_api_key_and_bedrock_do_not_reuse_old_chatgpt_limits(self):
        baseline = dict(self.env)
        self.records[-1]['payload']['rate_limits'] = {
            'limit_id': 'codex', 'plan_type': 'plus',
            'primary': {'used_percent': 48, 'window_minutes': 300,
                        'resets_at': self.now + 3000}}
        self.write(self.path, self.records)
        for account_type in ('apiKey', 'amazonBedrock'):
            with self.subTest(account_type=account_type):
                self.env = dict(baseline)
                self.assertTrue(self.collect()['_windows'])
                self.env = self.rpc_fixture('limits_error')
                self.env['RPC_ACCOUNT_TYPE'] = account_type
                data = self.collect()
                self.assertEqual(data['_windows'], [])
                self.assertEqual(data['buckets'], [])
                self.assertEqual(data['_plan'], '')
                self.assertEqual(data['5h-utilization'], '')
                self.assertEqual(data['7d-utilization'], '')
                self.assertEqual(data['_source'], 'rpc')
                self.assert_totals(data)
                self.assertEqual(data['_spending']['kind'], 'api-equivalent')
                trace = json.loads((self.home / 'rpc-trace.json').read_text())
                self.assertEqual(trace['methods'], ['initialize', 'initialized', 'account/read'])
                self.assertNotIn(self.env['OPENAI_API_KEY'], self.cache.read_text())

    def test_rpc_partial_json_line_cannot_bypass_timeout(self):
        script = '''
import io,os,runpy,sys
from types import SimpleNamespace
module=runpy.run_path(sys.argv[1])
read,write=os.pipe()
with os.fdopen(read,'rb',buffering=0) as stream:
    os.write(write,b'{"id":')
    try:
        module['rpc_request'](SimpleNamespace(stdin=io.StringIO(),stdout=stream),1,'initialize',timeout=0.05)
    except TimeoutError:
        pass
    else:
        raise AssertionError('partial JSON escaped timeout')
    finally:
        os.close(write)
'''
        result = subprocess.run([sys.executable, '-c', script, str(COLLECTOR)],
                                capture_output=True, text=True, timeout=2)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_rpc_preserves_buffered_frames_between_requests(self):
        from types import SimpleNamespace
        module = runpy.run_path(str(COLLECTOR))
        with tempfile.TemporaryFile() as stream:
            stream.write(b'{"id":1,"result":{"ok":1}}\n{"id":2,"result":{"ok":2}}\n')
            stream.seek(0)
            proc = SimpleNamespace(stdin=io.StringIO(), stdout=stream)
            for index in (1, 2):
                self.assertEqual(module['rpc_request'](proc, index, 'initialize')['result']['ok'], index)

    def test_rpc_bounds_response_size(self):
        from types import SimpleNamespace
        module = runpy.run_path(str(COLLECTOR))
        with tempfile.TemporaryFile() as stream:
            stream.write(b'x' * (1024 * 1024 + 1))
            stream.seek(0)
            proc = SimpleNamespace(stdin=io.StringIO(), stdout=stream)
            with self.assertRaisesRegex(RuntimeError, 'size limit'):
                module['rpc_request'](proc, 1, 'initialize')

    def test_usage_and_activity_cache_permissions_are_private(self):
        module = runpy.run_path(str(COLLECTOR))
        for path in (self.cache, self.activity):
            path.write_text('{}')
            path.chmod(0o644)
            old_umask = os.umask(0o022)
            try:
                module['atomic_write_json'](path, {'safe': True})
            finally:
                os.umask(old_umask)
            self.assertEqual(path.stat().st_mode & 0o777, 0o600)
            self.assertEqual(json.loads(path.read_text()), {'safe': True})

    def test_failed_cache_publish_preserves_previous_file(self):
        module = runpy.run_path(str(COLLECTOR))
        self.cache.write_text('{"previous":true}')
        with patch.object(os, 'replace', side_effect=OSError('fixture failure')):
            with self.assertRaises(OSError):
                module['atomic_write_json'](self.cache, {'replacement': True})
        self.assertEqual(json.loads(self.cache.read_text()), {'previous': True})
        self.assertEqual(list(self.home.glob('.usage.json.*')), [])

    def spending_fixture(self):
        self.now = int(datetime.now().replace(day=7, hour=12, minute=0, second=0, microsecond=0).timestamp())
        self.env['CODEX_USAGE_NOW'] = str(self.now)
        self.set_limits('plus', 300, 10080)
        self.records = [self.meta('one'), {'type': 'turn_context', 'payload': {'model': 'gpt-test'}},
                        self.event(1000, 200, 300, -120), self.event(3000, 900, 1200, -60)]
        self.write(self.path, self.records)
        self.set_prices()

    def set_prices(self, multiplier=1, **extra):
        rates = dict(input=2*multiplier, output=10*multiplier,
                     cache_read=0.1*multiplier, cache_write=2.5*multiplier)
        rates.update(extra)
        self.env['CODEX_USAGE_PRICES_JSON'] = json.dumps({'openai': {'models': {
            'gpt-test': {'cost': rates}}}})

    def ledger(self):
        month = datetime.fromtimestamp(int(self.env['CODEX_USAGE_NOW'])).strftime('%Y-%m')
        return json.loads((self.home / 'spending' / f'spending-{month}.json').read_text())

    def test_spending_daily_snapshot_deduplicates_and_preserves_seen_usage(self):
        self.spending_fixture()
        first = self.collect()['_spending']
        self.assertAlmostEqual(first['amount'], 0.01389)
        self.assertEqual(first['priced_tokens'], 3000)
        self.assertFalse(first['partial'])
        self.write(self.archive / self.path.name, self.records)
        self.set_prices(multiplier=10)
        second = self.collect()['_spending']
        self.assertEqual(first, second)
        self.assertEqual(len(self.ledger()['events']), 2)
        self.path.unlink()
        (self.archive / self.path.name).unlink()
        self.assertEqual(self.collect()['_spending']['amount'], first['amount'])
        self.assertEqual(next(iter(self.ledger()['days'].values()))['prices']['gpt-test']['input'], 2)

    def test_spending_rollover_uses_new_prices_and_accepts_late_old_usage(self):
        self.spending_fixture()
        self.collect()
        first_day = datetime.fromtimestamp(self.now).strftime('%Y-%m-%d')
        self.now += 86400
        self.env['CODEX_USAGE_NOW'] = str(self.now)
        self.set_prices(multiplier=10)
        self.records.append(self.event(5000, 1500, 2000, -60))
        self.write(self.path, self.records)
        estimate = self.collect()['_spending']
        self.assertAlmostEqual(estimate['amount'], 0.10649)
        self.assertEqual(len(estimate['priced_days']), 2)
        late = self.archive / 'late.jsonl'
        self.write(late, [self.meta('late'), {'type': 'turn_context', 'payload': {'model': 'gpt-test'}},
                         self.event(500, 0, 100, -86400)])
        self.assertAlmostEqual(self.collect()['_spending']['amount'], 0.10829)
        self.assertEqual(self.ledger()['days'][first_day]['prices']['gpt-test']['input'], 2)

    def test_spending_does_not_backdate_first_observed_prices(self):
        self.spending_fixture()
        self.records = [self.meta('one'), {'type': 'turn_context', 'payload': {'model': 'gpt-test'}},
                        self.event(3000, 900, 1200, -86400)]
        self.write(self.path, self.records)
        estimate = self.collect()['_spending']
        self.assertIsNone(estimate['amount'])
        self.assertTrue(estimate['partial'])
        self.assertEqual(estimate['unpriced_tokens'], 3000)
        self.assertEqual(estimate['priced_days'], [])
        old = next(day for day in self.ledger()['days'].values() if day['unpriced_tokens'])
        self.assertNotIn('prices', old)

    def test_spending_context_tiers_cache_write_and_reasoning_are_counted_once(self):
        self.spending_fixture()
        self.set_prices(tiers=[{'tier': {'type': 'context', 'size': 600},
                               'input': 4, 'output': 15, 'cache_read': 0.2, 'cache_write': 5}])
        event = self.event(1000, 200, 300, -60)
        event['payload']['info']['total_token_usage'].update(cache_write_input_tokens=100,
                                                            reasoning_output_tokens=50)
        self.records = self.records[:2] + [event]
        self.write(self.path, self.records)
        estimate = self.collect()['_spending']
        self.assertAlmostEqual(estimate['amount'], 0.00664)
        self.assertEqual(estimate['priced_tokens'], 1000)

    def test_spending_missing_prices_and_nonstandard_tiers_are_unavailable(self):
        self.spending_fixture()
        self.set_prices(cache_read=None)
        estimate = self.collect()['_spending']
        self.assertIsNone(estimate['amount'])
        self.assertEqual(estimate['unpriced_tokens'], 3000)
        self.records[1]['payload']['service_tier'] = 'fast'
        self.write(self.path, self.records)
        self.assertIsNone(self.collect()['_spending']['amount'])

    def test_spending_offline_rates_are_marked_stale_then_refresh_today_only(self):
        self.spending_fixture()
        self.collect()
        self.now += 86400
        self.env['CODEX_USAGE_NOW'] = str(self.now)
        self.env.pop('CODEX_USAGE_PRICES_JSON')
        self.records.append(self.event(5000, 1500, 2000, -60))
        self.write(self.path, self.records)
        offline = self.collect()['_spending']
        self.assertTrue(offline['stale_prices'])
        self.assertAlmostEqual(offline['amount'], 0.02315)
        self.set_prices(multiplier=10)
        self.env['CODEX_USAGE_NOW'] = str(self.now+901)
        refreshed = self.collect()['_spending']
        self.assertFalse(refreshed['stale_prices'])
        self.assertAlmostEqual(refreshed['amount'], 0.10649)

    def test_spending_corrupt_ledger_is_not_overwritten(self):
        self.spending_fixture()
        month = datetime.fromtimestamp(self.now).strftime('%Y-%m')
        path = self.home / 'spending' / f'spending-{month}.json'
        path.parent.mkdir()
        path.write_text('broken history')
        self.assertIsNone(self.collect()['_spending']['amount'])
        self.assertEqual(path.read_text(), 'broken history')

    def test_public_price_request_contains_no_login_credentials(self):
        module = runpy.run_path(str(COLLECTOR))
        fixture = {'openai': {'models': {'gpt-test': {'cost': {'input': 2, 'output': 10}}}}}
        opener = unittest.mock.MagicMock()
        opener.open.return_value.__enter__.return_value.read.return_value = json.dumps(fixture).encode()
        clean_env = dict(os.environ, CODEX_USAGE_DISABLE_PRICES='0', OPENAI_API_KEY='FAKE-SECRET')
        clean_env.pop('CODEX_USAGE_PRICES_JSON', None)
        with patch.dict(os.environ, clean_env, clear=True), patch.object(urllib_request := module['urllib'].request,
                                                                       'build_opener', return_value=opener) as build:
            self.assertIn('gpt-test', module['fetch_prices']())
        request = opener.open.call_args.args[0]
        self.assertEqual(request.full_url, 'https://models.dev/api.json')
        self.assertIsNone(request.data)
        self.assertNotIn('authorization', {key.lower() for key in request.headers})
        self.assertNotIn('FAKE-SECRET', str(request.headers))
        redirect = build.call_args.args[1]
        self.assertIsNone(redirect.redirect_request(None, None, 302, '', {}, 'https://other.invalid'))

    def test_spending_survives_without_quota_rpc_or_snapshot(self):
        self.spending_fixture()
        self.env.pop('CODEX_USAGE_RATE_LIMITS_JSON')
        cache = self.collect()
        self.assertEqual(cache['_source'], 'local')
        self.assertAlmostEqual(cache['_spending']['amount'], 0.01389)



if __name__ == '__main__':
    unittest.main(verbosity=2)

#!/usr/bin/env python3
"""Regression cases for Codex rollout replay and account quota windows."""
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import time
import unittest

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
                        CODEX_USAGE_ACTIVITY_FILE=str(self.activity))
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



if __name__ == '__main__':
    unittest.main(verbosity=2)

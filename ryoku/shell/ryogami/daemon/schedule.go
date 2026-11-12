package main

import (
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// Caps one sleep so a clock jump or suspend never parks the engine for a whole day.
	maxScheduleWait     = 6 * time.Hour
	scheduleIdleRecheck = 6 * time.Hour
	powerSupplyDir      = "/sys/class/power_supply"
)

func init() {
	onStart(func(d *daemon) { go d.runScheduleEngine() })
	watchSetting("schedule", func(d *daemon, _ string, _ interface{}) { scheduleReload() })
}

type cmpOp int

const (
	cmpGe cmpOp = iota
	cmpLe
	cmpGt
	cmpLt
	cmpEq
)

type atKind int

const (
	atSunrise atKind = iota
	atSunset
	atClock
)

type atSpec struct {
	kind   atKind
	offset int
	clock  int
}

type dateSpec struct {
	year    int
	hasYear bool
	month   int
	day     int
}

type dateRange struct {
	start dateSpec
	end   dateSpec
}

type schedPredicate struct {
	kind      string // timewindow, time, weekday, date, year, weather, power, battery, output, outputcount, never
	from      atSpec
	to        atSpec
	cmp       cmpOp
	at        atSpec
	days      []int
	date      dateRange
	year      int64
	tags      []string
	onBattery bool
	percent   int64
	output    string
	count     int64
}

type schedExpr struct {
	op       string
	pred     *schedPredicate
	children []*schedExpr
}

type scheduleRule struct {
	priority      int64
	name          string
	cond          *schedExpr
	set           string // derived target: random | folder:<dir> | playlist:<id> | <wallpaper ref>
	mode          string
	rotateMinutes int
	noRepeat      bool
	randomTypes   []string
	randomFav     *bool
}

type scheduleNow struct {
	year, month, day int
	wday             int
	minute           int
	sunrise, sunset  int
	weather          []string
	onBattery        *bool
	batteryPercent   *int
	outputs          []string
}

func (d *daemon) loadScheduleRules() []scheduleRule {
	if !d.settingBool("schedule.enabled") {
		return nil
	}
	var out []scheduleRule
	for _, item := range d.settingList("schedule.rules") {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if r, ok := parseScheduleRule(obj); ok {
			out = append(out, r)
		}
	}
	return out
}

func parseScheduleRule(m map[string]interface{}) (scheduleRule, bool) {
	if enabled, ok := m["enabled"].(bool); ok && !enabled {
		return scheduleRule{}, false
	}
	r := scheduleRule{
		priority: 50,
		cond:     clauseNever(),
	}
	if p, ok := m["priority"].(float64); ok {
		r.priority = int64(p)
	}
	r.name, _ = m["name"].(string)
	if v, ok := m["rotateMinutes"].(float64); ok && v > 0 {
		r.rotateMinutes = int(v)
	}
	r.noRepeat, _ = m["noRepeat"].(bool)
	if cond, ok := m["condition"]; ok {
		r.cond = parseSchedCondition(cond)
	}

	switch theme, _ := m["theme"].(string); theme {
	case "light", "dark":
		r.mode = theme
	case "keep", "":
		r.mode, _ = m["mode"].(string)
	}

	if tgt, ok := m["target"].(map[string]interface{}); ok {
		r.set = targetToSet(tgt)
		if strings.HasPrefix(r.set, "random") || strings.HasPrefix(r.set, "folder:") {
			r.randomTypes, r.randomFav = targetFilters(tgt)
		}
	} else if set, ok := m["set"].(string); ok {
		r.set = strings.TrimSpace(set)
	}
	if strings.TrimSpace(r.set) == "" {
		return scheduleRule{}, false
	}
	return r, true
}

func clauseNever() *schedExpr {
	return &schedExpr{op: "clause", pred: &schedPredicate{kind: "never"}}
}

func targetToSet(tgt map[string]interface{}) string {
	value := strings.TrimSpace(toStr(tgt["value"]))
	switch strings.TrimSpace(toStr(tgt["type"])) {
	case "random":
		return "random"
	case "folder":
		if value == "" {
			return ""
		}
		return "folder:" + value
	case "playlist":
		if value == "" {
			return ""
		}
		return "playlist:" + value
	case "wallpaper":
		return value
	}
	return ""
}

func targetFilters(tgt map[string]interface{}) ([]string, *bool) {
	filters, ok := tgt["filters"].(map[string]interface{})
	if !ok {
		return nil, nil
	}
	var types []string
	if raw, ok := filters["types"].([]interface{}); ok {
		for _, t := range raw {
			if s := strings.TrimSpace(toStr(t)); s != "" {
				types = append(types, s)
			}
		}
	}
	var fav *bool
	if b, ok := filters["favouritesOnly"].(bool); ok {
		fav = &b
	}
	return types, fav
}

const (
	schedMaxDepth = 32
	schedMaxNodes = 256
)

func parseSchedExpression(v interface{}) *schedExpr {
	never := &schedExpr{op: "clause", pred: &schedPredicate{kind: "never"}}
	root, ok := v.(map[string]interface{})
	if !ok {
		return never
	}
	version := 0
	if ver, ok := root["version"].(float64); ok {
		version = int(ver)
	}
	if version != 1 && version != 2 {
		return never
	}
	node, ok := root["root"].(map[string]interface{})
	if !ok {
		return never
	}
	if kind, _ := node["kind"].(string); kind != "group" {
		return never
	}
	remaining := schedMaxNodes
	if e := parseSchedNode(node, version, 0, &remaining); e != nil {
		return e
	}
	return never
}

func parseSchedNode(node map[string]interface{}, version, depth int, remaining *int) *schedExpr {
	if depth > schedMaxDepth || *remaining == 0 {
		return nil
	}
	*remaining--
	switch kind, _ := node["kind"].(string); kind {
	case "predicate":
		val, ok := node["value"].(string)
		if !ok {
			return nil
		}
		pred := parseSchedClause(val, version)
		if pred.kind == "never" {
			return nil
		}
		return maybeNegate(&schedExpr{op: "clause", pred: &pred}, node)
	case "group":
		rawChildren, ok := node["children"].([]interface{})
		if !ok {
			return nil
		}
		children := make([]*schedExpr, 0, len(rawChildren))
		for _, c := range rawChildren {
			cm, ok := c.(map[string]interface{})
			if !ok {
				return nil
			}
			child := parseSchedNode(cm, version, depth+1, remaining)
			if child == nil {
				return nil
			}
			children = append(children, child)
		}
		op, _ := node["operator"].(string)
		if op != "all" && op != "any" {
			return nil
		}
		return maybeNegate(&schedExpr{op: op, children: children}, node)
	default:
		return nil
	}
}

func maybeNegate(e *schedExpr, node map[string]interface{}) *schedExpr {
	neg, present := node["negated"]
	if !present {
		return e
	}
	b, ok := neg.(bool)
	if !ok {
		return nil
	}
	if !b {
		return e
	}
	return &schedExpr{op: "not", children: []*schedExpr{e}}
}

// Accepts the editor's structured group or the older string-encoded tree.
func parseSchedCondition(v interface{}) *schedExpr {
	m, ok := v.(map[string]interface{})
	if !ok {
		return clauseNever()
	}
	if _, structured := m["op"]; structured {
		remaining := schedMaxNodes
		if e := parseSchedGroup(m, 0, &remaining); e != nil {
			return e
		}
		return clauseNever()
	}
	return parseSchedExpression(v)
}

func parseSchedGroup(node map[string]interface{}, depth int, remaining *int) *schedExpr {
	if depth > schedMaxDepth || *remaining == 0 {
		return nil
	}
	*remaining--
	op, _ := node["op"].(string)
	if op != "all" && op != "any" {
		return nil
	}
	rawChildren, _ := node["children"].([]interface{})
	children := make([]*schedExpr, 0, len(rawChildren))
	for _, c := range rawChildren {
		cm, ok := c.(map[string]interface{})
		if !ok {
			return nil
		}
		var child *schedExpr
		if _, isPredicate := cm["block"]; isPredicate {
			child = parseSchedPredicateObj(cm)
		} else {
			child = parseSchedGroup(cm, depth+1, remaining)
		}
		if child == nil {
			return nil
		}
		children = append(children, child)
	}
	return maybeNegate(&schedExpr{op: op, children: children}, node)
}

func parseSchedPredicateObj(node map[string]interface{}) *schedExpr {
	block, _ := node["block"].(string)
	var pred schedPredicate
	switch block {
	case "weekday":
		var days []int
		if raw, ok := node["days"].([]interface{}); ok {
			for _, d := range raw {
				if n, ok := weekdayNum(toStr(d)); ok {
					days = append(days, n)
				}
			}
		}
		if len(days) == 0 {
			return nil
		}
		pred = schedPredicate{kind: "weekday", days: days}
	case "weather":
		var tags []string
		if raw, ok := node["tags"].([]interface{}); ok {
			for _, t := range raw {
				if s := strings.ToLower(strings.TrimSpace(toStr(t))); s != "" {
					tags = append(tags, s)
				}
			}
		}
		if len(tags) == 0 {
			return nil
		}
		pred = schedPredicate{kind: "weather", tags: tags}
	case "timewindow":
		from, ok1 := parseAt(toStr(node["from"]))
		to, ok2 := parseAt(toStr(node["to"]))
		if !ok1 || !ok2 {
			return nil
		}
		pred = schedPredicate{kind: "timewindow", from: from, to: to}
	case "timecmp":
		at, ok := parseAt(toStr(node["at"]))
		if !ok {
			return nil
		}
		pred = schedPredicate{kind: "time", cmp: parseCmpOpStr(toStr(node["op"])), at: at}
	case "date":
		p := parseSchedClause("date:"+toStr(node["value"]), 2)
		if p.kind == "never" {
			return nil
		}
		pred = p
	case "year":
		y, ok := toInt64(node["value"])
		if !ok {
			return nil
		}
		pred = schedPredicate{kind: "year", cmp: parseCmpOpStr(toStr(node["op"])), year: y}
	case "power":
		switch strings.ToLower(strings.TrimSpace(toStr(node["source"]))) {
		case "battery":
			pred = schedPredicate{kind: "power", onBattery: true}
		case "external":
			pred = schedPredicate{kind: "power", onBattery: false}
		default:
			return nil
		}
	case "battery":
		n, ok := toInt64(node["value"])
		if !ok || n < 0 || n > 100 {
			return nil
		}
		pred = schedPredicate{kind: "battery", cmp: parseCmpOpStr(toStr(node["op"])), percent: n}
	case "output":
		name := strings.TrimSpace(toStr(node["value"]))
		if name == "" || len(name) > 128 {
			return nil
		}
		pred = schedPredicate{kind: "output", output: name}
	case "outputcount":
		n, ok := toInt64(node["value"])
		if !ok || n < 0 || n > 64 {
			return nil
		}
		pred = schedPredicate{kind: "outputcount", cmp: parseCmpOpStr(toStr(node["op"])), count: n}
	case "raw":
		p := parseSchedClause(strings.TrimSpace(toStr(node["value"])), 2)
		if p.kind == "never" {
			return nil
		}
		pred = p
	default:
		return nil
	}
	return maybeNegate(&schedExpr{op: "clause", pred: &pred}, node)
}

func parseCmpOpStr(op string) cmpOp {
	switch strings.TrimSpace(op) {
	case ">=":
		return cmpGe
	case "<=":
		return cmpLe
	case ">":
		return cmpGt
	case "<":
		return cmpLt
	}
	return cmpEq
}

func toStr(v interface{}) string {
	switch s := v.(type) {
	case string:
		return s
	case float64:
		return strconv.FormatFloat(s, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(s)
	}
	return ""
}

func toInt64(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case string:
		i, err := strconv.ParseInt(strings.TrimSpace(n), 10, 64)
		return i, err == nil
	}
	return 0, false
}

func parseCmp(spec string) (cmpOp, string) {
	switch {
	case strings.HasPrefix(spec, ">="):
		return cmpGe, spec[2:]
	case strings.HasPrefix(spec, "<="):
		return cmpLe, spec[2:]
	case strings.HasPrefix(spec, ">"):
		return cmpGt, spec[1:]
	case strings.HasPrefix(spec, "<"):
		return cmpLt, spec[1:]
	case strings.HasPrefix(spec, "="):
		return cmpEq, spec[1:]
	}
	return cmpEq, spec
}

func weekdayNum(text string) (int, bool) {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "sun", "sunday":
		return 0, true
	case "mon", "monday":
		return 1, true
	case "tue", "tuesday":
		return 2, true
	case "wed", "wednesday":
		return 3, true
	case "thu", "thursday":
		return 4, true
	case "fri", "friday":
		return 5, true
	case "sat", "saturday":
		return 6, true
	}
	if n, err := strconv.Atoi(strings.TrimSpace(text)); err == nil && n >= 0 && n < 7 {
		return n, true
	}
	return 0, false
}

func offsetSolar(a atSpec) bool {
	return (a.kind == atSunrise || a.kind == atSunset) && a.offset != 0
}

func parseAt(text string) (atSpec, bool) {
	value := strings.ToLower(strings.TrimSpace(text))
	for name, kind := range map[string]atKind{"sunrise": atSunrise, "sunset": atSunset} {
		if rest, ok := strings.CutPrefix(value, name); ok {
			if rest == "" {
				return atSpec{kind: kind}, true
			}
			off, err := strconv.Atoi(rest)
			if err != nil || off < -720 || off > 720 {
				return atSpec{}, false
			}
			return atSpec{kind: kind, offset: off}, true
		}
	}
	h, m, found := strings.Cut(value, ":")
	if !found {
		return atSpec{}, false
	}
	hour, err1 := strconv.Atoi(strings.TrimSpace(h))
	minute, err2 := strconv.Atoi(strings.TrimSpace(m))
	if err1 != nil || err2 != nil || hour < 0 || hour >= 24 || minute < 0 || minute >= 60 {
		return atSpec{}, false
	}
	return atSpec{kind: atClock, clock: hour*60 + minute}, true
}

func parseSchedDate(text string) (dateSpec, bool) {
	valid := func(month, day int) bool { return month >= 1 && month <= 12 && day >= 1 && day <= 31 }
	parts := strings.Split(strings.TrimSpace(text), "-")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	switch len(parts) {
	case 2:
		month, err1 := strconv.Atoi(parts[0])
		day, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil || !valid(month, day) {
			return dateSpec{}, false
		}
		return dateSpec{month: month, day: day}, true
	case 3:
		year, err0 := strconv.Atoi(parts[0])
		month, err1 := strconv.Atoi(parts[1])
		day, err2 := strconv.Atoi(parts[2])
		if err0 != nil || err1 != nil || err2 != nil || !valid(month, day) {
			return dateSpec{}, false
		}
		return dateSpec{year: year, hasYear: true, month: month, day: day}, true
	}
	return dateSpec{}, false
}

func neverPredicate() schedPredicate { return schedPredicate{kind: "never"} }

func parseSchedClause(token string, version int) schedPredicate {
	factor, value, found := strings.Cut(token, ":")
	if !found {
		return neverPredicate()
	}
	factor = strings.TrimSpace(factor)
	value = strings.TrimSpace(value)
	switch factor {
	case "time":
		return parseTimeClause(value, version)
	case "weekday", "day":
		var days []int
		for _, part := range strings.Split(value, ",") {
			if n, ok := weekdayNum(part); ok {
				days = append(days, n)
			}
		}
		if len(days) == 0 {
			return neverPredicate()
		}
		return schedPredicate{kind: "weekday", days: days}
	case "date":
		if from, to, ok := strings.Cut(value, ".."); ok {
			start, ok1 := parseSchedDate(from)
			end, ok2 := parseSchedDate(to)
			if !ok1 || !ok2 {
				return neverPredicate()
			}
			return schedPredicate{kind: "date", date: dateRange{start: start, end: end}}
		}
		d, ok := parseSchedDate(value)
		if !ok {
			return neverPredicate()
		}
		return schedPredicate{kind: "date", date: dateRange{start: d, end: d}}
	case "year":
		op, rest := parseCmp(value)
		y, err := strconv.ParseInt(strings.TrimSpace(rest), 10, 64)
		if err != nil {
			return neverPredicate()
		}
		return schedPredicate{kind: "year", cmp: op, year: y}
	case "weather":
		var tags []string
		for _, t := range strings.Split(value, ",") {
			t = strings.ToLower(strings.TrimSpace(t))
			if t != "" {
				tags = append(tags, t)
			}
		}
		if len(tags) == 0 {
			return neverPredicate()
		}
		return schedPredicate{kind: "weather", tags: tags}
	case "power":
		if version < 2 {
			return neverPredicate()
		}
		switch strings.ToLower(value) {
		case "battery":
			return schedPredicate{kind: "power", onBattery: true}
		case "external":
			return schedPredicate{kind: "power", onBattery: false}
		}
		return neverPredicate()
	case "battery":
		if version < 2 {
			return neverPredicate()
		}
		op, rest := parseCmp(value)
		n, err := strconv.ParseInt(strings.TrimSpace(rest), 10, 64)
		if err != nil || n < 0 || n > 100 {
			return neverPredicate()
		}
		return schedPredicate{kind: "battery", cmp: op, percent: n}
	case "output":
		if version < 2 {
			return neverPredicate()
		}
		if value == "" || len(value) > 128 {
			return neverPredicate()
		}
		return schedPredicate{kind: "output", output: value}
	case "outputs":
		if version < 2 {
			return neverPredicate()
		}
		op, rest := parseCmp(value)
		n, err := strconv.ParseInt(strings.TrimSpace(rest), 10, 64)
		if err != nil || n < 0 || n > 64 {
			return neverPredicate()
		}
		return schedPredicate{kind: "outputcount", cmp: op, count: n}
	}
	return neverPredicate()
}

func parseTimeClause(value string, version int) schedPredicate {
	if from, to, ok := strings.Cut(value, ".."); ok {
		fromAt, ok1 := parseAt(strings.TrimSpace(from))
		toAt, ok2 := parseAt(strings.TrimSpace(to))
		if !ok1 || !ok2 {
			return neverPredicate()
		}
		if version < 2 && (offsetSolar(fromAt) || offsetSolar(toAt)) {
			return neverPredicate()
		}
		return schedPredicate{kind: "timewindow", from: fromAt, to: toAt}
	}
	op, rest := parseCmp(value)
	at, ok := parseAt(strings.TrimSpace(rest))
	if !ok {
		return neverPredicate()
	}
	if version < 2 && offsetSolar(at) {
		return neverPredicate()
	}
	return schedPredicate{kind: "time", cmp: op, at: at}
}

func fireMinute(a atSpec, sunrise, sunset int) int {
	switch a.kind {
	case atSunrise:
		return wrapMinute(sunrise + a.offset)
	case atSunset:
		return wrapMinute(sunset + a.offset)
	}
	return a.clock
}

func wrapMinute(m int) int {
	m %= 1440
	if m < 0 {
		m += 1440
	}
	return m
}

func cmpOrd(op cmpOp, lhs, rhs int64) bool {
	switch op {
	case cmpGe:
		return lhs >= rhs
	case cmpLe:
		return lhs <= rhs
	case cmpGt:
		return lhs > rhs
	case cmpLt:
		return lhs < rhs
	default:
		return lhs == rhs
	}
}

func windowActive(from, until atSpec, minute, sunrise, sunset int) bool {
	start := fireMinute(from, sunrise, sunset)
	end := fireMinute(until, sunrise, sunset)
	if start <= end {
		return minute >= start && minute < end
	}
	return minute >= start || minute < end
}

func dateInRange(r dateRange, year, month, day int) bool {
	if r.start.hasYear && r.end.hasYear {
		today := [3]int{year, month, day}
		start := [3]int{r.start.year, r.start.month, r.start.day}
		end := [3]int{r.end.year, r.end.month, r.end.day}
		return tripleLE(start, today) && tripleLE(today, end)
	}
	md := [2]int{month, day}
	start := [2]int{r.start.month, r.start.day}
	end := [2]int{r.end.month, r.end.day}
	if pairLE(start, end) {
		return pairLE(start, md) && pairLE(md, end)
	}
	// A recurring range that wraps the new year (e.g. 12-15..01-05).
	return pairGE(md, start) || pairLE(md, end)
}

func tripleLE(a, b [3]int) bool {
	for i := 0; i < 3; i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return true
}

func pairLE(a, b [2]int) bool {
	if a[0] != b[0] {
		return a[0] < b[0]
	}
	return a[1] <= b[1]
}

func pairGE(a, b [2]int) bool { return pairLE(b, a) }

func clauseMatches(p *schedPredicate, now *scheduleNow) bool {
	switch p.kind {
	case "timewindow":
		return windowActive(p.from, p.to, now.minute, now.sunrise, now.sunset)
	case "time":
		return cmpOrd(p.cmp, int64(now.minute), int64(fireMinute(p.at, now.sunrise, now.sunset)))
	case "weekday":
		for _, d := range p.days {
			if d == now.wday {
				return true
			}
		}
		return false
	case "date":
		return dateInRange(p.date, now.year, now.month, now.day)
	case "year":
		return cmpOrd(p.cmp, int64(now.year), p.year)
	case "weather":
		for _, tag := range now.weather {
			for _, want := range p.tags {
				if tag == want {
					return true
				}
			}
		}
		return false
	case "power":
		return now.onBattery != nil && *now.onBattery == p.onBattery
	case "battery":
		return now.batteryPercent != nil && cmpOrd(p.cmp, int64(*now.batteryPercent), p.percent)
	case "output":
		for _, o := range now.outputs {
			if o == p.output {
				return true
			}
		}
		return false
	case "outputcount":
		return cmpOrd(p.cmp, int64(len(now.outputs)), p.count)
	}
	return false
}

func exprMatches(e *schedExpr, now *scheduleNow) bool {
	switch e.op {
	case "clause":
		return clauseMatches(e.pred, now)
	case "all":
		for _, c := range e.children {
			if !exprMatches(c, now) {
				return false
			}
		}
		return true
	case "any":
		for _, c := range e.children {
			if exprMatches(c, now) {
				return true
			}
		}
		return false
	case "not":
		return !exprMatches(e.children[0], now)
	}
	return false
}

func predicateCount(e *schedExpr) int {
	switch e.op {
	case "clause":
		return 1
	case "not":
		return predicateCount(e.children[0])
	default:
		sum := 0
		for _, c := range e.children {
			sum += predicateCount(c)
		}
		return sum
	}
}

func anyClause(e *schedExpr, pred func(*schedPredicate) bool) bool {
	switch e.op {
	case "clause":
		return pred(e.pred)
	case "not":
		return anyClause(e.children[0], pred)
	default:
		for _, c := range e.children {
			if anyClause(c, pred) {
				return true
			}
		}
		return false
	}
}

func visitClauses(e *schedExpr, visit func(*schedPredicate)) {
	switch e.op {
	case "clause":
		visit(e.pred)
	case "not":
		visitClauses(e.children[0], visit)
	default:
		for _, c := range e.children {
			visitClauses(c, visit)
		}
	}
}

// Ties go to the more specific rule (more predicates), then the earlier one.
func scheduleWinner(rules []scheduleRule, now *scheduleNow) int {
	best := -1
	var bestPriority int64
	bestSpecificity := -1
	for i := range rules {
		if !exprMatches(rules[i].cond, now) {
			continue
		}
		spec := predicateCount(rules[i].cond)
		if best == -1 || rules[i].priority < bestPriority ||
			(rules[i].priority == bestPriority && spec > bestSpecificity) {
			best = i
			bestPriority = rules[i].priority
			bestSpecificity = spec
		}
	}
	return best
}

func scheduleUsesWeather(rules []scheduleRule) bool {
	return anyRuleClause(rules, func(p *schedPredicate) bool { return p.kind == "weather" })
}

func scheduleUsesPower(rules []scheduleRule) bool {
	return anyRuleClause(rules, func(p *schedPredicate) bool { return p.kind == "power" || p.kind == "battery" })
}

func scheduleUsesOutputs(rules []scheduleRule) bool {
	return anyRuleClause(rules, func(p *schedPredicate) bool { return p.kind == "output" || p.kind == "outputcount" })
}

func anyRuleClause(rules []scheduleRule, pred func(*schedPredicate) bool) bool {
	for i := range rules {
		if anyClause(rules[i].cond, pred) {
			return true
		}
	}
	return false
}

// Weather rules cap the wait at 60 minutes so a forecast change is picked up.
func scheduleNextBoundaryWait(rules []scheduleRule, nowMinute, sunrise, sunset int) int {
	minutes := []int{0}
	for i := range rules {
		visitClauses(rules[i].cond, func(p *schedPredicate) {
			switch p.kind {
			case "timewindow":
				minutes = append(minutes, fireMinute(p.from, sunrise, sunset), fireMinute(p.to, sunrise, sunset))
			case "time":
				minutes = append(minutes, fireMinute(p.at, sunrise, sunset))
			}
		})
	}
	wait := 1440
	for _, m := range minutes {
		w := m - nowMinute
		if w <= 0 {
			w += 1440
		}
		if w < wait {
			wait = w
		}
	}
	if scheduleUsesWeather(rules) && wait > 60 {
		wait = 60
	}
	return wait
}

var scheduleWake = make(chan struct{}, 1)

func scheduleReload() {
	select {
	case scheduleWake <- struct{}{}:
	default:
	}
}

func (d *daemon) runScheduleEngine() {
	firstRun := true
	lastKey := ""
	var lastFire time.Time
	for {
		if !d.settingBool("schedule.enabled") {
			d.schedSleep(scheduleIdleRecheck)
			continue
		}
		rules := d.loadScheduleRules()
		if len(rules) == 0 {
			d.schedSleep(scheduleIdleRecheck)
			continue
		}
		now := d.schedNow(rules, true)
		idx := scheduleWinner(rules, &now)
		wait := scheduleNextBoundaryWait(rules, now.minute, now.sunrise, now.sunset)
		if idx >= 0 {
			r := rules[idx]
			key := r.set + "\x00" + r.mode
			switch {
			case firstRun && !d.settingBool("schedule.applyOnStart"):
				// Honour the last wallpaper the restore put up; only track it.
				lastKey = key
				lastFire = time.Now()
			case key != lastKey:
				d.scheduleFire(r)
				lastKey = key
				lastFire = time.Now()
			case r.rotateMinutes > 0 && time.Since(lastFire) >= time.Duration(r.rotateMinutes)*time.Minute:
				d.scheduleFire(r)
				lastFire = time.Now()
			}
			if r.rotateMinutes > 0 {
				remaining := r.rotateMinutes - int(time.Since(lastFire).Minutes())
				if remaining < 1 {
					remaining = 1
				}
				if remaining < wait {
					wait = remaining
				}
			}
		} else {
			lastKey = ""
		}
		firstRun = false
		d.schedSleepMinutes(wait)
	}
}

func (d *daemon) schedSleep(dur time.Duration) {
	timer := time.NewTimer(dur)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-scheduleWake:
	}
}

func (d *daemon) schedSleepMinutes(minutes int) {
	if minutes < 1 {
		minutes = 1
	}
	dur := time.Duration(minutes) * time.Minute
	if dur > maxScheduleWait {
		dur = maxScheduleWait
	}
	d.schedSleep(dur)
}

// The RPC passes allowFetch=false so schedule.get never blocks on a forecast fetch.
func (d *daemon) schedNow(rules []scheduleRule, allowFetch bool) scheduleNow {
	t := time.Now()
	_, offsetSec := t.Zone()
	sunrise, sunset := d.sunWindow(t.YearDay(), offsetSec)
	now := scheduleNow{
		year:    t.Year(),
		month:   int(t.Month()),
		day:     t.Day(),
		wday:    int(t.Weekday()),
		minute:  t.Hour()*60 + t.Minute(),
		sunrise: sunrise,
		sunset:  sunset,
	}
	if scheduleUsesWeather(rules) {
		if allowFetch {
			now.weather = d.currentWeather()
		} else {
			now.weather = currentWeatherCached()
		}
	}
	if scheduleUsesPower(rules) {
		b := powerOnBattery()
		now.onBattery = &b
		if pct, ok := batteryPercent(); ok {
			now.batteryPercent = &pct
		}
	}
	if scheduleUsesOutputs(rules) {
		now.outputs = connectedOutputNames()
	}
	return now
}

func (d *daemon) sunWindow(dayOfYear, offsetSec int) (int, int) {
	lat := parseCoord(d.settingString("schedule.latitude"))
	lon := parseCoord(d.settingString("schedule.longitude"))
	if lat == 0 && lon == 0 {
		return 6 * 60, 18 * 60
	}
	sr, ss, ok := sunTimesUTCMin(dayOfYear, lat, lon)
	if !ok {
		return 6 * 60, 18 * 60
	}
	return utcToLocalMin(sr, offsetSec), utcToLocalMin(ss, offsetSec)
}

func utcToLocalMin(utcMin float64, offsetSec int) int {
	local := utcMin + float64(offsetSec)/60.0
	m := int(math.Round(local)) % 1440
	if m < 0 {
		m += 1440
	}
	return m
}

func parseCoord(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return v
}

func connectedOutputNames() []string {
	var names []string
	for _, o := range outputs.list() {
		if o.Name != "" {
			names = append(names, o.Name)
		}
	}
	return names
}

func (d *daemon) scheduleFire(r scheduleRule) {
	// The theme mode goes first so the palette the apply triggers lands in the requested mode.
	if r.mode == "light" || r.mode == "dark" {
		if err := d.setSetting("theme.mode", r.mode); err != nil {
			fmt.Fprintf(os.Stderr, "ryogami: schedule theme mode: %v\n", err)
		}
	}
	set := strings.TrimSpace(r.set)
	// A schedule apply supersedes a playlist the schedule assigned, unless this rule targets it.
	if !strings.HasPrefix(set, "playlist:") {
		d.playlists.unassign("*")
	}
	switch {
	case set == "random":
		types, favOnly := d.randomPoolConfig()
		if r.randomTypes != nil {
			types = r.randomTypes
		}
		if r.randomFav != nil {
			favOnly = *r.randomFav
		}
		if pick := d.pickRandom(types, favOnly); pick != "" {
			_ = d.applyWallpaperReason("schedule", typeOf(pick), pick, "set", nil, nil, nil)
		}
	case strings.HasPrefix(set, "folder:"):
		dir := resolvePath(strings.TrimSpace(strings.TrimPrefix(set, "folder:")))
		if pick := d.pickFromFolder(r, dir); pick != "" {
			_ = d.applyWallpaperReason("schedule", typeOf(pick), pick, "set", nil, nil, nil)
		}
	case strings.HasPrefix(set, "playlist:"):
		if id, err := strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(set, "playlist:")), 10, 64); err == nil {
			d.playlists.assign("*", id)
		}
	case strings.HasPrefix(set, "we:"):
		_ = d.applyWEReason("schedule", strings.TrimSpace(strings.TrimPrefix(set, "we:")), nil, nil, nil)
	default:
		if p := d.resolveSetPath(set); p != "" {
			_ = d.applyWallpaperReason("schedule", typeOf(p), p, "set", nil, nil, nil)
		}
	}
}

func (d *daemon) resolveSetPath(set string) string {
	name := set
	if prefix, rest, ok := strings.Cut(set, ":"); ok && (prefix == "static" || prefix == "video") {
		name = rest
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	for _, key := range []string{name, strings.ReplaceAll(name, "/", "--")} {
		if e, ok := d.store.get(key); ok {
			if e.VideoFile != "" {
				return e.VideoFile
			}
			return filepath.Join(d.config().wallpaperDir(), e.Name)
		}
	}
	if cand := filepath.Join(d.config().wallpaperDir(), name); fileExists(cand) {
		return cand
	}
	if r := resolvePath(name); fileExists(r) {
		return r
	}
	return ""
}

func (d *daemon) pickFromFolder(r scheduleRule, dir string) string {
	if dir == "" {
		return ""
	}
	files := listFolderMedia(dir, r.randomTypes)
	if len(files) == 0 {
		return ""
	}
	if !r.noRepeat {
		return pickAvoidingCurrent(files, d.currentName())
	}
	return folderShown.pick(dir, files, d.currentName())
}

func listFolderMedia(dir string, types []string) []string {
	allowStatic, allowVideo := true, true
	if len(types) > 0 {
		allowStatic = containsStr(types, "static")
		allowVideo = containsStr(types, "video")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(e.Name()), "."))
		switch {
		case imageExts[ext] && allowStatic:
			out = append(out, filepath.Join(dir, e.Name()))
		case videoExts[ext] && allowVideo:
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(out)
	return out
}

func pickAvoidingCurrent(files []string, current string) string {
	if len(files) == 0 {
		return ""
	}
	candidates := files
	if current != "" && len(files) > 1 {
		filtered := make([]string, 0, len(files))
		for _, f := range files {
			if filepath.Base(f) != current {
				filtered = append(filtered, f)
			}
		}
		if len(filtered) > 0 {
			candidates = filtered
		}
	}
	return candidates[rand.IntN(len(candidates))]
}

type schedFolderState struct {
	mu    sync.Mutex
	day   int
	shown map[string]map[string]bool
}

var folderShown = &schedFolderState{shown: map[string]map[string]bool{}}

func (s *schedFolderState) pick(dir string, files []string, current string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	today := time.Now().YearDay()
	if s.day != today {
		s.day = today
		s.shown = map[string]map[string]bool{}
	}
	seen := s.shown[dir]
	if seen == nil {
		seen = map[string]bool{}
		s.shown[dir] = seen
	}
	var candidates []string
	for _, f := range files {
		if !seen[f] {
			candidates = append(candidates, f)
		}
	}
	if len(candidates) == 0 {
		for k := range seen {
			delete(seen, k)
		}
		candidates = files
	}
	if current != "" && len(candidates) > 1 {
		filtered := make([]string, 0, len(candidates))
		for _, f := range candidates {
			if filepath.Base(f) != current {
				filtered = append(filtered, f)
			}
		}
		if len(filtered) > 0 {
			candidates = filtered
		}
	}
	pick := candidates[rand.IntN(len(candidates))]
	seen[pick] = true
	return pick
}

func (d *daemon) scheduleSnapshot() map[string]interface{} {
	return map[string]interface{}{
		"enabled":      d.settingBool("schedule.enabled"),
		"applyOnStart": d.settingBool("schedule.applyOnStart"),
		"latitude":     d.settingString("schedule.latitude"),
		"longitude":    d.settingString("schedule.longitude"),
		"rules":        d.settingList("schedule.rules"),
		"next":         d.scheduleNext(),
	}
}

func (d *daemon) scheduleNext() map[string]interface{} {
	rules := d.loadScheduleRules()
	if len(rules) == 0 {
		return nil
	}
	now := d.schedNow(rules, false)
	wait := scheduleNextBoundaryWait(rules, now.minute, now.sunrise, now.sunset)
	res := map[string]interface{}{
		"waitMinutes": wait,
		"atMinute":    wrapMinute(now.minute + wait),
	}
	if idx := scheduleWinner(rules, &now); idx >= 0 {
		res["ruleIndex"] = idx
		res["ruleName"] = rules[idx].name
		res["set"] = rules[idx].set
		res["mode"] = rules[idx].mode
	}
	return res
}

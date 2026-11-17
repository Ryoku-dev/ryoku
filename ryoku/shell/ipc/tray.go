package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"

	wm "ryoku-wm"
)

// tray.go owns the system tray: the daemon runs the StatusNotifierWatcher
// (falling back to host-only when another watcher already holds the name),
// proxies each StatusNotifierItem, resolves its icon, walks its dbusmenu tree,
// and streams the lot to QML on the "tray" state topic. The bus names, paths,
// interfaces, and signals are the StatusNotifier and dbusmenu specs, reproduced
// as-is; they are third-party protocol, not reference API.
//
// Divergence from the reference (recorded per contract 04): the reference never
// reads an item's attention icon, so an item that asks for attention keeps its
// ordinary icon. Ryoku prefers the attention icon while Status is NeedsAttention
// so the state is actually visible; see resolveTrayIcon.

const (
	watcherName      = "org.kde.StatusNotifierWatcher"
	watcherPath      = "/StatusNotifierWatcher"
	watcherIface     = "org.kde.StatusNotifierWatcher"
	itemIface        = "org.kde.StatusNotifierItem"
	menuIface        = "com.canonical.dbusmenu"
	propsIface       = "org.freedesktop.DBus.Properties"
	trayMenuDebounce = 100 * time.Millisecond // matches the reference menu-refresh debounce
	trayIconTarget   = 24                     // preferred pixmap size, in pixels
)

var errUnknownProp = dbus.NewError("org.freedesktop.DBus.Error.UnknownProperty", nil)

// trayPixmap is one size of an item's ARGB32 icon, as delivered over D-Bus.
type trayPixmap struct {
	W, H int
	Data []byte
}

// trayIconInput is everything the precedence chain needs to choose an icon.
type trayIconInput struct {
	Status           string
	IconName         string
	IconThemePath    string
	IconPixmaps      []trayPixmap
	AttentionName    string
	AttentionPixmaps []trayPixmap
}

// trayIcon is the chosen icon: a theme Name, a ready-to-load file Path, or a
// converted RGBA pixmap the caller persists and references by path.
type trayIcon struct {
	Name string
	Path string
	RGBA []byte
	W, H int
}

type trayTooltip struct {
	IconName    string `json:"iconName,omitempty"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

type trayMenuItem struct {
	ID          int            `json:"id"`
	Label       string         `json:"label,omitempty"`
	Type        string         `json:"type,omitempty"`
	Enabled     bool           `json:"enabled"`
	Visible     bool           `json:"visible"`
	ToggleType  string         `json:"toggleType,omitempty"`
	ToggleState int            `json:"toggleState,omitempty"`
	IconName    string         `json:"iconName,omitempty"`
	Children    []trayMenuItem `json:"children,omitempty"`
}

// trayItem is one tray entry. Exported fields are the QML view; the rest are the
// D-Bus coordinates used to read and drive it.
type trayItem struct {
	Service    string        `json:"service"` // stable key: bus name + object path
	ID         string        `json:"id"`
	Title      string        `json:"title"`
	Status     string        `json:"status"`
	Category   string        `json:"category"`
	IconName   string        `json:"iconName,omitempty"`
	IconPath   string        `json:"iconPath,omitempty"`
	Tooltip    trayTooltip   `json:"tooltip"`
	ItemIsMenu bool          `json:"itemIsMenu"`
	Menu       *trayMenuItem `json:"menu,omitempty"`

	busName   string
	owner     string
	alias     string
	path      dbus.ObjectPath
	menuPath  dbus.ObjectPath
	menuTimer *time.Timer
}

type trayState struct {
	conn      *dbus.Conn
	mu        sync.Mutex
	items     map[string]*trayItem
	order     []string
	topic     *stateTopic
	cacheDir  string
	isWatcher bool
	hostReg   bool
}

// startTray brings the tray watcher/host up, registers the topic and control
// calls, and publishes an empty snapshot. A missing session bus disables the
// tray without failing the daemon.
func (d *daemon) startTray() {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		log.Printf("ryoku-shell: system tray disabled: %v", err)
		return
	}
	t := &trayState{
		conn:     conn,
		items:    map[string]*trayItem{},
		topic:    d.registerTopic("tray"),
		cacheDir: trayCacheDir(),
	}
	d.tray = t
	_ = os.MkdirAll(t.cacheDir, 0o700)

	_ = conn.Export(t, dbus.ObjectPath(watcherPath), watcherIface)
	_ = conn.Export(t, dbus.ObjectPath(watcherPath), propsIface)
	reply, err := conn.RequestName(watcherName, dbus.NameFlagDoNotQueue)
	t.isWatcher = err == nil && reply == dbus.RequestNameReplyPrimaryOwner

	_ = conn.AddMatchSignal(dbus.WithMatchInterface(itemIface))
	_ = conn.AddMatchSignal(dbus.WithMatchInterface(menuIface))
	_ = conn.AddMatchSignal(
		dbus.WithMatchSender("org.freedesktop.DBus"),
		dbus.WithMatchInterface("org.freedesktop.DBus"),
		dbus.WithMatchMember("NameOwnerChanged"),
	)
	if !t.isWatcher {
		_ = conn.AddMatchSignal(dbus.WithMatchInterface(watcherIface))
	}
	go t.watchSignals()

	if t.isWatcher {
		t.mu.Lock()
		t.hostReg = true
		t.mu.Unlock()
		_ = conn.Emit(dbus.ObjectPath(watcherPath), watcherIface+".StatusNotifierHostRegistered")
	} else {
		t.adoptExisting()
	}

	d.registerCall("tray.activate", func(raw json.RawMessage) (any, error) {
		a := trayPoint(raw)
		if id := d.trayWindow(t.itemPid(a.Service)); id != "" {
			return nil, d.wmc.Act(wm.ActionWindowFocus, id)
		}
		return nil, t.itemCall(a.Service, itemIface+".Activate", int32(a.X), int32(a.Y))
	})
	d.registerCall("tray.secondaryActivate", func(raw json.RawMessage) (any, error) {
		a := trayPoint(raw)
		return nil, t.itemCall(a.Service, itemIface+".SecondaryActivate", int32(a.X), int32(a.Y))
	})
	d.registerCall("tray.contextMenu", func(raw json.RawMessage) (any, error) {
		a := trayPoint(raw)
		return nil, t.itemCall(a.Service, itemIface+".ContextMenu", int32(a.X), int32(a.Y))
	})
	d.registerCall("tray.scroll", func(raw json.RawMessage) (any, error) {
		var a struct {
			Service     string `json:"service"`
			Delta       int    `json:"delta"`
			Orientation string `json:"orientation"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		if a.Orientation == "" {
			a.Orientation = "vertical"
		}
		return nil, t.itemCall(a.Service, itemIface+".Scroll", int32(a.Delta), a.Orientation)
	})
	d.registerCall("tray.menuEvent", func(raw json.RawMessage) (any, error) {
		var a struct {
			Service string `json:"service"`
			Item    int    `json:"item"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		return nil, t.menuEvent(a.Service, a.Item)
	})
	d.registerCall("tray.aboutToShow", func(raw json.RawMessage) (any, error) {
		var a struct {
			Service string `json:"service"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		return nil, t.aboutToShow(a.Service)
	})

	t.publish()
}

func trayCacheDir() string {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".cache")
	}
	return filepath.Join(base, "ryoku", "tray")
}

// --- exported D-Bus surface (watcher + properties) ---

// RegisterStatusNotifierItem records an item and starts tracking it. The heavy
// work runs off the method handler so a blocking property read cannot stall the
// bus connection.
func (t *trayState) RegisterStatusNotifierItem(service string, sender dbus.Sender) *dbus.Error {
	busName, path := parseTrayService(service, string(sender))
	if busName == "" || !path.IsValid() {
		return dbus.NewError("org.freedesktop.DBus.Error.InvalidArgs", nil)
	}
	if busName != string(sender) {
		var owner string
		if t.conn.BusObject().Call("org.freedesktop.DBus.GetNameOwner", 0, busName).Store(&owner) != nil || owner != string(sender) {
			return dbus.NewError("org.freedesktop.DBus.Error.AccessDenied", nil)
		}
	}
	t.addItem(service, string(sender))
	return nil
}

func (t *trayState) RegisterStatusNotifierHost(string) *dbus.Error {
	t.mu.Lock()
	t.hostReg = true
	t.mu.Unlock()
	_ = t.conn.Emit(dbus.ObjectPath(watcherPath), watcherIface+".StatusNotifierHostRegistered")
	return nil
}

func (t *trayState) Get(iface, prop string) (dbus.Variant, *dbus.Error) {
	if iface != watcherIface {
		return dbus.Variant{}, errUnknownProp
	}
	switch prop {
	case "RegisteredStatusNotifierItems":
		return dbus.MakeVariant(t.serviceList()), nil
	case "IsStatusNotifierHostRegistered":
		t.mu.Lock()
		defer t.mu.Unlock()
		return dbus.MakeVariant(t.hostReg), nil
	case "ProtocolVersion":
		return dbus.MakeVariant(int32(0)), nil
	}
	return dbus.Variant{}, errUnknownProp
}

func (t *trayState) GetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	if iface != watcherIface {
		return map[string]dbus.Variant{}, nil
	}
	t.mu.Lock()
	host := t.hostReg
	t.mu.Unlock()
	return map[string]dbus.Variant{
		"RegisteredStatusNotifierItems":  dbus.MakeVariant(t.serviceList()),
		"IsStatusNotifierHostRegistered": dbus.MakeVariant(host),
		"ProtocolVersion":                dbus.MakeVariant(int32(0)),
	}, nil
}

func (t *trayState) serviceList() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]string, 0, len(t.order))
	out = append(out, t.order...)
	return out
}

// adoptExisting joins an already-running watcher as a host and pulls its items.
func (t *trayState) adoptExisting() {
	obj := t.conn.Object(watcherName, dbus.ObjectPath(watcherPath))
	if names := t.conn.Names(); len(names) > 0 {
		_ = obj.Call(watcherIface+".RegisterStatusNotifierHost", 0, names[0]).Err
	}
	var services []string
	if v, err := obj.GetProperty(watcherIface + ".RegisteredStatusNotifierItems"); err == nil {
		_ = v.Store(&services)
	}
	for _, svc := range services {
		go t.addItem(svc, "")
	}
}

// --- item lifecycle ---

func (t *trayState) addItem(service, sender string) {
	busName, path := parseTrayService(service, sender)
	if busName == "" || !path.IsValid() {
		return
	}
	owner := t.nameOwner(busName)
	registeredName := busName
	if sender != "" {
		if owner != sender {
			return
		}
		busName = sender
	}
	key := busName + string(path)
	t.mu.Lock()
	if _, ok := t.items[key]; ok {
		t.mu.Unlock()
		return
	}
	it := &trayItem{Service: key, busName: busName, owner: owner, alias: registeredName, path: path}
	t.items[key] = it
	t.order = append(t.order, key)
	t.mu.Unlock()

	if t.isWatcher {
		// hosts need the sender as well as the path to reach the item
		_ = t.conn.Emit(dbus.ObjectPath(watcherPath), watcherIface+".StatusNotifierItemRegistered", key)
	}
	go func() {
		t.refreshItem(it)
		t.refreshMenu(it)
	}()
}

func (t *trayState) removeItemByService(service string) {
	busName, path := parseTrayService(service, "")
	t.removeKey(busName + string(path))
}

func (t *trayState) removeKey(key string) {
	t.mu.Lock()
	it, ok := t.items[key]
	if ok {
		delete(t.items, key)
		for i, k := range t.order {
			if k == key {
				t.order = append(t.order[:i], t.order[i+1:]...)
				break
			}
		}
		if it.menuTimer != nil {
			it.menuTimer.Stop()
		}
	}
	t.mu.Unlock()
	if ok {
		if t.isWatcher {
			_ = t.conn.Emit(dbus.ObjectPath(watcherPath), watcherIface+".StatusNotifierItemUnregistered", key)
		}
		t.publish()
	}
}

// ownerLost drops every item on a bus name that vanished.
func (t *trayState) ownerLost(name string) {
	t.mu.Lock()
	var gone []string
	for key, it := range t.items {
		if it.busName == name || it.owner == name || it.alias == name {
			gone = append(gone, key)
		}
	}
	t.mu.Unlock()
	for _, k := range gone {
		t.removeKey(k)
	}
}

// refreshItem re-reads an item's properties and republishes. The D-Bus read and
// the icon persistence happen off-lock; only the field swap is guarded.
func (t *trayState) refreshItem(it *trayItem) {
	var props map[string]dbus.Variant
	if err := t.conn.Object(it.busName, it.path).Call(propsIface+".GetAll", 0, itemIface).Store(&props); err != nil {
		return
	}
	icon := resolveTrayIcon(trayIconInput{
		Status:           varString(props["Status"]),
		IconName:         varString(props["IconName"]),
		IconThemePath:    varString(props["IconThemePath"]),
		IconPixmaps:      parsePixmaps(props["IconPixmap"]),
		AttentionName:    varString(props["AttentionIconName"]),
		AttentionPixmaps: parsePixmaps(props["AttentionIconPixmap"]),
	}, fileExists)
	iconPath := icon.Path
	if icon.RGBA != nil {
		if enc, ok := rgbaToPNG(icon.RGBA, icon.W, icon.H); ok {
			p := filepath.Join(t.cacheDir, "icon-"+sanitizeKey(it.Service)+".png")
			if os.WriteFile(p, enc, 0o600) == nil {
				iconPath = p
			}
		}
	}
	var menuPath dbus.ObjectPath
	if mv, ok := props["Menu"]; ok {
		if op, ok := mv.Value().(dbus.ObjectPath); ok {
			menuPath = op
		}
	}

	t.mu.Lock()
	it.ID = varString(props["Id"])
	it.Title = varString(props["Title"])
	it.Status = varString(props["Status"])
	it.Category = varString(props["Category"])
	it.ItemIsMenu = varBool(props["ItemIsMenu"])
	it.Tooltip = parseTooltip(props["ToolTip"])
	it.IconName = icon.Name
	it.IconPath = iconPath
	if menuPath != "" {
		it.menuPath = menuPath
	}
	t.mu.Unlock()
	t.publish()
}

// refreshMenu fetches the whole dbusmenu layout and republishes.
func (t *trayState) refreshMenu(it *trayItem) {
	t.mu.Lock()
	bus, mp := it.busName, it.menuPath
	t.mu.Unlock()
	if mp == "" {
		return
	}
	var revision uint32
	var layout any
	if err := t.conn.Object(bus, mp).Call(menuIface+".GetLayout", 0, int32(0), int32(-1), []string{}).Store(&revision, &layout); err != nil {
		return
	}
	menu := parseMenuNode(layout)
	t.mu.Lock()
	it.Menu = menu
	t.mu.Unlock()
	t.publish()
}

// scheduleMenu coalesces menu refreshes so a burst of dbusmenu updates costs one
// fetch after the debounce window.
func (t *trayState) scheduleMenu(it *trayItem) {
	t.mu.Lock()
	if it.menuTimer != nil {
		it.menuTimer.Stop()
	}
	it.menuTimer = time.AfterFunc(trayMenuDebounce, func() { t.refreshMenu(it) })
	t.mu.Unlock()
}

// --- signals ---

func (t *trayState) watchSignals() {
	ch := make(chan *dbus.Signal, 128)
	t.conn.Signal(ch)
	for sig := range ch {
		t.dispatchSignal(sig)
	}
}

func (t *trayState) dispatchSignal(sig *dbus.Signal) {
	switch {
	case sig.Name == "org.freedesktop.DBus.NameOwnerChanged" && sig.Sender == "org.freedesktop.DBus":
		if len(sig.Body) == 3 {
			name, _ := sig.Body[0].(string)
			oldOwner, _ := sig.Body[1].(string)
			newOwner, _ := sig.Body[2].(string)
			if name != "" && oldOwner != "" && oldOwner != newOwner {
				go t.ownerLost(name)
			}
		}
	case !t.isWatcher && strings.HasPrefix(sig.Name, watcherIface+".StatusNotifierItem") && sig.Sender == t.nameOwner(watcherName):
		if len(sig.Body) >= 1 {
			svc, _ := sig.Body[0].(string)
			switch {
			case strings.HasSuffix(sig.Name, "Unregistered"):
				go t.removeItemByService(svc)
			case strings.HasSuffix(sig.Name, "Registered"):
				go t.addItem(svc, "")
			}
		}
	case strings.HasPrefix(sig.Name, itemIface+"."):
		if it := t.itemBySender(sig.Sender); it != nil {
			if strings.HasSuffix(sig.Name, "NewMenu") {
				go t.scheduleMenu(it)
			} else {
				go t.refreshItem(it)
			}
		}
	case strings.HasPrefix(sig.Name, menuIface+"."):
		if it := t.itemBySender(sig.Sender); it != nil {
			go t.scheduleMenu(it)
		}
	}
}

func (t *trayState) itemBySender(sender string) *trayItem {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, it := range t.items {
		if it.owner == sender || it.busName == sender {
			return it
		}
	}
	return nil
}

// --- control ---

type trayPointArgs struct {
	Service string `json:"service"`
	X       int    `json:"x"`
	Y       int    `json:"y"`
}

func trayPoint(raw json.RawMessage) trayPointArgs {
	var a trayPointArgs
	_ = json.Unmarshal(raw, &a)
	return a
}

func (t *trayState) itemCall(service, method string, args ...any) error {
	t.mu.Lock()
	it := t.items[service]
	t.mu.Unlock()
	if it == nil {
		return fmt.Errorf("no tray item %s", service)
	}
	return t.conn.Object(it.busName, it.path).Call(method, 0, args...).Err
}

// itemPid is the process owning a tray item's bus name, 0 when unknown.
func (t *trayState) itemPid(service string) int {
	t.mu.Lock()
	it := t.items[service]
	t.mu.Unlock()
	if it == nil {
		return 0
	}
	name := it.owner
	if name == "" {
		name = it.busName
	}
	var pid uint32
	if t.conn.BusObject().Call("org.freedesktop.DBus.GetConnectionUnixProcessID", 0, name).Store(&pid) != nil {
		return 0
	}
	return int(pid)
}

// trayWindow is the unfocused window a tray click should bring forward. An app
// cannot raise its own window without an activation token, so a window on
// another workspace stayed hidden; a focused window keeps the app's own toggle.
func (d *daemon) trayWindow(pid int) string {
	if pid <= 0 {
		return ""
	}
	d.wmMu.Lock()
	defer d.wmMu.Unlock()
	id, order := "", -1
	for _, w := range d.wmWindows {
		if w.Pid != pid {
			continue
		}
		if w.FocusOrder == 0 {
			return ""
		}
		if order < 0 || w.FocusOrder < order {
			id, order = w.ID, w.FocusOrder
		}
	}
	return id
}

func (t *trayState) menuEvent(service string, id int) error {
	t.mu.Lock()
	it := t.items[service]
	var bus string
	var mp dbus.ObjectPath
	if it != nil {
		bus, mp = it.busName, it.menuPath
	}
	t.mu.Unlock()
	if it == nil || mp == "" {
		return fmt.Errorf("no menu for %s", service)
	}
	return t.conn.Object(bus, mp).Call(menuIface+".Event", dbus.FlagNoReplyExpected,
		int32(id), "clicked", dbus.MakeVariant(int32(0)), uint32(time.Now().Unix())).Err
}

// aboutToShow asks the item's dbusmenu to populate itself before the shell draws
// it, then re-reads the layout so a lazily-built menu (submenus filled only on
// demand) is current. The AboutToShow error is ignored: menus that never
// implement it still refresh from the last GetLayout.
func (t *trayState) aboutToShow(service string) error {
	t.mu.Lock()
	it := t.items[service]
	var bus string
	var mp dbus.ObjectPath
	if it != nil {
		bus, mp = it.busName, it.menuPath
	}
	t.mu.Unlock()
	if it == nil || mp == "" {
		return fmt.Errorf("no menu for %s", service)
	}
	var needUpdate bool
	_ = t.conn.Object(bus, mp).Call(menuIface+".AboutToShow", 0, int32(0)).Store(&needUpdate)
	t.refreshMenu(it)
	return nil
}

func (t *trayState) publish() {
	if t.topic == nil {
		return
	}
	t.mu.Lock()
	items := make([]*trayItem, 0, len(t.order))
	for _, k := range t.order {
		if it := t.items[k]; it != nil {
			items = append(items, it)
		}
	}
	frame, err := json.Marshal(map[string]any{"items": items})
	t.mu.Unlock()
	if err != nil {
		return
	}
	t.topic.publish(frame)
}

func (t *trayState) nameOwner(name string) string {
	if name == "" || strings.HasPrefix(name, ":") {
		return name
	}
	var owner string
	if err := t.conn.BusObject().Call("org.freedesktop.DBus.GetNameOwner", 0, name).Store(&owner); err != nil {
		return name
	}
	return owner
}

// --- pure helpers (unit-tested) ---

// resolveTrayIcon is the icon precedence chain. An icon name resolves against
// the item's own icon_theme_path first (an item-supplied search dir), then the
// ambient theme; failing a name, the pixmap nearest the target size is converted
// ARGB to RGBA; failing that, a generic executable icon. When the item needs
// attention its attention icon is preferred (the Ryoku divergence).
func resolveTrayIcon(in trayIconInput, exists func(string) bool) trayIcon {
	name := in.IconName
	pixmaps := in.IconPixmaps
	if in.Status == "NeedsAttention" {
		if in.AttentionName != "" {
			name = in.AttentionName
		}
		if len(in.AttentionPixmaps) > 0 {
			pixmaps = in.AttentionPixmaps
		}
	}
	if name != "" {
		if in.IconThemePath != "" {
			for _, ext := range []string{"png", "svg", "xpm"} {
				p := filepath.Join(in.IconThemePath, name+"."+ext)
				if exists(p) {
					return trayIcon{Path: p}
				}
			}
		}
		return trayIcon{Name: name}
	}
	if best := selectBestPixmap(pixmaps, trayIconTarget); best != nil {
		return trayIcon{RGBA: argbToRGBA(best.Data), W: best.W, H: best.H}
	}
	return trayIcon{Name: "application-x-executable-symbolic"}
}

// selectBestPixmap picks the pixmap whose dimensions are closest to target,
// skipping any whose byte length cannot hold its claimed size.
func selectBestPixmap(pixmaps []trayPixmap, target int) *trayPixmap {
	var best *trayPixmap
	bestScore := 0
	for i := range pixmaps {
		p := &pixmaps[i]
		if p.W <= 0 || p.H <= 0 || len(p.Data) < p.W*p.H*4 {
			continue
		}
		score := absInt(p.W-target) + absInt(p.H-target)
		if best == nil || score < bestScore {
			best, bestScore = p, score
		}
	}
	return best
}

// argbToRGBA rewrites ARGB32 (network order: A,R,G,B) as RGBA (R,G,B,A).
func argbToRGBA(argb []byte) []byte {
	rgba := make([]byte, len(argb))
	for i := 0; i+3 < len(argb); i += 4 {
		rgba[i] = argb[i+1]   // R
		rgba[i+1] = argb[i+2] // G
		rgba[i+2] = argb[i+3] // B
		rgba[i+3] = argb[i]   // A
	}
	return rgba
}

// parseTrayService splits a registration string into a bus name and object path.
// A bare bus name uses the default item path; a value starting with "/" is a
// path and the bus name is the caller; "name/path" splits at the first slash.
func parseTrayService(service, sender string) (string, dbus.ObjectPath) {
	if service == "" {
		return sender, "/StatusNotifierItem"
	}
	if strings.HasPrefix(service, "/") {
		return sender, dbus.ObjectPath(service)
	}
	if i := strings.IndexByte(service, '/'); i >= 0 {
		return service[:i], dbus.ObjectPath(service[i:])
	}
	return service, "/StatusNotifierItem"
}

func rgbaToPNG(rgba []byte, w, h int) ([]byte, bool) {
	if w <= 0 || h <= 0 || len(rgba) < w*h*4 {
		return nil, false
	}
	img := &image.RGBA{Pix: rgba[:w*h*4], Stride: w * 4, Rect: image.Rect(0, 0, w, h)}
	var buf bytes.Buffer
	if png.Encode(&buf, img) != nil {
		return nil, false
	}
	return buf.Bytes(), true
}

func parsePixmaps(v dbus.Variant) []trayPixmap {
	// godbus decodes a(iiay) as [][]any; tolerate a plain []any of structs too.
	var rows [][]any
	switch arr := v.Value().(type) {
	case [][]any:
		rows = arr
	case []any:
		for _, e := range arr {
			if s, ok := e.([]any); ok {
				rows = append(rows, s)
			}
		}
	default:
		return nil
	}
	var out []trayPixmap
	for _, e := range rows {
		if len(e) != 3 {
			continue
		}
		w, _ := e[0].(int32)
		h, _ := e[1].(int32)
		data, _ := e[2].([]byte)
		out = append(out, trayPixmap{W: int(w), H: int(h), Data: data})
	}
	return out
}

func parseTooltip(v dbus.Variant) trayTooltip {
	s, ok := v.Value().([]any)
	if !ok || len(s) != 4 {
		return trayTooltip{}
	}
	name, _ := s[0].(string)
	title, _ := s[2].(string)
	desc, _ := s[3].(string)
	return trayTooltip{IconName: name, Title: title, Description: desc}
}

func parseMenuNode(v any) *trayMenuItem {
	s, ok := v.([]any)
	if !ok || len(s) != 3 {
		return nil
	}
	id, _ := s[0].(int32)
	props, _ := s[1].(map[string]dbus.Variant)
	children, _ := s[2].([]dbus.Variant)
	node := &trayMenuItem{
		ID:          int(id),
		Label:       varString(props["label"]),
		Type:        varString(props["type"]),
		ToggleType:  varString(props["toggle-type"]),
		IconName:    varString(props["icon-name"]),
		Enabled:     varBoolDefault(props["enabled"], true),
		Visible:     varBoolDefault(props["visible"], true),
		ToggleState: varInt(props["toggle-state"]),
	}
	for _, cv := range children {
		if child := parseMenuNode(cv.Value()); child != nil {
			node.Children = append(node.Children, *child)
		}
	}
	return node
}

func varString(v dbus.Variant) string {
	if s, ok := v.Value().(string); ok {
		return s
	}
	return ""
}

func varBool(v dbus.Variant) bool {
	if b, ok := v.Value().(bool); ok {
		return b
	}
	return false
}

func varBoolDefault(v dbus.Variant, def bool) bool {
	if b, ok := v.Value().(bool); ok {
		return b
	}
	return def
}

func varInt(v dbus.Variant) int {
	switch n := v.Value().(type) {
	case int32:
		return int(n)
	case int:
		return n
	case uint32:
		return int(n)
	}
	return 0
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func sanitizeKey(s string) string {
	return strings.NewReplacer("/", "_", ":", "_", ".", "_").Replace(s)
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

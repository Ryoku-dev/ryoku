package main

import (
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// customOutputMode turns theme.mode into the light or dark the shell's palette run picks for the same wallpaper.
func (d *daemon) customOutputMode(img string) string {
	switch mode := d.settingString("theme.mode"); mode {
	case "light", "dark":
		return mode
	case "sun":
		now := time.Now()
		_, offset := now.Zone()
		sunrise, sunset := d.sunWindow(now.YearDay(), offset)
		if minute := now.Hour()*60 + now.Minute(); minute >= sunrise && minute < sunset {
			return "light"
		}
		return "dark"
	}
	pix, err := decodeImage(img)
	if err != nil || imageLuma(pix) < 0.5 {
		return "dark"
	}
	return "light"
}

// imageLuma is the shell's smart-mode rule: mean BT.601 luma over a 64 by 64 grid, 0 to 1.
func imageLuma(m *image.RGBA) float64 {
	b := m.Bounds()
	if b.Empty() {
		return 0
	}
	sx, sy := max(1, b.Dx()/64), max(1, b.Dy()/64)
	var sum, n float64
	for y := b.Min.Y; y < b.Max.Y; y += sy {
		for x := b.Min.X; x < b.Max.X; x += sx {
			c := m.RGBAAt(x, y)
			sum += 0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)
			n++
		}
	}
	return sum / n / 255
}

func (d *daemon) matugenIntegrationsConfigPath() string {
	return filepath.Join(d.cfg.cacheDir(), "matugen-config.toml")
}

func (d *daemon) enabledIntegrations() []map[string]interface{} {
	rows := []map[string]interface{}{}
	for _, raw := range d.settingList("integrations") {
		m, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if on, ok := m["enabled"].(bool); ok && !on {
			continue
		}
		if strParam(m, "template", "") == "" || strParam(m, "output", "") == "" {
			continue
		}
		rows = append(rows, m)
	}
	return rows
}

func (d *daemon) writeMatugenIntegrationsConfig(rows []map[string]interface{}) (string, error) {
	templateDir := resolvePath("~/.config/matugen/templates")
	var b strings.Builder
	b.WriteString("[config]\nreload_apps = false\n\n")
	for i, m := range rows {
		template := strParam(m, "template", "")
		input := template
		if strings.Contains(template, "/") {
			input = resolvePath(template)
		} else {
			input = filepath.Join(templateDir, template)
		}
		output := strParam(m, "output", "")
		if strings.Contains(output, "/") {
			output = resolvePath(output)
		} else {
			output = filepath.Join(d.cfg.cacheDir(), output)
		}
		name := strParam(m, "name", "")
		safe := sanitizeTOMLKey(name)
		if safe == "" {
			safe = fmt.Sprintf("integration_%d", i)
		}
		fmt.Fprintf(&b, "[templates.%s]\n", safe)
		fmt.Fprintf(&b, "input_path = %q\n", input)
		fmt.Fprintf(&b, "output_path = %q\n", output)
		if reload := strings.TrimSpace(strParam(m, "reload", "")); reload != "" {
			fmt.Fprintf(&b, "post_hook = %q\n", reload)
		}
		b.WriteString("\n")
	}
	path := d.matugenIntegrationsConfigPath()
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func sanitizeTOMLKey(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

func (d *daemon) renderMatugenIntegrations(ev applyEvent) {
	if d.settingString("theme.policy") != "wallpaper" || !d.settingBool("features.matugen") {
		return
	}
	rows := d.enabledIntegrations()
	if len(rows) == 0 {
		return
	}
	config, err := d.writeMatugenIntegrationsConfig(rows)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ryogami: matugen outputs config: %v\n", err)
		return
	}
	img := ev.Path
	if ev.Type == "video" {
		if e, ok := d.store.get(ev.Key); ok && e.Thumb != "" {
			img = e.Thumb
		}
	}
	if img == "" {
		return
	}
	command := d.settingString("externalMatugenCommand")
	if strings.TrimSpace(command) == "" {
		return
	}
	mode := d.customOutputMode(img)
	scheme := d.settingString("matugen.schemeType")
	if scheme == "" {
		scheme = "scheme-tonal-spot"
	}
	index := strconv.Itoa(int(d.settingNumber("matugen.colorIndex")))
	repl := strings.NewReplacer(
		"%config%", config,
		"%path%", img,
		"%scheme%", scheme,
		"%mode%", mode,
		"%index%", index,
	)
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return
	}
	args := make([]string, len(fields))
	for i, f := range fields {
		args[i] = repl.Replace(f)
	}
	if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "ryogami: matugen outputs render: %v\n", err)
	}
}

func init() {
	afterApply(func(d *daemon, ev applyEvent) { d.renderMatugenIntegrations(ev) })
}

package main

import (
	"fmt"
	"path/filepath"
)

func stopTargets(outputs []string) []string {
	if len(outputs) == 0 || contains(outputs, "*") {
		return nil
	}
	return outputs
}

func paperAudioTargets(weOutputs []string) []string {
	if contains(weOutputs, "*") {
		return nil
	}
	return weOutputs
}

func newWeAssignment(itemDir, fill string, outputs []string, mute bool, volume int, props map[string]interface{}) paperAssignment {
	src := paperSource{Kind: "we", Path: itemDir}
	if len(props) > 0 {
		src.Properties = props
	}
	return paperAssignment{
		Outputs:  outputs,
		Source:   src,
		FillMode: fill,
		Mute:     mute,
		Volume:   clampVolume(volume),
		Layer:    "background",
	}
}

func weFillMode(fit string) string {
	switch fit {
	case "fill", "fit", "stretch", "center", "tile", "span":
		return fit
	default:
		return "fill"
	}
}

func (d *daemon) buildWeApply(itemDir, fill string, outputs []string, mute map[string]bool, volume map[string]int, props map[string]interface{}, policy interface{}) paperApplyRequest {
	def := wallAudioDefaults()
	var assignments []paperAssignment
	if len(outputs) == 0 || contains(outputs, "*") {
		m, vol := effectiveAudio("*", mute, volume, def)
		assignments = append(assignments, newWeAssignment(itemDir, fill, []string{"*"}, m, vol, props))
	} else {
		for _, out := range outputs {
			m, vol := effectiveAudio(out, mute, volume, def)
			assignments = append(assignments, newWeAssignment(itemDir, fill, []string{out}, m, vol, props))
		}
	}
	return paperApplyRequest{Assignments: assignments, Policy: policy}
}

func (d *daemon) applyWE(weID string, outputs []string, mute map[string]bool, volume map[string]int) error {
	return d.applyWEReason("user", weID, outputs, mute, volume)
}

func (d *daemon) applyWEReason(reason, weID string, outputs []string, mute map[string]bool, volume map[string]int) error {
	if !d.featureSteam() {
		return fmt.Errorf("Wallpaper Engine support is disabled")
	}
	if !validWeID(weID) {
		return fmt.Errorf("invalid Wallpaper Engine id: %s", weID)
	}
	itemDir := filepath.Join(d.workshop.workshopDir(), weID)
	if _, err := validateProject(itemDir); err != nil {
		return fmt.Errorf("workshop item %s: %w", weID, err)
	}

	req := &applyRequest{Type: "we", Path: itemDir, WeID: weID, Outputs: outputs, Reason: reason}
	pickOnly := false
	if err := d.runApplyGuards(req); err != nil {
		if err != errPickOnly {
			return err
		}
		// Pick-only: an external setter paints, so only record the choice and run the hooks.
		pickOnly = true
	}
	wantedAll := len(outputs) == 0
	outputs = req.Outputs
	if !wantedAll && len(outputs) == 0 {
		return nil
	}

	if !pickOnly {
		if err := d.driveWE(weID, itemDir, outputs, mute, volume); err != nil {
			return err
		}
	}

	key := "we:" + weID
	name := weID
	if e, ok := d.store.get(key); ok && e.Name != "" {
		name = e.Name
	}
	d.setCurrent(name)
	d.saveOutputs(outputs, "we", itemDir, mute, volume)
	d.store.mutate(key, func(e *Entry) { e.ApplyCount++ })
	d.broadcast("ryogami.wall.applied", map[string]interface{}{
		"type": "we", "name": name, "path": itemDir, "we_id": weID, "key": key,
	})
	d.runAfterApply(applyEvent{applyRequest: *req, Name: name, Key: key})
	return nil
}

// No guards, persistence or hooks: restore and live property edits repaint through it.
func (d *daemon) driveWE(weID, itemDir string, outputs []string, mute map[string]bool, volume map[string]int) error {
	fit := contentFit()
	// skwd-paper owns the screen from here on: drop any ryogami-live player on
	// these outputs, or its background surface keeps committing underneath the
	// scene and the two producers fight over the output.
	d.video.StopOutputs(stopTargets(outputs))
	props := d.workshop.props.overrides(weID)
	policy := d.buildRendererPolicy()
	if policy != nil {
		fps := uint32(d.workshop.sceneFps(weID))
		if policy.Scene == nil {
			policy.Scene = &paperScenePolicyFull{}
		}
		policy.Scene.Fps = &fps
	}
	fill := weFillMode(d.weSceneFillMode())
	if err := d.paper.apply(d.buildWeApply(itemDir, fill, outputs, mute, volume, props, policy)); err != nil {
		return err
	}
	d.beginPaint(outputs, "we:"+weID)
	still := d.weStill(weID, itemDir)
	if len(outputs) == 0 || contains(outputs, "*") {
		d.surface.show(still, fit, nil, true, false, videoClip{})
	} else {
		for _, out := range outputs {
			d.surface.showOutput(out, still, fit, nil, true, false, videoClip{})
		}
	}
	return nil
}

func (d *daemon) weStill(weID, itemDir string) string {
	if e, ok := d.store.get("we:" + weID); ok {
		if e.Thumb != "" && fileExists(e.Thumb) {
			return e.Thumb
		}
		if e.Preview != "" && fileExists(e.Preview) {
			return e.Preview
		}
	}
	if project, err := resolveProject(itemDir); err == nil {
		if p := findPreview(itemDir, project.document); p != "" {
			return p
		}
	}
	return ""
}

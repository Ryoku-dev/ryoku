package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"

	"ryoku-cli/internal/sys"
	i18n "ryoku-i18n"
)

// Screen gestures have no native equivalent. Only clipboard OCR languages move;
// the retired settings remain in a private backup, never become Oni permissions.
func reconcileRetiredVisionSettings(checkOnly bool) recResult {
	path := filepath.Join(sys.ConfigHome(), "ryoku", "shell.json")
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return okRes(i18n.T("no shell.json yet"))
	}
	if err != nil {
		return failRes(i18n.T("could not read %s: %v"), path, err)
	}
	top, changed, err := retireVisionSettings(raw)
	if err != nil {
		return failRes(i18n.T("retired screen settings could not be migrated: %v"), err)
	}
	if !changed {
		return okRes(i18n.T("no retired screen gesture settings remain"))
	}
	if checkOnly {
		return wouldRes(i18n.T("retired screen gesture settings remain")).
			withFix(i18n.T("ryoku doctor backs them up and preserves clipboard OCR languages without enabling Oni"))
	}
	info, err := os.Stat(path)
	if err != nil {
		return failRes(i18n.T("could not read %s: %v"), path, err)
	}
	out, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return failRes(i18n.T("could not encode %s: %v"), path, err)
	}
	backup, err := os.CreateTemp(filepath.Dir(path), "shell.json.before-oni-vision-*")
	if err != nil {
		return failRes(i18n.T("could not back up %s: %v"), path, err)
	}
	_, writeErr := backup.Write(raw)
	closeErr := backup.Close()
	if writeErr != nil {
		return failRes(i18n.T("could not write %s: %v"), backup.Name(), writeErr)
	}
	if closeErr != nil {
		return failRes(i18n.T("could not write %s: %v"), backup.Name(), closeErr)
	}
	if err := replaceFile(path, append(out, '\n'), info.Mode().Perm()); err != nil {
		return failRes(i18n.T("could not write %s: %v"), path, err)
	}
	return fixedRes(i18n.T("retired screen settings backed up to %s; clipboard OCR languages preserved"), backup.Name())
}

func retireVisionSettings(raw []byte) (map[string]json.RawMessage, bool, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, false, err
	}
	old, present := top["sight"]
	if !present {
		return top, false, nil
	}
	var retired map[string]json.RawMessage
	if err := json.Unmarshal(old, &retired); err != nil {
		return nil, false, err
	}
	if languages, present := retired["ocrLanguages"]; present {
		current, err := rawLeaf(top, "clipboard", "ocrLanguages")
		if err != nil {
			return nil, false, err
		}
		if current == nil {
			if err := setRawLeaf(top, languages, "clipboard", "ocrLanguages"); err != nil {
				return nil, false, err
			}
		}
	}
	delete(top, "sight")
	return top, true, nil
}

package main

func (d *daemon) dispatchTheme(req *request, p map[string]interface{}) (response, bool) {
	switch req.Method {
	case "theme.apps":
		return ok(req.ID, d.themeAppsResult()), true
	case "theme.previews":
		return ok(req.ID, themePreviews()), true
	case "theme.designer.load":
		return ok(req.ID, d.designerLoad(p)), true
	case "theme.designer.save":
		res, err := d.designerSave(p)
		if err != nil {
			return errResp(req.ID, 5, err.Error()), true
		}
		return ok(req.ID, res), true
	case "theme.designer.delete":
		res, err := d.designerDelete(p)
		if err != nil {
			return errResp(req.ID, 5, err.Error()), true
		}
		return ok(req.ID, res), true
	}
	return response{}, false
}

func (d *daemon) dispatchPalette(req *request, p map[string]interface{}) (response, bool) {
	switch req.Method {
	case "palette.preview":
		res, err := d.palettePreview(p)
		if err != nil {
			return errResp(req.ID, 3, err.Error()), true
		}
		return ok(req.ID, res), true
	case "palette.previewColors":
		res, err := d.palettePreviewColors(p)
		if err != nil {
			return errResp(req.ID, 3, err.Error()), true
		}
		return ok(req.ID, res), true
	case "palette.hover":
		res, err := d.paletteHover(p)
		if err != nil {
			return errResp(req.ID, 3, err.Error()), true
		}
		return ok(req.ID, res), true
	}
	return response{}, false
}

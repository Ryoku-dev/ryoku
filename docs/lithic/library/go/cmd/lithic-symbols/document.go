package main

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// document is the part of a symbol SVG that draws: its square view box and the path data of each path element.
type document struct {
	viewBox [4]float64
	paths   []string
}

// parseDocument accepts only an svg root holding plain filled paths and refuses anything it would draw wrongly.
func parseDocument(data []byte) (document, error) {
	var doc document
	decoder := xml.NewDecoder(bytes.NewReader(data))
	root := false
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return doc, fmt.Errorf("parse svg: %w", err)
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		switch start.Name.Local {
		case "svg":
			if root {
				return doc, errors.New("nested <svg>")
			}
			root = true
			view, err := readViewBox(start.Attr)
			if err != nil {
				return doc, err
			}
			doc.viewBox = view
		case "path":
			if !root {
				return doc, errors.New("<path> outside <svg>")
			}
			data, err := readPathData(start.Attr)
			if err != nil {
				return doc, err
			}
			doc.paths = append(doc.paths, data)
		default:
			return doc, fmt.Errorf("unsupported element <%s>", start.Name.Local)
		}
	}
	if !root || len(doc.paths) == 0 {
		return doc, errors.New("no <svg> with a <path>")
	}
	return doc, nil
}

// readViewBox falls back to 0 0 width height, as SVG does, when the root has no viewBox.
func readViewBox(attributes []xml.Attr) ([4]float64, error) {
	var view [4]float64
	var viewBox, width, height string
	for _, attribute := range attributes {
		switch attribute.Name.Local {
		case "xmlns":
		case "viewBox":
			viewBox = attribute.Value
		case "width":
			width = attribute.Value
		case "height":
			height = attribute.Value
		default:
			return view, fmt.Errorf("unsupported <svg> attribute %s", attribute.Name.Local)
		}
	}
	if viewBox == "" {
		viewBox = "0 0 " + width + " " + height
	}
	fields := strings.Fields(strings.ReplaceAll(viewBox, ",", " "))
	if len(fields) != 4 {
		return view, fmt.Errorf("view box %q", viewBox)
	}
	for index, field := range fields {
		value, err := strconv.ParseFloat(field, 64)
		if err != nil {
			return view, fmt.Errorf("view box %q: %w", viewBox, err)
		}
		view[index] = value
	}
	if view[2] <= 0 || view[2] != view[3] {
		return view, fmt.Errorf("view box %q is not a square", viewBox)
	}
	return view, nil
}

// readPathData refuses fill rules, transforms and opacity, which the IconVG output would not carry.
func readPathData(attributes []xml.Attr) (string, error) {
	data := ""
	for _, attribute := range attributes {
		if attribute.Name.Local != "d" {
			return "", fmt.Errorf("unsupported <path> attribute %s", attribute.Name.Local)
		}
		data = attribute.Value
	}
	if data == "" {
		return "", errors.New("<path> without data")
	}
	return data, nil
}

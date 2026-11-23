package engine

import (
	"net/url"
	"path"
	"path/filepath"
	"strings"

	i18n "ryoku-i18n"

	"ryoget/internal/httpdl"
)

const (
	typeVideo     = "video"
	typeMusic     = "music"
	typeDocuments = "documents"
	typePrograms  = "programs"
	typeArchives  = "archives"
	typePictures  = "pictures"
)

// typeExts is ported from prism-download-manager crates/prism-core/src/categories.rs (MIT)
var typeExts = map[string][]string{
	typeVideo: {
		"3gp", "asf", "avi", "flv", "m2ts", "m4v", "mkv", "mov", "mp4", "mpe", "mpeg", "mpg",
		"ogv", "qt", "rm", "rmvb", "ts", "vob", "webm", "wmv",
	},
	typeMusic: {
		"aac", "aif", "aiff", "alac", "ape", "flac", "m4a", "mid", "mp3", "mpa", "oga", "ogg",
		"opus", "ra", "wav", "wma",
	},
	typeDocuments: {
		"csv", "doc", "docx", "epub", "md", "odp", "ods", "odt", "pdf", "pps", "ppt", "pptx",
		"rtf", "txt", "xls", "xlsx", "xps",
	},
	typePrograms: {
		"apk", "appimage", "bin", "deb", "dmg", "exe", "flatpak", "jar", "msi", "msix", "msu",
		"pkg", "rpm", "run", "snap",
	},
	typeArchives: {
		"7z", "ace", "arj", "bz2", "cab", "gz", "gzip", "img", "iso", "lz", "lzh", "lzma",
		"r00", "r01", "rar", "sit", "sitx", "tar", "tbz", "tgz", "txz", "xz", "z", "zip",
		"zst",
	},
	typePictures: {
		"avif", "bmp", "gif", "heic", "ico", "jpeg", "jpg", "png", "psd", "raw", "svg", "tif",
		"tiff", "webp",
	},
}

var extType = func() map[string]string {
	m := map[string]string{}
	for kind, exts := range typeExts {
		for _, x := range exts {
			m[x] = kind
		}
	}
	return m
}()

// typeFolder names a kind's folder in the reader's language; "" keeps the file in the base folder.
func typeFolder(kind string) string {
	var name string
	switch kind {
	case typeVideo:
		name = i18n.T("Videos")
	case typeMusic:
		name = i18n.T("Music")
	case typeDocuments:
		name = i18n.T("Documents")
	case typePrograms:
		name = i18n.T("Programs")
	case typeArchives:
		name = i18n.T("Archives")
	case typePictures:
		name = i18n.T("Pictures")
	}
	return httpdl.Sanitize(name)
}

// linkType classifies a web link by the extension of its path.
func linkType(link string) string {
	u, err := url.Parse(link)
	if err != nil {
		return ""
	}
	ext := strings.TrimPrefix(strings.ToLower(path.Ext(u.Path)), ".")
	return extType[ext]
}

// sortedDir is where a download of this kind lands when downloads are sorted by type under base.
func sortedDir(base, kind, mode, link string) string {
	var t string
	switch kind {
	case "torrent":
		return base
	case "media":
		t = typeVideo
		if mode == "audio" {
			t = typeMusic
		}
	default:
		t = linkType(link)
	}
	if f := typeFolder(t); f != "" && f != "." && f != ".." {
		return filepath.Join(base, f)
	}
	return base
}

// placeByKind refiles a type-sorted task once resolving tells its kind; call it with the lock held
func (e *Engine) placeByKind(t *task) {
	if t.Sort == "" {
		return
	}
	dir := sortedDir(t.Sort, t.Kind, t.Mode, t.URL)
	if dir == t.Dir {
		return
	}
	t.Dir = dir
	if t.Name != "" {
		t.Name = httpdl.Unique(dir, t.Name, e.nameTaken(dir, t.ID))
		t.Path = filepath.Join(dir, t.Name)
	}
}

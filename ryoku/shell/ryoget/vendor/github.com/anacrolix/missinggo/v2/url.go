package missinggo

import (
	"net/url"
	"path"
)

// Returns URL opaque as an unrooted path.
func URLOpaquePath(u *url.URL) string {
	if u.Opaque != "" {
		return u.Opaque
	}
	return u.Path
}

// Cleans the (absolute) URL path, removing unnecessary . and .. elements. See
// "net/http".cleanPath.
func CleanURLPath(p string) string {
	if p == "" {
		return "/"
	}
	if p[0] != '/' {
		p = "/" + p
	}
	cp := path.Clean(p)
	// Add the trailing slash back, as it's relevant to a URL.
	if p[len(p)-1] == '/' && cp != "/" {
		cp += "/"
	}
	return cp
}

func URLJoinSubPath(base, rel string) string {
	baseURL, err := url.Parse(base)
	if err != nil {
		// Honey badger doesn't give a fuck.
		panic(err)
	}
	rel = CleanURLPath(rel)
	baseURL.Path = path.Join(baseURL.Path, rel)
	return baseURL.String()
}

// This exists because it's nontrivial to get everything after the scheme in a URL. You can't just
// use URL.Opaque because path handling kicks in if there's a slash after the scheme's colon.
func PopScheme(u *url.URL) (scheme string, popped string) {
	// We can copy just the part we modify. We can't modify it in place because the caller could pass
	// through directly from the app arguments, and they don't know how deep our mutation goes.
	uCopy := *u
	scheme = uCopy.Scheme
	uCopy.Scheme = ""
	popped = uCopy.String()
	return
}

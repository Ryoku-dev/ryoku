package ryokuapps

import (
	"strings"

	"ryoku-cli/internal/host"
)

// desktopRoot is the package whose dependency closure is the Ryoku desktop.
const desktopRoot = "ryoku-desktop"

// alwaysProtected stay installed whatever the closure says: a checkout box has
// no ryoku-desktop package, and these keep the desktop and these verbs working.
var alwaysProtected = map[string]bool{
	desktopRoot: true, "gpk": true, "polkit": true, "sudo": true,
	"pacman": true, "dnf": true, "dnf5": true, "rpm": true, "xbps": true,
}

// protection is the set of packages a removal must not take: ryoku-desktop,
// everything it depends on, its compositor variants, and alwaysProtected.
type protection map[string]bool

func (p protection) covers(name string) bool {
	return p[name] || alwaysProtected[name] || strings.HasPrefix(name, desktopRoot+"-")
}

// protectedPackages reads the installed dependency closure of ryoku-desktop
// from the package manager's own database. A query that fails leaves only the
// fixed set, never an empty guard.
func protectedPackages(manager host.PackageManager, query queryFunc) protection {
	switch manager {
	case host.Pacman:
		out, _ := query("pacman", "-Qi")
		return closure(parsePacmanDatabase(out))
	case host.DNF:
		provides, _ := query("rpm", "-qa", "--qf", "[%{NAME}\t%{PROVIDENAME}\n]")
		requires, _ := query("rpm", "-qa", "--qf", "[%{NAME}\t%{REQUIRENAME}\n]")
		return closure(parseRPMPairs(provides, requires))
	case host.XBPS:
		out, code := query("xbps-query", "--fulldeptree", "-x", desktopRoot)
		set := protection{}
		if code != 0 {
			return set
		}
		set[desktopRoot] = true
		for _, line := range strings.Fields(out) {
			set[xbpsName(line)] = true
		}
		return set
	}
	return protection{}
}

// database is what the closure walk needs: each package's dependency names and
// which packages provide each name.
type database struct {
	depends  map[string][]string
	provides map[string][]string
}

func newDatabase() database {
	return database{depends: map[string][]string{}, provides: map[string][]string{}}
}

func (d database) addPackage(name string) {
	if _, ok := d.depends[name]; !ok {
		d.depends[name] = nil
	}
	d.provides[name] = appendUnique(d.provides[name], name)
}

func closure(db database) protection {
	set := protection{}
	if _, installed := db.depends[desktopRoot]; !installed {
		return set
	}
	queue := []string{desktopRoot}
	set[desktopRoot] = true
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		for _, dep := range db.depends[name] {
			for _, provider := range db.provides[dep] {
				if !set[provider] {
					set[provider] = true
					queue = append(queue, provider)
				}
			}
		}
	}
	return set
}

// parsePacmanDatabase reads `LC_ALL=C pacman -Qi` for every installed package.
func parsePacmanDatabase(out string) database {
	db := newDatabase()
	name, key := "", ""
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "" {
			name, key = "", ""
			continue
		}
		value := line
		if line[0] != ' ' && line[0] != '\t' {
			k, v, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}
			key, value = strings.TrimSpace(k), v
		}
		switch key {
		case "Name":
			name = strings.TrimSpace(value)
			db.addPackage(name)
		case "Provides", "Depends On":
			if name == "" {
				continue
			}
			for _, field := range strings.Fields(value) {
				if field == "None" {
					continue
				}
				dep := dependencyName(field)
				if key == "Provides" {
					db.provides[dep] = appendUnique(db.provides[dep], name)
				} else {
					db.depends[name] = appendUnique(db.depends[name], dep)
				}
			}
		}
	}
	return db
}

// parseRPMPairs reads rpm's NAME<TAB>PROVIDENAME and NAME<TAB>REQUIRENAME
// lines. File and rich requirements name no package and are skipped.
func parseRPMPairs(provides, requires string) database {
	db := newDatabase()
	for _, line := range strings.Split(provides, "\n") {
		name, provided, ok := strings.Cut(line, "\t")
		if !ok || name == "" {
			continue
		}
		db.addPackage(name)
		provided = dependencyName(strings.TrimSpace(provided))
		db.provides[provided] = appendUnique(db.provides[provided], name)
	}
	for _, line := range strings.Split(requires, "\n") {
		name, required, ok := strings.Cut(line, "\t")
		required = strings.TrimSpace(required)
		if !ok || name == "" || required == "" || strings.HasPrefix(required, "/") ||
			strings.HasPrefix(required, "(") || strings.HasPrefix(required, "rpmlib(") {
			continue
		}
		db.depends[name] = appendUnique(db.depends[name], dependencyName(required))
	}
	return db
}

// dependencyName drops a version constraint: "glibc>=2.40" is "glibc".
func dependencyName(field string) string {
	if cut := strings.IndexAny(field, "<>="); cut > 0 {
		field = field[:cut]
	}
	return strings.TrimSpace(field)
}

// xbpsName is the package name of an XBPS pkgver ("name-1.2_3") or pattern.
func xbpsName(pkgver string) string {
	if cut := strings.IndexAny(pkgver, "<>="); cut > 0 {
		return pkgver[:cut]
	}
	if cut := strings.LastIndexByte(pkgver, '-'); cut > 0 {
		return pkgver[:cut]
	}
	return pkgver
}

func appendUnique(list []string, value string) []string {
	for _, have := range list {
		if have == value {
			return list
		}
	}
	return append(list, value)
}

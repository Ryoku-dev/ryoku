package host

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type packageMapping struct {
	Names   []string
	Special string
	Note    string
}

type packageTable map[string]packageMapping

func readPackageTable(path string) (packageTable, error) {
	return readPackageTableFor(path, "void")
}

func readPackageTableFor(path, distribution string) (packageTable, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	table := packageTable{}
	scanner := bufio.NewScanner(file)
	lineNo := 0
	header := false
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if !header {
			header = true
			if len(fields) != 4 || fields[0] != "arch" || fields[1] != distribution || fields[2] != "lanes" || fields[3] != "notes" {
				return nil, fmt.Errorf("%s:%d: invalid header", path, lineNo)
			}
			continue
		}
		if len(fields) != 4 || fields[0] == "" || fields[1] == "" || fields[2] == "" {
			return nil, fmt.Errorf("%s:%d: malformed package row", path, lineNo)
		}
		if _, exists := table[fields[0]]; exists {
			return nil, fmt.Errorf("%s:%d: duplicate package %s", path, lineNo, fields[0])
		}
		mapping := packageMapping{Note: strings.TrimSpace(fields[3])}
		switch fields[1] {
		case "@repo":
			if mapping.Note == "" {
				return nil, fmt.Errorf("%s:%d: special mapping needs notes", path, lineNo)
			}
			mapping.Names = []string{fields[0]}
		case "@fetch":
			return nil, fmt.Errorf("%s:%d: @fetch mappings are retired", path, lineNo)
		case "-":
			mapping.Special = "-"
		default:
			mapping.Names = strings.Fields(fields[1])
			if len(mapping.Names) == 0 {
				return nil, fmt.Errorf("%s:%d: empty package mapping", path, lineNo)
			}
		}
		table[fields[0]] = mapping
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if !header {
		return nil, fmt.Errorf("%s: missing header", path)
	}
	return table, nil
}

func (a *App) packageTable(path, distribution string) (packageTable, error) {
	table, err := readPackageTableFor(path, distribution)
	if os.IsNotExist(err) {
		fmt.Fprintf(a.cfg.Stderr, "ryoku-host: warning: %s missing; using identity package names\n", path)
		return packageTable{}, nil
	}
	return table, err
}

func (a *App) xbpsTable() (packageTable, error) {
	path := a.getenv("RYOKU_HOST_PKG_TABLE")
	if path == "" {
		path = a.cfg.PackageTable
	}
	return a.packageTable(path, "void")
}

func translatePackage(table packageTable, name string) packageMapping {
	if mapped, ok := table[name]; ok {
		return mapped
	}
	return packageMapping{Names: []string{name}}
}

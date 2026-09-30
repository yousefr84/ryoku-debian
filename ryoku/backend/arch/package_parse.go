package arch

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	backend "ryoku-backend"
)

func parsePackageInfo(output []byte) ([]backend.Package, error) {
	var packages []backend.Package
	fields := make(map[string]string)
	flush := func() error {
		name := fields["Name"]
		if name == "" {
			fields = make(map[string]string)
			return nil
		}
		size, err := parseSize(fields["Installed Size"])
		if err != nil {
			return fmt.Errorf("package %s installed size: %w", name, err)
		}
		packages = append(packages, backend.Package{
			Name:          name,
			Version:       fields["Version"],
			Architecture:  fields["Architecture"],
			Source:        packageSource(fields["Repository"], fields["Installed From"]),
			InstalledSize: size,
		})
		fields = make(map[string]string)
		return nil
	}

	for _, line := range strings.Split(string(output), "\n") {
		if strings.TrimSpace(line) == "" {
			if err := flush(); err != nil {
				return nil, err
			}
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		fields[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return packages, nil
}

func parseSize(value string) (int64, error) {
	if value == "" || value == "None" {
		return 0, nil
	}
	parts := strings.Fields(value)
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid value %q", value)
	}
	amount, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid value %q", value)
	}
	multipliers := map[string]float64{
		"B":   1,
		"KiB": 1024,
		"MiB": 1024 * 1024,
		"GiB": 1024 * 1024 * 1024,
		"TiB": 1024 * 1024 * 1024 * 1024,
	}
	multiplier, ok := multipliers[parts[1]]
	if !ok {
		return 0, fmt.Errorf("invalid unit %q", parts[1])
	}
	return int64(math.Round(amount * multiplier)), nil
}

func parseNames(output []byte) map[string]bool {
	names := make(map[string]bool)
	for _, line := range strings.Split(string(output), "\n") {
		name := strings.TrimSpace(line)
		if name != "" && !strings.HasPrefix(name, "error:") {
			names[name] = true
		}
	}
	return names
}

func packagesByName(packages []backend.Package) map[string]backend.Package {
	byName := make(map[string]backend.Package, len(packages))
	for _, pkg := range packages {
		byName[pkg.Name] = pkg
	}
	return byName
}

func parseOwner(output []byte) (backend.Package, error) {
	const separator = " is owned by "
	line := strings.TrimSpace(string(output))
	_, owner, ok := strings.Cut(line, separator)
	if !ok {
		return backend.Package{}, fmt.Errorf("invalid value %q", line)
	}
	parts := strings.Fields(owner)
	if len(parts) != 2 {
		return backend.Package{}, fmt.Errorf("invalid value %q", line)
	}
	return backend.Package{
		Name:    parts[0],
		Version: parts[1],
		Source:  backend.PackageSourceUnknown,
	}, nil
}

func packageSource(repository, installedFrom string) backend.PackageSource {
	switch {
	case repository == "ryoku" || installedFrom == "ryoku":
		return backend.PackageSourceRyoku
	case installedFrom == "None":
		return backend.PackageSourceSupplemental
	case repository != "" || installedFrom != "":
		return backend.PackageSourceDistribution
	default:
		return backend.PackageSourceUnknown
	}
}

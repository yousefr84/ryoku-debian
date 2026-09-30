package arch

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	backend "ryoku-backend"
)

// PackageFacts is the Arch read-only package facts adapter.
type PackageFacts struct {
	runner CommandRunner
}

// NewPackageFacts builds an Arch package facts adapter. A nil runner selects
// the host command executor; tests and embedding applications may inject one.
func NewPackageFacts(runner CommandRunner) *PackageFacts {
	if runner == nil {
		runner = execRunner{}
	}
	return &PackageFacts{runner: runner}
}

func (f *PackageFacts) Inventory(ctx context.Context) ([]backend.PackageState, error) {
	output, err := f.run(ctx, "pacman", "-Qi")
	if err != nil {
		return nil, err
	}
	packages, err := parsePackageInfo(output)
	if err != nil {
		return nil, fmt.Errorf("parse installed package inventory: %w", err)
	}
	if len(packages) == 0 {
		return []backend.PackageState{}, nil
	}

	explicit, err := f.nameSet(ctx, "-Qqe")
	if err != nil {
		return nil, err
	}

	states := make([]backend.PackageState, 0, len(packages))
	for _, pkg := range packages {
		states = append(states, backend.PackageState{
			Package:           pkg,
			Installed:         true,
			ManuallyInstalled: explicit[pkg.Name],
		})
	}
	return states, nil
}

func (f *PackageFacts) Query(ctx context.Context, names []string) ([]backend.PackageState, error) {
	if len(names) == 0 {
		return []backend.PackageState{}, nil
	}

	installedOutput, installedErr := f.runner.Run(ctx, "pacman", append([]string{"-Qi"}, names...)...)
	if installedErr != nil && !onlyPackageNotFound(installedOutput) {
		return nil, commandError("pacman", append([]string{"-Qi"}, names...), installedOutput, installedErr)
	}
	installed, err := parsePackageInfo(installedOutput)
	if err != nil {
		return nil, fmt.Errorf("parse installed package query: %w", err)
	}

	explicitOutput, explicitErr := f.runner.Run(ctx, "pacman", "-Qqe")
	if explicitErr != nil {
		return nil, commandError("pacman", []string{"-Qqe"}, explicitOutput, explicitErr)
	}
	explicit := parseNames(explicitOutput)

	availableOutput, availableErr := f.runner.Run(ctx, "pacman", append([]string{"-Si"}, names...)...)
	if availableErr != nil && !onlyPackageNotFound(availableOutput) {
		return nil, commandError("pacman", append([]string{"-Si"}, names...), availableOutput, availableErr)
	}
	available, err := parsePackageInfo(availableOutput)
	if err != nil {
		return nil, fmt.Errorf("parse available package query: %w", err)
	}

	installedByName := packagesByName(installed)
	availableByName := packagesByName(available)
	states := make([]backend.PackageState, 0, len(names))
	for _, name := range names {
		state := backend.PackageState{Package: backend.Package{Name: name}}
		if pkg, ok := installedByName[name]; ok {
			state.Package = pkg
			state.Installed = true
			state.ManuallyInstalled = explicit[name]
		}
		if pkg, ok := availableByName[name]; ok {
			state.AvailableVersion = pkg.Version
			if !state.Installed {
				state.Package = pkg
			}
		}
		states = append(states, state)
	}
	return states, nil
}

func (f *PackageFacts) Available(ctx context.Context, names []string) ([]backend.Package, error) {
	if len(names) == 0 {
		return []backend.Package{}, nil
	}
	args := append([]string{"-Si"}, names...)
	output, err := f.runner.Run(ctx, "pacman", args...)
	if err != nil && !onlyPackageNotFound(output) {
		return nil, commandError("pacman", args, output, err)
	}
	packages, parseErr := parsePackageInfo(output)
	if parseErr != nil {
		return nil, fmt.Errorf("parse available packages: %w", parseErr)
	}
	return packages, nil
}

func (f *PackageFacts) CompareVersions(a, b string) (int, error) {
	output, err := f.runner.Run(context.Background(), "vercmp", a, b)
	if err != nil {
		return 0, commandError("vercmp", []string{a, b}, output, err)
	}
	comparison, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil || comparison < -1 || comparison > 1 {
		return 0, fmt.Errorf("parse vercmp result %q", strings.TrimSpace(string(output)))
	}
	return comparison, nil
}

func (f *PackageFacts) nameSet(ctx context.Context, args ...string) (map[string]bool, error) {
	output, err := f.run(ctx, "pacman", args...)
	if err != nil {
		return nil, err
	}
	return parseNames(output), nil
}

func (f *PackageFacts) run(ctx context.Context, name string, args ...string) ([]byte, error) {
	output, err := f.runner.Run(ctx, name, args...)
	if err != nil {
		return nil, commandError(name, args, output, err)
	}
	return output, nil
}

func commandError(name string, args []string, output []byte, err error) error {
	detail := strings.TrimSpace(string(output))
	if detail == "" {
		return fmt.Errorf("run %s %s: %w", name, strings.Join(args, " "), err)
	}
	return fmt.Errorf("run %s %s: %w: %s", name, strings.Join(args, " "), err, detail)
}

func onlyPackageNotFound(output []byte) bool {
	found := false
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "error: package '") && strings.HasSuffix(line, "' was not found"):
			found = true
		case strings.HasPrefix(line, "error:") || strings.HasPrefix(line, "warning:"):
			return false
		}
	}
	return found
}

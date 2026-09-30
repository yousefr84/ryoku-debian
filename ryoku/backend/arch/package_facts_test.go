package arch

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	backend "ryoku-backend"
)

type commandCall struct {
	name   string
	args   []string
	output string
	err    error
}

type commandMock struct {
	t     *testing.T
	calls []commandCall
	next  int
}

func (m *commandMock) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	m.t.Helper()
	if m.next >= len(m.calls) {
		m.t.Fatalf("unexpected command: %s %v", name, args)
	}
	want := m.calls[m.next]
	m.next++
	if name != want.name || !reflect.DeepEqual(args, want.args) {
		m.t.Fatalf("command = %s %v, want %s %v", name, args, want.name, want.args)
	}
	return []byte(want.output), want.err
}

func (m *commandMock) done() {
	m.t.Helper()
	if m.next != len(m.calls) {
		m.t.Fatalf("ran %d commands, want %d", m.next, len(m.calls))
	}
}

func TestInventory(t *testing.T) {
	runner := &commandMock{t: t, calls: []commandCall{
		{
			name: "pacman",
			args: []string{"-Qi"},
			output: packageInfo("core", "bash", "5.3.3-1", "x86_64", "1.50 MiB") + "\n" +
				packageInfo("None", "aur-tool", "2.0-1", "any", "4.00 KiB") + "\n" +
				packageInfo("ryoku", "ryoku-shell", "1.4-2", "x86_64", "8 B"),
		},
		{name: "pacman", args: []string{"-Qqe"}, output: "bash\nryoku-shell\n"},
	}}

	got, err := NewPackageFacts(runner).Inventory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	runner.done()
	want := []backend.PackageState{
		{
			Package: backend.Package{
				Name: "bash", Version: "5.3.3-1", Architecture: "x86_64",
				Source: backend.PackageSourceDistribution, InstalledSize: 1572864,
			},
			Installed: true, ManuallyInstalled: true,
		},
		{
			Package: backend.Package{
				Name: "aur-tool", Version: "2.0-1", Architecture: "any",
				Source: backend.PackageSourceSupplemental, InstalledSize: 4096,
			},
			Installed: true,
		},
		{
			Package: backend.Package{
				Name: "ryoku-shell", Version: "1.4-2", Architecture: "x86_64",
				Source: backend.PackageSourceRyoku, InstalledSize: 8,
			},
			Installed: true, ManuallyInstalled: true,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Inventory() = %#v, want %#v", got, want)
	}
}

func TestQueryReportsInstalledAvailableAndMissingPackages(t *testing.T) {
	notFound := errors.New("exit status 1")
	runner := &commandMock{t: t, calls: []commandCall{
		{
			name: "pacman", args: []string{"-Qi", "alpha", "local-tool", "missing"},
			output: packageInfo("core", "alpha", "1.0-1", "x86_64", "2 KiB") + "\n" +
				packageInfo("None", "local-tool", "3.0-1", "any", "1 MiB") +
				"\nerror: package 'missing' was not found\n",
			err: notFound,
		},
		{name: "pacman", args: []string{"-Qqe"}, output: "alpha\n"},
		{
			name: "pacman", args: []string{"-Si", "alpha", "local-tool", "missing"},
			output: repositoryInfo("core", "alpha", "1.1-1", "x86_64", "2.25 KiB") +
				"\nerror: package 'local-tool' was not found\nerror: package 'missing' was not found\n",
			err: notFound,
		},
	}}

	got, err := NewPackageFacts(runner).Query(
		context.Background(), []string{"alpha", "local-tool", "missing"},
	)
	if err != nil {
		t.Fatal(err)
	}
	runner.done()

	if len(got) != 3 {
		t.Fatalf("Query() returned %d states, want 3", len(got))
	}
	if got[0].Package.Name != "alpha" || !got[0].Installed || !got[0].ManuallyInstalled ||
		got[0].AvailableVersion != "1.1-1" || got[0].Package.Source != backend.PackageSourceDistribution {
		t.Fatalf("installed repository package = %#v", got[0])
	}
	if got[1].Package.Name != "local-tool" || !got[1].Installed || got[1].ManuallyInstalled ||
		got[1].AvailableVersion != "" || got[1].Package.Source != backend.PackageSourceSupplemental {
		t.Fatalf("installed supplemental package = %#v", got[1])
	}
	if got[2].Package.Name != "missing" || got[2].Installed || got[2].AvailableVersion != "" {
		t.Fatalf("missing package = %#v", got[2])
	}
}

func TestAvailable(t *testing.T) {
	runner := &commandMock{t: t, calls: []commandCall{
		{
			name: "pacman", args: []string{"-Si", "linux", "ryoku-shell"},
			output: repositoryInfo("core", "linux", "6.18-1", "x86_64", "125 MiB") + "\n" +
				repositoryInfo("ryoku", "ryoku-shell", "1.4-2", "x86_64", "8 MiB"),
		},
	}}

	got, err := NewPackageFacts(runner).Available(context.Background(), []string{"linux", "ryoku-shell"})
	if err != nil {
		t.Fatal(err)
	}
	runner.done()
	if len(got) != 2 || got[0].Source != backend.PackageSourceDistribution ||
		got[1].Source != backend.PackageSourceRyoku || got[1].InstalledSize != 8*1024*1024 {
		t.Fatalf("Available() = %#v", got)
	}
}

func TestCompareVersions(t *testing.T) {
	runner := &commandMock{t: t, calls: []commandCall{
		{name: "vercmp", args: []string{"1.0-1", "2.0-1"}, output: "-1\n"},
		{name: "vercmp", args: []string{"2.0-1", "2.0-1"}, output: "0\n"},
		{name: "vercmp", args: []string{"3.0-1", "2.0-1"}, output: "1\n"},
	}}
	facts := NewPackageFacts(runner)
	for _, test := range []struct {
		a, b string
		want int
	}{
		{a: "1.0-1", b: "2.0-1", want: -1},
		{a: "2.0-1", b: "2.0-1", want: 0},
		{a: "3.0-1", b: "2.0-1", want: 1},
	} {
		got, err := facts.CompareVersions(test.a, test.b)
		if err != nil {
			t.Fatal(err)
		}
		if got != test.want {
			t.Fatalf("CompareVersions(%q, %q) = %d, want %d", test.a, test.b, got, test.want)
		}
	}
	runner.done()
}

func TestCommandFailureIncludesNativeDiagnostic(t *testing.T) {
	runner := &commandMock{t: t, calls: []commandCall{
		{name: "pacman", args: []string{"-Si", "alpha"}, output: "error: database is invalid\n", err: errors.New("exit status 1")},
	}}

	_, err := NewPackageFacts(runner).Available(context.Background(), []string{"alpha"})
	if err == nil || !strings.Contains(err.Error(), "database is invalid") {
		t.Fatalf("Available() error = %v", err)
	}
	runner.done()
}

func TestCompareVersionsRejectsInvalidOutput(t *testing.T) {
	runner := &commandMock{t: t, calls: []commandCall{
		{name: "vercmp", args: []string{"one", "two"}, output: "newer\n"},
	}}

	_, err := NewPackageFacts(runner).CompareVersions("one", "two")
	if err == nil || !strings.Contains(err.Error(), "parse vercmp result") {
		t.Fatalf("CompareVersions() error = %v", err)
	}
	runner.done()
}

func packageInfo(repository, name, version, architecture, size string) string {
	return "Installed From  : " + repository + "\n" +
		"Name            : " + name + "\n" +
		"Version         : " + version + "\n" +
		"Architecture    : " + architecture + "\n" +
		"Installed Size  : " + size + "\n"
}

func repositoryInfo(repository, name, version, architecture, size string) string {
	return "Repository      : " + repository + "\n" +
		"Name            : " + name + "\n" +
		"Version         : " + version + "\n" +
		"Architecture    : " + architecture + "\n" +
		"Installed Size  : " + size + "\n"
}

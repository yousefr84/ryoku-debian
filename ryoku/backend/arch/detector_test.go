package arch

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	backend "ryoku-backend"
)

type fileRead struct {
	path     string
	contents string
	err      error
}

type fileReaderMock struct {
	t     *testing.T
	reads []fileRead
}

func (m *fileReaderMock) ReadFile(path string) ([]byte, error) {
	m.t.Helper()
	if len(m.reads) == 0 {
		m.t.Fatalf("unexpected read of %q", path)
	}
	read := m.reads[0]
	m.reads = m.reads[1:]
	if path != read.path {
		m.t.Fatalf("read path = %q, want %q", path, read.path)
	}
	return []byte(read.contents), read.err
}

func (m *fileReaderMock) done() {
	m.t.Helper()
	if len(m.reads) != 0 {
		m.t.Fatalf("%d unread fixture(s)", len(m.reads))
	}
}

func TestPlatformDetectorDetectsArch(t *testing.T) {
	reader := &fileReaderMock{t: t, reads: []fileRead{{
		path: etcOSRelease,
		contents: `NAME="Arch Linux"
ID=arch
VERSION_ID='rolling'
ID_LIKE=linux
`,
	}}}

	got, err := NewPlatformDetector(reader).Detect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	reader.done()
	want := backend.PlatformIdentity{
		BackendID:           backend.BackendArch,
		DistributionID:      "arch",
		DistributionVersion: "rolling",
	}
	if got != want {
		t.Fatalf("identity = %#v, want %#v", got, want)
	}
}

func TestPlatformDetectorUsesVendorOSReleaseFallback(t *testing.T) {
	reader := &fileReaderMock{t: t, reads: []fileRead{
		{path: etcOSRelease, err: os.ErrNotExist},
		{path: usrOSRelease, contents: "ID=arch\n"},
	}}

	got, err := NewPlatformDetector(reader).Detect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	reader.done()
	if got.BackendID != backend.BackendArch || got.DistributionID != "arch" {
		t.Fatalf("identity = %#v, want Arch", got)
	}
}

func TestPlatformDetectorRejectsNonArchDistribution(t *testing.T) {
	reader := &fileReaderMock{t: t, reads: []fileRead{{
		path:     etcOSRelease,
		contents: "ID=debian\nID_LIKE=arch\nVERSION_ID=13\n",
	}}}

	got, err := NewPlatformDetector(reader).Detect(context.Background())
	reader.done()
	if err == nil || !strings.Contains(err.Error(), `unsupported distribution "debian"`) {
		t.Fatalf("error = %v, want unsupported distribution", err)
	}
	if got != (backend.PlatformIdentity{}) {
		t.Fatalf("identity = %#v, want zero value", got)
	}
}

func TestPlatformDetectorReportsMetadataErrors(t *testing.T) {
	readErr := errors.New("permission denied")
	reader := &fileReaderMock{t: t, reads: []fileRead{{
		path: etcOSRelease,
		err:  readErr,
	}}}

	_, err := NewPlatformDetector(reader).Detect(context.Background())
	reader.done()
	if !errors.Is(err, readErr) {
		t.Fatalf("error = %v, want wrapped read error", err)
	}
}

func TestPlatformDetectorHonorsCanceledContext(t *testing.T) {
	reader := &fileReaderMock{t: t}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := NewPlatformDetector(reader).Detect(ctx)
	reader.done()
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", err)
	}
}

func TestParseOSReleaseRequiresDistributionID(t *testing.T) {
	_, err := parseOSRelease([]byte("NAME=Arch Linux\n"))
	if err == nil || !strings.Contains(err.Error(), "ID is missing") {
		t.Fatalf("error = %v, want missing ID", err)
	}
}

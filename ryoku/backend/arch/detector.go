package arch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	backend "ryoku-backend"
)

const (
	etcOSRelease = "/etc/os-release"
	usrOSRelease = "/usr/lib/os-release"
)

// FileReader is the filesystem boundary used for platform detection.
type FileReader interface {
	ReadFile(string) ([]byte, error)
}

// PlatformDetector detects platforms supported by the Arch backend.
type PlatformDetector struct {
	reader FileReader
}

var _ backend.PlatformDetector = (*PlatformDetector)(nil)

// NewPlatformDetector builds an Arch platform detector. A nil reader uses the
// host filesystem.
func NewPlatformDetector(reader FileReader) *PlatformDetector {
	if reader == nil {
		reader = osFileReader{}
	}
	return &PlatformDetector{reader: reader}
}

// Detect reads Linux distribution metadata without constructing a backend.
func (d *PlatformDetector) Detect(ctx context.Context) (backend.PlatformIdentity, error) {
	if err := ctx.Err(); err != nil {
		return backend.PlatformIdentity{}, err
	}

	contents, path, err := d.readOSRelease()
	if err != nil {
		return backend.PlatformIdentity{}, err
	}
	release, err := parseOSRelease(contents)
	if err != nil {
		return backend.PlatformIdentity{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if release.id != "arch" {
		return backend.PlatformIdentity{}, fmt.Errorf("%s identifies unsupported distribution %q", path, release.id)
	}

	return backend.PlatformIdentity{
		BackendID:           backend.BackendArch,
		DistributionID:      release.id,
		DistributionVersion: release.versionID,
	}, nil
}

func (d *PlatformDetector) readOSRelease() ([]byte, string, error) {
	contents, err := d.reader.ReadFile(etcOSRelease)
	if err == nil {
		return contents, etcOSRelease, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, "", fmt.Errorf("read %s: %w", etcOSRelease, err)
	}

	contents, err = d.reader.ReadFile(usrOSRelease)
	if err != nil {
		return nil, "", fmt.Errorf("read %s: %w", usrOSRelease, err)
	}
	return contents, usrOSRelease, nil
}

type osRelease struct {
	id        string
	versionID string
}

func parseOSRelease(contents []byte) (osRelease, error) {
	var release osRelease
	for lineNumber, line := range strings.Split(string(contents), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, rawValue, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if key != "ID" && key != "VERSION_ID" {
			continue
		}
		value, err := parseOSReleaseValue(rawValue)
		if err != nil {
			return osRelease{}, fmt.Errorf("line %d: %w", lineNumber+1, err)
		}
		switch key {
		case "ID":
			release.id = value
		case "VERSION_ID":
			release.versionID = value
		}
	}
	if release.id == "" {
		return osRelease{}, errors.New("ID is missing")
	}
	return release, nil
}

func parseOSReleaseValue(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if raw[0] != '\'' && raw[0] != '"' {
		return raw, nil
	}
	if len(raw) < 2 || raw[len(raw)-1] != raw[0] {
		return "", fmt.Errorf("unterminated quoted value %q", raw)
	}
	return raw[1 : len(raw)-1], nil
}

type osFileReader struct{}

func (osFileReader) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

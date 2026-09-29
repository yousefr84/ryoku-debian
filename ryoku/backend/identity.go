package backend

// BackendID identifies a system-lifecycle implementation.
type BackendID string

const (
	BackendArch   BackendID = "arch"
	BackendDebian BackendID = "debian"
)

// InstallationMode describes how Ryoku is delivered on the installed system.
type InstallationMode string

const (
	InstallationModePackaged InstallationMode = "packaged"
	InstallationModeSource   InstallationMode = "source"
)

// PlatformIdentity records the selected backend and independently detected
// distribution facts. MarkerMatches is false when the installed marker and
// detected platform disagree.
type PlatformIdentity struct {
	BackendID              BackendID
	DistributionID         string
	DistributionVersion    string
	NativeArchitecture     string
	RepositoryArchitecture string
	InstalledBackendID     BackendID
	MarkerMatches          bool
	ReleaseChannel         string
	InstallationMode       InstallationMode
}

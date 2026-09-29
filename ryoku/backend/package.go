package backend

import "context"

// PackageSource identifies the class of source that provides a package.
type PackageSource string

const (
	PackageSourceUnknown      PackageSource = "unknown"
	PackageSourceDistribution PackageSource = "distribution"
	PackageSourceRyoku        PackageSource = "ryoku"
	PackageSourceSupplemental PackageSource = "supplemental"
	PackageSourceLocal        PackageSource = "local"
)

// Package is distribution-neutral native package metadata.
type Package struct {
	Name          string
	Version       string
	Architecture  string
	Source        PackageSource
	InstalledSize int64
}

// PackageState records the installed and available state of one package.
type PackageState struct {
	Package           Package
	Installed         bool
	ManuallyInstalled bool
	AvailableVersion  string
}

// InstallPlan is the exact package transaction reviewed before installation.
// StateFingerprint binds the plan to the package database state used to create
// it.
type InstallPlan struct {
	BackendID          BackendID
	Requested          []string
	Install            []Package
	Upgrade            []Package
	Downgrade          []Package
	Remove             []Package
	DownloadBytes      int64
	InstalledSizeDelta int64
	StateFingerprint   string
	Warnings           []string
}

// RemovePlan is the exact package transaction reviewed before removal.
// StateFingerprint binds the plan to the package database state used to create
// it.
type RemovePlan struct {
	BackendID        BackendID
	Requested        []string
	Remove           []Package
	FreedBytes       int64
	StateFingerprint string
	Warnings         []string
}

// PackageManager owns native package inventory, resolution, and transactions.
// ApplyInstall and ApplyRemove must reject plans whose state fingerprint no
// longer matches the native package database.
type PackageManager interface {
	Inventory(ctx context.Context) ([]PackageState, error)
	Query(ctx context.Context, names []string) ([]PackageState, error)
	OwnerOf(ctx context.Context, path string) (Package, bool, error)
	Available(ctx context.Context, names []string) ([]Package, error)
	CompareVersions(a, b string) (int, error)
	PlanInstall(ctx context.Context, names []string) (InstallPlan, error)
	ApplyInstall(ctx context.Context, plan InstallPlan) error
	PlanRemove(ctx context.Context, names []string) (RemovePlan, error)
	ApplyRemove(ctx context.Context, plan RemovePlan) error
}

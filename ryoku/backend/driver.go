package backend

import "context"

// DriverInfo describes a distribution-neutral hardware driver choice and the
// native packages that fulfill it.
type DriverInfo struct {
	ID             string
	Name           string
	Vendor         string
	Version        string
	Packages       []Package
	Installed      bool
	Active         bool
	Recommended    bool
	RequiresReboot bool
}

// DriverManager owns distribution-specific driver discovery and fulfillment.
type DriverManager interface {
	Drivers(ctx context.Context) ([]DriverInfo, error)
	PlanInstall(ctx context.Context, drivers []DriverInfo) (InstallPlan, error)
	Apply(ctx context.Context, plan InstallPlan) error
	Verify(ctx context.Context, drivers []DriverInfo) ([]DriverInfo, error)
}

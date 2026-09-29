package backend

import (
	"context"
	"testing"
)

type packageManagerStub struct{}

func (packageManagerStub) Inventory(context.Context) ([]PackageState, error) {
	return nil, nil
}

func (packageManagerStub) Query(context.Context, []string) ([]PackageState, error) {
	return nil, nil
}

func (packageManagerStub) OwnerOf(context.Context, string) (Package, bool, error) {
	return Package{}, false, nil
}

func (packageManagerStub) Available(context.Context, []string) ([]Package, error) {
	return nil, nil
}

func (packageManagerStub) CompareVersions(string, string) (int, error) {
	return 0, nil
}

func (packageManagerStub) PlanInstall(context.Context, []string) (InstallPlan, error) {
	return InstallPlan{}, nil
}

func (packageManagerStub) ApplyInstall(context.Context, InstallPlan) error {
	return nil
}

func (packageManagerStub) PlanRemove(context.Context, []string) (RemovePlan, error) {
	return RemovePlan{}, nil
}

func (packageManagerStub) ApplyRemove(context.Context, RemovePlan) error {
	return nil
}

type repositoryManagerStub struct{}

func (repositoryManagerStub) Inspect(context.Context) (RepositoryState, error) {
	return RepositoryState{}, nil
}

func (repositoryManagerStub) Ensure(context.Context, RepositoryState) error {
	return nil
}

func (repositoryManagerStub) SetChannel(context.Context, string) error {
	return nil
}

func (repositoryManagerStub) Refresh(context.Context) error {
	return nil
}

type bootManagerStub struct{}

func (bootManagerStub) Kernels(context.Context) ([]KernelInfo, error) {
	return nil, nil
}

func (bootManagerStub) DefaultKernel(context.Context) (KernelInfo, error) {
	return KernelInfo{}, nil
}

func (bootManagerStub) RebuildInitramfs(context.Context, []KernelInfo) error {
	return nil
}

func (bootManagerStub) RefreshBootEntries(context.Context) error {
	return nil
}

type driverManagerStub struct{}

func (driverManagerStub) Drivers(context.Context) ([]DriverInfo, error) {
	return nil, nil
}

func (driverManagerStub) PlanInstall(context.Context, []DriverInfo) (InstallPlan, error) {
	return InstallPlan{}, nil
}

func (driverManagerStub) Apply(context.Context, InstallPlan) error {
	return nil
}

func (driverManagerStub) Verify(context.Context, []DriverInfo) ([]DriverInfo, error) {
	return nil, nil
}

var (
	_ PackageManager    = packageManagerStub{}
	_ RepositoryManager = repositoryManagerStub{}
	_ BootManager       = bootManagerStub{}
	_ DriverManager     = driverManagerStub{}
)

func TestSystemBackendComposesContracts(t *testing.T) {
	packages := packageManagerStub{}
	repositories := repositoryManagerStub{}
	boot := bootManagerStub{}
	drivers := driverManagerStub{}
	wantIdentity := PlatformIdentity{
		BackendID:              BackendArch,
		DistributionID:         "arch",
		NativeArchitecture:     "x86_64",
		RepositoryArchitecture: "x86_64",
		InstalledBackendID:     BackendArch,
		MarkerMatches:          true,
		ReleaseChannel:         "stable",
		InstallationMode:       InstallationModePackaged,
	}

	got := SystemBackend{
		Identity:     wantIdentity,
		Packages:     packages,
		Repositories: repositories,
		Boot:         boot,
		Drivers:      drivers,
	}

	if got.Identity != wantIdentity {
		t.Fatalf("identity = %#v, want %#v", got.Identity, wantIdentity)
	}
	if got.Packages == nil || got.Repositories == nil || got.Boot == nil || got.Drivers == nil {
		t.Fatal("SystemBackend did not retain every composed service")
	}
}

func TestPlansRetainBackendAndFingerprint(t *testing.T) {
	install := InstallPlan{BackendID: BackendArch, StateFingerprint: "install-state"}
	remove := RemovePlan{BackendID: BackendArch, StateFingerprint: "remove-state"}

	if install.BackendID != BackendArch || install.StateFingerprint != "install-state" {
		t.Fatalf("install plan lost identity or fingerprint: %#v", install)
	}
	if remove.BackendID != BackendArch || remove.StateFingerprint != "remove-state" {
		t.Fatalf("remove plan lost identity or fingerprint: %#v", remove)
	}
}

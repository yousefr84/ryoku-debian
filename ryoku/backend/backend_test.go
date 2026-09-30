package backend

import (
	"context"
	"testing"
)

type packageFactsStub struct{}

func (packageFactsStub) Inventory(context.Context) ([]PackageState, error) {
	return nil, nil
}

func (packageFactsStub) Query(context.Context, []string) ([]PackageState, error) {
	return nil, nil
}

func (packageFactsStub) OwnerOf(context.Context, string) (Package, bool, error) {
	return Package{}, false, nil
}

func (packageFactsStub) Available(context.Context, []string) ([]Package, error) {
	return nil, nil
}

func (packageFactsStub) CompareVersions(string, string) (int, error) {
	return 0, nil
}

type packageTransactionsStub struct{}

func (packageTransactionsStub) PlanInstall(context.Context, []string) (InstallPlan, error) {
	return InstallPlan{}, nil
}

func (packageTransactionsStub) ApplyInstall(context.Context, InstallPlan) error {
	return nil
}

func (packageTransactionsStub) PlanRemove(context.Context, []string) (RemovePlan, error) {
	return RemovePlan{}, nil
}

func (packageTransactionsStub) ApplyRemove(context.Context, RemovePlan) error {
	return nil
}

type packageManagerStub struct {
	packageFactsStub
	packageTransactionsStub
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
	_ PackageFacts        = packageFactsStub{}
	_ PackageTransactions = packageTransactionsStub{}
	_ PackageManager      = packageManagerStub{}
	_ RepositoryManager   = repositoryManagerStub{}
	_ BootManager         = bootManagerStub{}
	_ DriverManager       = driverManagerStub{}
)

func TestSystemBackendComposesContracts(t *testing.T) {
	packageFacts := packageFactsStub{}
	packageTransactions := packageTransactionsStub{}
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
		Identity:            wantIdentity,
		PackageFacts:        packageFacts,
		PackageTransactions: packageTransactions,
		Repositories:        repositories,
		Boot:                boot,
		Drivers:             drivers,
	}

	if got.Identity != wantIdentity {
		t.Fatalf("identity = %#v, want %#v", got.Identity, wantIdentity)
	}
	if got.PackageFacts == nil || got.PackageTransactions == nil || got.Repositories == nil || got.Boot == nil || got.Drivers == nil {
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

package arch

import (
	"context"
	"testing"

	backend "ryoku-backend"
)

type fakePackageFacts struct{}

func (*fakePackageFacts) Inventory(context.Context) ([]backend.PackageState, error) {
	return nil, nil
}

func (*fakePackageFacts) Query(context.Context, []string) ([]backend.PackageState, error) {
	return nil, nil
}

func (*fakePackageFacts) OwnerOf(context.Context, string) (backend.Package, bool, error) {
	return backend.Package{}, false, nil
}

func (*fakePackageFacts) Available(context.Context, []string) ([]backend.Package, error) {
	return nil, nil
}

func (*fakePackageFacts) CompareVersions(string, string) (int, error) {
	return 0, nil
}

func TestNewComposesAvailableCapabilities(t *testing.T) {
	packageFacts := &fakePackageFacts{}
	got := New(packageFacts)

	wantIdentity := backend.PlatformIdentity{
		BackendID:      backend.BackendArch,
		DistributionID: "arch",
	}
	if got.Identity != wantIdentity {
		t.Fatalf("identity = %#v, want %#v", got.Identity, wantIdentity)
	}
	if got.PackageFacts != packageFacts {
		t.Fatalf("package facts = %T, want injected dependency", got.PackageFacts)
	}
	if got.PackageTransactions != nil || got.Repositories != nil || got.Boot != nil || got.Drivers != nil {
		t.Fatal("provider exposed an unsupported capability")
	}
}

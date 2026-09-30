package arch

import (
	"testing"

	backend "ryoku-backend"
)

func TestNewComposesAvailableCapabilities(t *testing.T) {
	got := New()

	wantIdentity := backend.PlatformIdentity{
		BackendID:      backend.BackendArch,
		DistributionID: "arch",
	}
	if got.Identity != wantIdentity {
		t.Fatalf("identity = %#v, want %#v", got.Identity, wantIdentity)
	}
	_, ok := got.PackageFacts.(*PackageFacts)
	if !ok {
		t.Fatalf("package facts = %T, want *arch.PackageFacts", got.PackageFacts)
	}
	if got.PackageTransactions != nil || got.Repositories != nil || got.Boot != nil || got.Drivers != nil {
		t.Fatal("provider exposed an unsupported capability")
	}
}

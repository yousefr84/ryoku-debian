package arch

import (
	"testing"

	backend "ryoku-backend"
)

func TestRegisterAddsArchFactory(t *testing.T) {
	var registry backend.Registry
	packageFacts := &fakePackageFacts{}

	if err := Register(&registry); err != nil {
		t.Fatalf("Register returned an error: %v", err)
	}
	factory, err := registry.Lookup(backend.BackendArch)
	if err != nil {
		t.Fatalf("Lookup returned an error: %v", err)
	}

	got := factory(backend.BackendDependencies{PackageFacts: packageFacts})
	if got.PackageFacts != packageFacts {
		t.Fatalf("factory package facts = %T, want injected dependency", got.PackageFacts)
	}
	if got.Identity.BackendID != backend.BackendArch {
		t.Fatalf("factory backend ID = %q, want %q", got.Identity.BackendID, backend.BackendArch)
	}
}

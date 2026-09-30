package backend

import (
	"errors"
	"strings"
	"testing"
)

func TestRegistryRegisterAndLookupFactory(t *testing.T) {
	var registry Registry
	want := BackendID("fixture")
	factory := func(BackendDependencies) SystemBackend {
		return SystemBackend{Identity: PlatformIdentity{BackendID: want}}
	}

	if err := registry.Register(want, factory); err != nil {
		t.Fatalf("Register returned an error: %v", err)
	}
	got, err := registry.Lookup(want)
	if err != nil {
		t.Fatalf("Lookup returned an error: %v", err)
	}
	if backend := got(BackendDependencies{}); backend.Identity.BackendID != want {
		t.Fatalf("factory backend ID = %q, want %q", backend.Identity.BackendID, want)
	}
}

func TestRegistryLookupUnknownBackend(t *testing.T) {
	var registry Registry
	id := BackendID("unknown")

	_, err := registry.Lookup(id)
	if !errors.Is(err, ErrBackendNotRegistered) {
		t.Fatalf("Lookup error = %v, want ErrBackendNotRegistered", err)
	}
	if !strings.Contains(err.Error(), string(id)) {
		t.Fatalf("Lookup error %q does not identify backend %q", err, id)
	}
}

func TestRegistryRejectsDuplicateRegistration(t *testing.T) {
	var registry Registry
	id := BackendID("fixture")
	first := func(BackendDependencies) SystemBackend {
		return SystemBackend{Identity: PlatformIdentity{DistributionID: "first"}}
	}
	second := func(BackendDependencies) SystemBackend {
		return SystemBackend{Identity: PlatformIdentity{DistributionID: "second"}}
	}

	if err := registry.Register(id, first); err != nil {
		t.Fatalf("first Register returned an error: %v", err)
	}
	if err := registry.Register(id, second); !errors.Is(err, ErrBackendAlreadyRegistered) {
		t.Fatalf("duplicate Register error = %v, want ErrBackendAlreadyRegistered", err)
	}

	got, err := registry.Lookup(id)
	if err != nil {
		t.Fatalf("Lookup returned an error: %v", err)
	}
	if backend := got(BackendDependencies{}); backend.Identity.DistributionID != "first" {
		t.Fatalf("duplicate registration replaced original factory: %#v", backend.Identity)
	}
}

func TestBackendFactoryReceivesDependencies(t *testing.T) {
	var registry Registry
	id := BackendID("fixture")
	want := packageFactsStub{}

	if err := registry.Register(id, func(deps BackendDependencies) SystemBackend {
		return SystemBackend{PackageFacts: deps.PackageFacts}
	}); err != nil {
		t.Fatalf("Register returned an error: %v", err)
	}
	factory, err := registry.Lookup(id)
	if err != nil {
		t.Fatalf("Lookup returned an error: %v", err)
	}

	got := factory(BackendDependencies{PackageFacts: want})
	if got.PackageFacts != want {
		t.Fatalf("factory package facts = %T, want injected dependency", got.PackageFacts)
	}
}

package arch

import backend "ryoku-backend"

// New composes the capabilities currently implemented by the Arch backend.
func New(packageFacts backend.PackageFacts) backend.SystemBackend {
	return backend.SystemBackend{
		Identity: backend.PlatformIdentity{
			BackendID:      backend.BackendArch,
			DistributionID: "arch",
		},
		PackageFacts: packageFacts,
	}
}

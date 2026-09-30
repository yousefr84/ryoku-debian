package arch

import backend "ryoku-backend"

// New composes the capabilities currently implemented by the Arch backend.
func New() backend.SystemBackend {
	return backend.SystemBackend{
		Identity: backend.PlatformIdentity{
			BackendID:      backend.BackendArch,
			DistributionID: "arch",
		},
		PackageFacts: NewPackageFacts(nil),
	}
}

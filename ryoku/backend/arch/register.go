package arch

import backend "ryoku-backend"

// Register adds the Arch backend factory to registry.
func Register(registry *backend.Registry) error {
	return registry.Register(backend.BackendArch, func(deps backend.BackendDependencies) backend.SystemBackend {
		return New(deps.PackageFacts)
	})
}

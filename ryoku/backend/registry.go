package backend

import (
	"errors"
	"fmt"
)

var (
	ErrBackendAlreadyRegistered = errors.New("backend factory already registered")
	ErrBackendNotRegistered     = errors.New("backend factory not registered")
)

// BackendDependencies contains the shared services available to backend
// factories.
type BackendDependencies struct {
	PackageFacts PackageFacts
}

// BackendFactory constructs one system backend from injected dependencies.
type BackendFactory func(BackendDependencies) SystemBackend

// Registry stores backend factories by identity.
type Registry struct {
	factories map[BackendID]BackendFactory
}

// Register adds a factory without replacing an existing registration.
func (r *Registry) Register(id BackendID, factory BackendFactory) error {
	if r.factories == nil {
		r.factories = make(map[BackendID]BackendFactory)
	}
	if _, exists := r.factories[id]; exists {
		return fmt.Errorf("%w: %q", ErrBackendAlreadyRegistered, id)
	}
	r.factories[id] = factory
	return nil
}

// Lookup returns the factory registered for id.
func (r *Registry) Lookup(id BackendID) (BackendFactory, error) {
	factory, exists := r.factories[id]
	if !exists {
		return nil, fmt.Errorf("%w: %q", ErrBackendNotRegistered, id)
	}
	return factory, nil
}

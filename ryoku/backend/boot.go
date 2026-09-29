package backend

import "context"

// KernelInfo describes one installed kernel and its boot artifacts.
type KernelInfo struct {
	ID               string
	Version          string
	Package          Package
	KernelImage      string
	InitramfsImages  []string
	ModulesDirectory string
	Running          bool
	Default          bool
}

// BootManager owns distribution-specific kernel and boot-artifact mechanics.
type BootManager interface {
	Kernels(ctx context.Context) ([]KernelInfo, error)
	DefaultKernel(ctx context.Context) (KernelInfo, error)
	RebuildInitramfs(ctx context.Context, kernels []KernelInfo) error
	RefreshBootEntries(ctx context.Context) error
}

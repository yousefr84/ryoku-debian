package backend

import "context"

// RepositoryState is the normalized state of the configured Ryoku package
// source and the release it currently serves.
type RepositoryState struct {
	Configured   bool
	Trusted      bool
	Channel      string
	BaseURL      string
	Architecture string
	Release      string
	Version      string
	Codename     string
	Commit       string
	ManifestURL  string
	Problems     []string
}

// RepositoryManager owns Ryoku repository configuration, trust, and metadata.
type RepositoryManager interface {
	Inspect(ctx context.Context) (RepositoryState, error)
	Ensure(ctx context.Context, desired RepositoryState) error
	SetChannel(ctx context.Context, channel string) error
	Refresh(ctx context.Context) error
}

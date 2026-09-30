package backend

// SystemBackend composes the distribution-specific lifecycle services selected
// for one installed system. Selection and implementations live outside this
// contract package.
type SystemBackend struct {
	Identity            PlatformIdentity
	PackageFacts        PackageFacts
	PackageTransactions PackageTransactions
	Repositories        RepositoryManager
	Boot                BootManager
	Drivers             DriverManager
}

# System backend architecture

## Status and intent

This document defines the Phase 2 architecture for adding a Debian backend to
Ryoku. It follows the dependency audit in
`docs/migration/arch-dependency-map.md`. It defines boundaries, contracts,
responsibilities, sequencing, and risks only. It does not authorize or describe
an implementation patch.

The goal is to replace distribution lifecycle coupling while preserving the
Ryoku desktop architecture. The QML surfaces, Quickshell process model, shared
`Ryoku.Ui` module, shell IPC, compositor provider contract, configuration
overlay, materialization behavior, visual language, and user workflows remain
the product. Arch and Debian differ below those layers.

## Design principles

1. **Keep the desktop distribution-neutral.** QML and Quickshell must not learn
   apt, dpkg, pacman, AUR, repository file formats, kernel package names, or
   initramfs tools. They consume existing Ryoku commands and neutral data.
2. **Abstract semantics, not command lines.** The contract expresses operations
   such as “install these Ryoku components,” “plan safe removal,” and “rebuild
   boot artifacts.” It must not expose a generic `RunPackageManager(args)` API.
3. **Preserve native correctness.** Arch remains free to use pacman, libalpm
   hooks, makepkg, and mkinitcpio. Debian remains free to use apt, dpkg,
   maintainer scripts, kernel hooks, and its selected initramfs tooling.
4. **Keep Ryoku and distribution updates separate.** `ryoku update` moves only
   Ryoku-published components. A distribution upgrade remains a distinct,
   explicit operation, including when `ryoku update --system` sequences both.
5. **Plan before mutation.** Destructive or dependency-changing operations
   produce a reviewable plan. Application revalidates the plan against current
   package state and refuses unexpected expansion.
6. **Fail closed on an unknown platform.** No backend may silently fall back to
   Arch, Debian, source deployment, or raw shell commands.
7. **Declare unavailable features honestly.** A backend reports capabilities.
   It must not pretend an Arch-only feature exists on Debian or quietly skip a
   required package.
8. **Retain one-way delivery.** The repository remains the source of truth.
   Packages deliver system payloads; materialize delivers user config; doctor
   converges state and drift.
9. **Test each backend as a product.** The same behavioral contract applies to
   both, but native package and boot behavior is tested in native environments.
10. **Do not redesign UI.** Backend differences may change labels, commands,
    availability, and data. They do not create parallel Arch and Debian screens.

## Non-goals

- Replacing QML, Quickshell, Hyprland, niri, or the compositor seam.
- Creating a universal Linux package manager.
- Supporting every Debian derivative through `ID_LIKE=debian`.
- Making Debian imitate AUR, CachyOS, pacman hooks, or Arch package naming.
- Moving build toolchains onto installed systems.
- Combining Arch and Debian payloads in one native package artifact.
- Changing the user-edit overlay, materialization, shell IPC, or visual design.
- Selecting final Debian package versions or resolving every package gap in this
  phase.

## 1. System backend architecture

### 1.1 Layering

The backend is a shared system-lifecycle layer below existing application
services. UI processes continue to invoke `ryoku`, `ryoku-hub`, `ryoku-shell`,
and compositor providers through their current boundaries.

```text
QML / Quickshell / shell clients
                |
                v
Existing Ryoku application services
  updater | doctor | wm switch | Hub operations | installer orchestration
                |
                v
SystemBackend selected once for the installed system
  PackageManager | RepositoryManager | BootManager | DriverManager
                |
        +-------+-------+
        |               |
        v               v
   Arch backend     Debian backend
 pacman/alpm/...   apt/dpkg/...
```

The backend is not a daemon and does not create another user-facing protocol.
It is a shared library contract for Go consumers plus matching installer and
release contracts where Go cannot own the operation. Existing binaries remain
the authority for their current concerns.

### 1.2 Backend aggregate

The runtime aggregate has a small identity and four focused services:

```go
type SystemBackend interface {
    Identity() PlatformIdentity
    Capabilities() BackendCapabilities
    Packages() PackageManager
    Repositories() RepositoryManager
    Boot() BootManager
    Drivers() DriverManager
}
```

This is an architectural signature, not prescribed source code. The interfaces
may be split by consuming module to avoid oversized dependencies, but the
semantic boundary must remain the same.

`PlatformIdentity` contains:

- backend ID, initially `arch` or `debian`;
- distribution ID and version read from `/etc/os-release`;
- native architecture and Ryoku repository architecture;
- installed backend marker and whether it agrees with the detected system;
- release channel and packaged/source installation mode.

`BackendCapabilities` is typed and finite. Initial fields cover:

- packaged Ryoku updates;
- system updates;
- local native-package installation;
- supplemental package source;
- safe removal planning;
- offline target installation;
- supported bootloader and snapshot integration;
- proprietary NVIDIA installation;
- 32-bit graphics support;
- Secure Boot support level.

Capabilities describe promises the backend implements. They are not stringly
typed command names and do not contain UI copy.

### 1.3 Backend selection

The installed system records its backend in a root-owned Ryoku release marker.
The exact file format may extend `/etc/ryoku-release`, but there must be one
authoritative marker written by the installer or package. Runtime selection:

1. reads the installed marker;
2. reads `/etc/os-release` as an independent fact;
3. verifies that the combination is supported;
4. selects exactly one registered backend;
5. returns an actionable unsupported-platform error on mismatch or ambiguity.

`ID_LIKE` is a discovery hint, not proof that a derivative satisfies the Debian
contract. Supporting Ubuntu or another derivative requires an explicit support
declaration and tests.

Tests inject a backend. Production code must not mutate a global package-manager
function or search `PATH` to decide the distribution on every operation.

### 1.4 Shared application services

The backend does not absorb product policy. Existing services retain these
responsibilities:

- updater owns stage ordering, snapshots, run state, materialize, reload, and
  the distinction between Ryoku and system updates;
- doctor owns desired-state reconciliation and explanations;
- the WM seam owns compositor capabilities and config behavior;
- the Hub owns user interaction and presentation;
- the installer owns its stage machine and TUI contract;
- release tooling owns versions, codenames, promotion, and publication gates.

Those services ask the backend for native facts, plans, and mutations. They no
longer parse native package-manager output or native repository files.

### 1.5 Logical component catalog

Package names must stop acting as cross-distribution identity. The design uses a
logical component catalog with stable Ryoku IDs, for example:

```text
desktop.core
desktop.compositor.hyprland
desktop.compositor.niri
runtime.quickshell
hardware.gpu.amd.vulkan
hardware.cpu.amd.microcode
feature.dictation
```

Each backend resolves a component ID to a native delivery record:

```go
type ComponentBinding struct {
    ID          ComponentID
    NativeNames []string
    Source      PackageSource
    Required    bool
    Architectures []string
}
```

The catalog separates three facts currently mixed in `*.packages` files:

- product intent: which capability Ryoku requires;
- platform resolution: which native packages provide it;
- release state: which exact packages and versions were published.

Source manifests use component IDs. A release produces one resolved manifest
per backend and architecture. The resolved manifest records both logical IDs and
native package names so update, verify, rollback, and diagnostics remain
auditable.

Compositor providers declare logical component IDs instead of pacman package
names. The package backend resolves them before installation or removal
planning. This preserves the rule that only a compositor provider knows what it
is made of without making the provider Arch-specific.

### 1.6 Error, dry-run, and event contract

Every mutating backend operation supports:

- a dry-run or plan-only path with no filesystem or package-database mutation;
- structured progress events for the existing terminal and Hub renderers;
- a typed error class plus retained native diagnostic text;
- cancellation through context;
- an operation ID for update run-state and support reports.

Backends classify at least: unavailable package, trust failure, repository
failure, dependency conflict, insufficient disk space, lock held, permission
failure, unsupported operation, boot-artifact failure, and interrupted
transaction. Consumers act on the class and display sanitized native detail.
They do not parse English CLI output to decide control flow.

### 1.7 Privilege boundary

Read-only inventory and planning run unprivileged whenever the native tools
allow it. Mutations use one reviewed privilege runner shared by the runtime
backend. The runner accepts a fixed executable and argument vector from backend
code, never a shell fragment supplied by QML or manifest data.

The installer already runs with installation authority and uses a target runner.
Runtime elevation remains visible through the existing terminal or polkit flow.
No backend is permitted to make an interactive package call from a background
QML process with no usable prompt.

## 2. PackageManager interface

### 2.1 Responsibilities

`PackageManager` owns native package database semantics. It is the only runtime
layer allowed to know pacman, apt, dpkg, native package file formats, native
version ordering, transaction locks, or config-file upgrade artifacts.

It provides five groups of operations: inventory, resolution, planning,
execution, and artifact inspection.

### 2.2 Data model

```go
type PackageName string

type PackageRecord struct {
    Name          PackageName
    Version       string
    Architecture  string
    Source        PackageSource
    Installed     bool
    ManuallyInstalled bool
    InstalledSize int64
}

type PackageSource struct {
    Kind PackageSourceKind
    Name string
}

type UpdateCandidate struct {
    Package PackageRecord
    AvailableVersion string
    Security bool
}

type TransactionRequest struct {
    Action TransactionAction
    Scope  TransactionScope
    Components []ComponentID
    Packages []PackageName
    AllowDowngrade bool
    RemoveUnusedDependencies bool
}

type TransactionPlan struct {
    BackendID string
    Action TransactionAction
    Requested []PackageName
    Install []PackageChange
    Upgrade []PackageChange
    Downgrade []PackageChange
    Remove []PackageChange
    DownloadBytes int64
    InstalledDelta int64
    StateFingerprint string
    Warnings []PlanWarning
}
```

`PackageSourceKind` includes distribution, Ryoku, supplemental, local artifact,
and unknown. It does not include AUR as a universal concept. The Arch backend
may label its supplemental source `aur`; Debian may have no supplemental source
or may resolve the component from the Ryoku repository.

### 2.3 Contract

```go
type PackageManager interface {
    Inventory(ctx context.Context) (PackageInventory, error)
    Query(ctx context.Context, names []PackageName) ([]PackageRecord, error)
    OwnerOf(ctx context.Context, path string) (PackageRecord, bool, error)
    Available(ctx context.Context, names []PackageName) ([]PackageRecord, error)
    PendingUpdates(ctx context.Context, scope TransactionScope) ([]UpdateCandidate, error)
    CompareVersions(a, b string) (int, error)

    RefreshMetadata(ctx context.Context, sources []PackageSourceKind, events EventSink) error
    Plan(ctx context.Context, request TransactionRequest) (TransactionPlan, error)
    Revalidate(ctx context.Context, plan TransactionPlan) error
    Apply(ctx context.Context, plan TransactionPlan, events EventSink) (TransactionResult, error)

    InspectArtifact(ctx context.Context, path string) (NativeArtifact, error)
    PlanArtifactInstall(ctx context.Context, artifact NativeArtifact) (TransactionPlan, error)
}
```

The implementation may optimize inventory calls, but callers see a consistent
snapshot. `Inventory` must distinguish installed from manually installed
packages because manifest verification and user additions depend on it.

### 2.4 Transaction rules

- `TransactionScopeRyoku` may change only packages resolved from the configured
  Ryoku repository and explicitly listed external-release exceptions. It must
  not perform a distribution upgrade as a side effect.
- `TransactionScopeSystem` is the user's distribution upgrade and may move the
  kernel and base system according to native policy.
- `TransactionScopeSupplemental` is explicitly best-effort and never required
  for the core desktop unless Ryoku republishes that component in its own repo.
- A channel change or rollback sets `AllowDowngrade`; ordinary updates do not.
- `Apply` revalidates `StateFingerprint`. A removal plan that gains packages or
  otherwise changes after review is rejected and must be shown again.
- The backend returns structured package changes. Human-readable package-manager
  output is retained for logs, not used as the primary data model.
- A partial native transaction is reported honestly. The application service
  decides whether materialize, doctor, reload, or finalization may continue.

### 2.5 Safe removal

Compositor switching depends on exact removal planning. The interface must
support this without reproducing a dependency solver in Ryoku:

1. the WM provider supplies outgoing and incoming component IDs;
2. the catalog resolves native package names;
3. shared names and non-installed targets are removed from the request;
4. the native package manager plans the removal and unused dependency cascade;
5. the UI shows the exact plan and size;
6. immediately before execution, the backend revalidates the plan;
7. any expanded removal set aborts.

Arch may implement this with pacman's solver. Debian must use apt/dpkg planning
data and must not treat autoremove suggestions as approved removals unless they
appear in the reviewed plan.

### 2.6 Config-file and lock semantics

Native config upgrade behavior belongs inside the backend:

- Arch reports `.pacnew` and `.pacsave` state.
- Debian reports native conffile divergence, pending maintainer versions, or
  interrupted dpkg state through neutral findings.
- Lock detection reports owner/state when available and never deletes an active
  lock.
- Recovery is backend-specific and exposed as a typed, separately authorized
  repair. The common updater never deletes a hard-coded database lock.

### 2.7 Package clients

The following current clients must consume this interface rather than native
commands:

- updater and update status;
- manifest reconciliation and `ryoku verify`;
- package portions of doctor;
- compositor availability, switch, and reclaim;
- Hub GPU and optional-feature installation;
- Extras and Ryostore routing;
- system profile statistics and Rashin package facts;
- local native-package installation in the stash;
- recovery and developer track operations where packaged state matters.

QML receives already-neutral rows. It does not call a package manager or encode
manager filters.

## 3. Repository abstraction

### 3.1 Separation from package transactions

Repository configuration and package transactions are separate contracts.
`PackageManager` answers what the native solver can do. `RepositoryManager`
owns where Ryoku packages come from, how that source is trusted, and which
release/channel it represents.

### 3.2 Repository model

```go
type RepositoryState struct {
    Configured bool
    Trusted bool
    Channel string
    BaseURL string
    Architecture string
    Release ReleaseDescriptor
    Problems []RepositoryProblem
}

type ReleaseDescriptor struct {
    Release string
    Version string
    Codename string
    Commit string
    Channel string
    ManifestURL string
}

type RepositoryManager interface {
    Inspect(ctx context.Context) (RepositoryState, error)
    Ensure(ctx context.Context, desired RepositorySpec, events EventSink) error
    SetChannel(ctx context.Context, channel string, events EventSink) error
    Refresh(ctx context.Context, events EventSink) error
    CurrentRelease(ctx context.Context) (ReleaseDescriptor, error)
    PublishedComponents(ctx context.Context) ([]ResolvedComponent, error)
    Repair(ctx context.Context, problem RepositoryProblem, events EventSink) error
}
```

The repository manager owns native configuration format, key placement,
metadata cache invalidation, architecture substitution, and channel URL shape.
The updater never edits a repository file directly.

### 3.3 Channel contract

Stable, testing, and frozen release channels remain product concepts shared by
both backends. A channel maps to a backend-specific native repository endpoint
plus the same logical release descriptor.

Each backend publishes its own resolved manifest. A manifest includes:

- backend ID and native architecture;
- Ryoku release, version, codename, commit, and channel;
- logical component IDs;
- resolved native package names and expected versions;
- lane and opt-in status;
- payload/config delivery metadata needed by verification.

An Arch manifest must never be consumed on Debian or vice versa. The backend ID
is part of validation, not merely descriptive metadata.

### 3.4 Trust and key rotation

The release identity and signing policy are shared. Native trust installation is
not:

- Arch owns its pacman keyring package, required package/database signatures,
  and sync database recovery.
- Debian owns a dedicated archive keyring path, signed repository metadata, and
  source configuration that references that key explicitly.

No backend may lower global signature policy to make Ryoku install. Bootstrap
trust is delivered by the installer image or an independently verified keyring
package. Key rotation must be testable before the old key expires.

### 3.5 Build-time repository backend

Runtime `RepositoryManager` is paired with a release-side `RepositoryPublisher`
contract:

```go
type RepositoryPublisher interface {
    BuildPackages(release BuildRelease) (ArtifactSet, error)
    BuildIndex(artifacts ArtifactSet) (RepositoryArtifact, error)
    Sign(repository RepositoryArtifact) error
    Verify(repository RepositoryArtifact) error
    SmokeTest(repository RepositoryArtifact) error
    Publish(repository RepositoryArtifact, destination ChannelDestination) error
}
```

This is a CI contract, not runtime code. Arch continues to produce signed pacman
artifacts. Debian produces signed Debian repository metadata and `.deb`
artifacts. Both publish `release.json` and the backend-specific resolved
manifest, and both follow the existing rule that the tested artifact is the
artifact uploaded.

Package build recipes remain native and separate. Shared build helpers may
compile Go binaries, QML plugins, and other upstream payloads once per target
environment, but a PKGBUILD is never treated as a Debian recipe and a Debian
recipe is never treated as an Arch package source.

## 4. Installer backend separation

### 4.1 Shared installer policy

The installer TUI and its `RYOKU_*` choice contract remain shared. The common
installer layer owns:

- user choices and validation;
- disk enumeration and destructive-action confirmation;
- partition layout intent, alongside geometry, LUKS intent, and filesystem
  selection;
- mounting the target root;
- hostname, locale, keyboard, timezone, username, and password intent;
- network facts and hardware facts;
- stage state, logs, dry-run behavior, and failure reporting;
- calling the selected target, boot, and driver backends in order.

The common layer must not name a native package, repository stanza, chroot tool,
initramfs command, or package manager.

### 4.2 InstallerBackend contract

```go
type InstallerBackend interface {
    ID() string
    ValidateLiveEnvironment(ctx context.Context, facts InstallFacts) error
    BootstrapTarget(ctx context.Context, target TargetRoot, plan InstallPlan, events EventSink) error
    ConfigureRepositories(ctx context.Context, target TargetRoot, plan InstallPlan, events EventSink) error
    InstallBase(ctx context.Context, target TargetRoot, components []ComponentID, events EventSink) error
    ConfigureSystem(ctx context.Context, target TargetRoot, plan InstallPlan, events EventSink) error
    InstallDesktop(ctx context.Context, target TargetRoot, components []ComponentID, events EventSink) error
    ConfigureServices(ctx context.Context, target TargetRoot, plan InstallPlan, events EventSink) error
    Finalize(ctx context.Context, target TargetRoot, plan InstallPlan, events EventSink) error
}
```

The exact language boundary may remain a shell orchestration contract during
migration. The important rule is that `arch-chroot` is an Arch implementation
detail, not the common target-execution API.

### 4.3 Target runner

A `TargetRunner` provides fixed-argv execution, environment, bind mounts, DNS,
and cleanup inside the target root. Arch can implement it with `arch-chroot`.
Debian can use its selected chroot mechanism. Callers request an operation in
the target; they do not construct the native wrapper command.

Target operations must declare whether `/proc`, `/sys`, `/dev`, EFI variables,
and network access are required. Cleanup is deterministic on success,
interruption, and failure.

### 4.4 Base install and desktop install are separate

The target backend first creates a bootable distribution base, then installs
the Ryoku desktop from the native Ryoku repository. This preserves recovery:

- failure before the base is complete is an installer failure;
- failure during desktop installation leaves a diagnosable base target;
- offline media carries both the distribution closure and the matching Ryoku
  repository snapshot;
- no target compiles Go, C++, QML plugins, or third-party packages.

### 4.5 Live image separation

Live media has a separate build backend:

- Arch image builder: current archiso/mkarchiso model.
- Debian image builder: Debian-native live-image tooling and package closure.

Both images carry the same TUI, translations, brand assets, stage protocol, and
backend-independent installer tests. They carry different native package
metadata, trust bootstrap, target backend, and bootable live root.

The image declares its target backend. Cross-installing Debian from an Arch
image, or Arch from a Debian image, is not required by this design.

### 4.6 Offline installation

Offline support is a backend contract, not a pacman repository trick. Each image
must contain:

- a complete native dependency closure for its supported install profiles;
- the exact signed Ryoku repository snapshot tested with that image;
- native repository metadata usable without network access;
- driver and firmware packages required by supported hardware profiles;
- an integrity and dependency-resolution gate run before image publication.

The installer records whether packages came from the offline snapshot or the
network, but the installed system is configured for its normal online channel
before first boot.

## 5. Boot backend separation

### 5.1 Shared boot policy

Ryoku's shared boot policy remains:

- UEFI validation and explicit Secure Boot support status;
- Limine as the Ryoku boot experience where supported;
- a known default kernel selected from installed kernels;
- boot entries derived from real installed artifacts;
- no stale entry whose kernel or initramfs is absent;
- Btrfs snapshot integration where the backend supports it;
- preflight protection against insufficient boot-space;
- initramfs rebuild after changes that require it;
- rollback guidance based on actual backend capabilities.

The mechanics of discovering kernels and producing images are backend-owned.

### 5.2 BootManager contract

```go
type KernelRecord struct {
    ID string
    Version string
    Package PackageName
    KernelImage string
    InitramfsImages []string
    ModulesDir string
    IsRunning bool
    IsDefault bool
}

type BootManager interface {
    Capabilities(ctx context.Context) (BootCapabilities, error)
    Kernels(ctx context.Context) ([]KernelRecord, error)
    DefaultKernel(ctx context.Context) (KernelRecord, error)
    PlanInitramfs(ctx context.Context, reason InitramfsReason) (BootPlan, error)
    ApplyInitramfs(ctx context.Context, plan BootPlan, events EventSink) error
    PlanBootEntries(ctx context.Context) (BootPlan, error)
    ApplyBootEntries(ctx context.Context, plan BootPlan, events EventSink) error
    ValidateBootSpace(ctx context.Context, plan BootPlan) (SpaceReport, error)
    SnapshotIntegration(ctx context.Context) (SnapshotBootState, error)
}
```

Boot plans list every file expected to be created, replaced, copied, or removed.
Apply refuses a plan if installed kernels or mount identity changed after the
plan was made.

### 5.3 Transaction lifecycle integration

Package transaction hooks are adapters into shared lifecycle intent:

- prepare live shell/power cutover;
- defer or coalesce initramfs work;
- install or remove kernel/driver content;
- rebuild required boot artifacts once;
- refresh boot entries;
- adopt the new live shell and services;
- leave a retryable marker if finalization fails.

Arch realizes these events with libalpm hooks and install scripts. Debian
realizes them with Debian package lifecycle and kernel/initramfs hook mechanisms.
The shared updater and doctor observe neutral markers and status, not hook file
formats.

### 5.4 Backend-specific boot facts

The Arch backend owns:

- `/usr/lib/modules/*/pkgbase` discovery;
- `/boot/vmlinuz-<pkgbase>` and Arch initramfs naming;
- mkinitcpio configuration and `ryoku-gpu-trim`;
- `limine-mkinitcpio`, `limine-update`, and Arch Limine packages;
- libalpm hook masks and ordering;
- `snap-pac` and `limine-snapper-sync` integration;
- CachyOS kernel selection.

The Debian backend owns:

- Debian kernel meta-package selection and installed-kernel discovery;
- the selected initramfs implementation and its configuration;
- Debian kernel post-install/remove integration;
- Limine installation and entry refresh for Debian image paths;
- snapshot hook equivalents or an explicit unsupported capability;
- Debian-native recovery for interrupted kernel or initramfs configuration.

Debian must select and support one default initramfs strategy. Runtime probing
may recognize alternatives, but it must not combine multiple tools in one boot
transaction without an explicit tested contract.

## 6. Hardware driver backend separation

### 6.1 Shared hardware facts

Hardware detection remains common and emits facts, not package names:

```go
type HardwareFacts struct {
    CPUVendor string
    GPUs []GPUFact
    FirmwareMode FirmwareMode
    SecureBoot SecureBootState
    NeedsBroadcomWiFi bool
    NeedsBroadcomBluetoothPatchram bool
    NeedsSOFFirmware bool
    Needs32BitGraphics bool
    IsLaptop bool
}
```

GPU facts include PCI ID, generation/classification, loaded DRM driver, boot VGA
status, render node, hybrid topology, and whether proprietary support was
requested. Existing sysfs and PCI probing can populate these facts.

### 6.2 Driver policy and native resolution

Shared driver policy converts facts and user choices into logical requirements:

```text
hardware.cpu.intel.microcode
hardware.gpu.amd.opengl
hardware.gpu.amd.vulkan
hardware.gpu.nvidia.proprietary
hardware.audio.sof-firmware
hardware.wifi.broadcom
hardware.graphics.32bit
```

`DriverManager` resolves and applies those requirements:

```go
type DriverManager interface {
    Analyze(ctx context.Context, facts HardwareFacts) (DriverRecommendation, error)
    Plan(ctx context.Context, recommendation DriverRecommendation) (DriverPlan, error)
    Apply(ctx context.Context, plan DriverPlan, events EventSink) error
    Verify(ctx context.Context, plan DriverPlan) (DriverReport, error)
}
```

`DriverPlan` embeds a package transaction plan plus modprobe, initramfs, service,
multiarch, and reboot actions. It never returns an arbitrary shell script.

### 6.3 Arch driver responsibilities

The Arch driver backend retains and formalizes current behavior:

- pacman package selection for AMD, Intel, Vulkan, firmware, and microcode;
- NVIDIA generation-to-package mapping, including supported legacy policy;
- official repository versus AUR/supplemental resolution;
- `[multilib]` enablement and `lib32-*` graphics packages;
- mkinitcpio configuration and NVIDIA recovery hook;
- DKMS/header matching against installed Arch kernels;
- CachyOS-specific kernel and driver variants;
- verification that a usable module exists before nouveau is blacklisted.

### 6.4 Debian driver responsibilities

The Debian driver backend owns:

- archive component policy required for firmware or proprietary drivers;
- Debian package mappings for Mesa, Vulkan, Intel media, SOF, microcode,
  Broadcom, and NVIDIA;
- NVIDIA branch policy supported by the chosen Debian release;
- DKMS and matching Debian kernel-header handling;
- Secure Boot module-signing/enrollment behavior or an explicit unsupported
  state;
- `i386` multiarch enablement and 32-bit graphics package resolution;
- Debian initramfs configuration and rebuild;
- conffile-safe modprobe changes;
- verification and rollback behavior when the proprietary module is unusable.

The Debian backend must not clone AUR recipes or build drivers on the installed
target as a substitute for native packaging.

### 6.5 Runtime hardware helpers

Runtime helpers that operate through sysfs, udev, D-Bus, logind, NetworkManager,
PipeWire, BlueZ, and DRM remain shared unless Debian proves a path or permission
difference. Package installation is removed from those helpers and delegated to
`DriverManager` or `PackageManager`.

## 7. Arch implementation responsibilities

The Arch implementation is not a compatibility shim. It is the owner of current
Arch behavior behind the new contracts and serves as the conformance reference.

### Package manager

- Preserve pacman version ordering, package ownership, installed/manual/foreign
  inventory, repo-qualified Ryoku targets, downgrade rules, and native solver
  plans.
- Preserve the separation between Ryoku updates, Arch updates, and AUR updates.
- Preserve file-conflict recovery, active lock safety, `.pacnew` reporting, and
  native diagnostics.
- Implement reviewed compositor removal with the same or stricter safety than
  current `wm.Reclaim` and `VerifyRemoval`.
- Treat AUR as an optional supplemental source, never as an implicit Debian-like
  universal lane.

### Repository

- Own `[ryoku]` pacman configuration, channel URLs, `pacman-key` trust,
  signature policy, sync database invalidation, and release metadata.
- Preserve stable/testing/frozen channels and private-mirror refusal behavior.
- Publish the Arch-resolved manifest.

### Installer and image

- Retain archiso, mkarchiso, pacstrap, arch-chroot, mirror ranking, offline
  pacman closure, AUR bake, and CachyOS variant inside the Arch backend.
- Preserve current dry-run matrix, offline integrity, noninteractive,
  dual-boot, VM, and staged-image gates.

### Boot and hardware

- Own mkinitcpio, Limine Arch packages, kernel `pkgbase`, libalpm hooks,
  snap-pac, CachyOS kernels, multilib, and Arch GPU package selection.
- Preserve the NVIDIA guard and live power/shell cutover behavior through
  neutral lifecycle markers.

### Release

- Continue building PKGBUILDs with makepkg, signing packages and pacman metadata,
  generating the pacman repository with repo-add, testing in Arch/CachyOS, and
  publishing the tested bytes.
- Continue Arch-specific dependency and delivery gates.

The first architectural milestone is complete only when Arch passes all current
behavioral gates through the new interfaces with no user-visible regression.

## 8. Debian implementation responsibilities

### Supported platform definition

- Declare exact Debian releases, architectures, repository components, and
  firmware/proprietary-driver policy.
- Reject untested derivatives and releases with a clear explanation.
- Record `BACKEND=debian` and release identity on every installed target.

### Package manager

- Implement inventory, manual package state, ownership, availability, native
  version comparison, update discovery, transaction planning, exact-plan
  application, local `.deb` inspection/install, lock state, and interrupted-dpkg
  recovery using Debian-native tools and data.
- Keep Ryoku-only updates separate from distribution upgrades.
- Model apt autoremove candidates as removals only when reviewed and approved.
- Translate Debian conffile state into neutral doctor findings.
- Provide structured progress without requiring QML to parse apt output.

### Repository and packages

- Build native `.deb` packages for every required first-party component and for
  third-party components Ryoku must guarantee but Debian does not provide.
- Provide desktop core and compositor-specific meta-packages with dependency,
  conflict, replacement, and upgrade semantics equivalent to product intent,
  not necessarily Arch package graph shape.
- Publish signed Debian repository metadata for stable, testing, and frozen
  releases, plus `release.json` and a Debian-resolved manifest.
- Install trust through a dedicated Ryoku archive keyring and scoped source
  definition; never weaken apt's global verification.
- Replace AUR dependencies with an explicit Debian source decision. Core
  functionality must not depend on best-effort target-side source builds.

### Installer and image

- Build a Debian-native live image containing the same TUI and common installer
  policy plus the Debian target backend.
- Bootstrap a minimal Debian base with Debian-native tooling, configure the
  selected archive suites/components, and execute target operations through the
  target runner.
- Install the signed Ryoku Debian meta-package, then run the same materialize and
  convergence contract.
- Produce and verify a complete offline Debian package closure.
- Add Debian-native container, VM, offline, dual-boot, service, and first-login
  gates.

### Boot and hardware

- Implement Debian kernel discovery, default selection, initramfs rebuild,
  Limine entries, package lifecycle integration, boot-space validation, and
  recovery.
- Map hardware facts to Debian firmware, microcode, Mesa/Vulkan, Intel media,
  Broadcom, NVIDIA, DKMS, headers, and multiarch packages.
- Verify service names, SDDM integration, portal availability, PipeWire/BlueZ,
  NetworkManager/iwd behavior, and systemd unit paths on every supported release.
- Report unavailable snapshot or proprietary-driver functionality through
  capabilities rather than silent degradation.

### Desktop compatibility

- Package a compatible Quickshell and all required QML modules, codecs, portals,
  fonts, themes, and helper binaries.
- Preserve the existing QML tree, shell IPC, config layout, compositor configs,
  and user-edit behavior.
- Adapt only backend-derived labels, package statistics, update commands, and
  availability. Do not fork the UI by distribution.

## 9. Migration order

Each stage is independently reviewable and keeps Arch releasable.

### Stage 0: freeze behavioral contracts

- Capture current Arch package query, update selection, rollback, channel,
  manifest, compositor removal, lock, `.pacnew`, boot, NVIDIA, and installer
  behavior as contract fixtures.
- Define neutral result and error schemas before moving call sites.
- Record current user-visible commands and JSON fields that QML consumes.

Exit criterion: the expected behavior is testable without requiring a live
package transaction for every unit test.

### Stage 1: introduce backend identity and read-only contracts

- Add backend selection and injected test backends.
- Define component IDs and the initial Arch component bindings.
- Route package inventory, ownership, availability, version comparison,
  repository inspection, and kernel inventory through read-only interfaces.
- Keep native mutation paths unchanged temporarily.

Exit criterion: no shared consumer needs to run pacman for a read-only fact, and
Arch output remains unchanged.

### Stage 2: move Arch mutations behind contracts

- Route update planning/application, manifest convergence, repository channel
  changes, doctor package repair, compositor switching, Extras, and local
  package installation through the Arch backend.
- Add structured progress and typed error mapping.
- Move native lock and config-upgrade handling behind `PackageManager`.

Exit criterion: pacman and yay invocations exist only in the Arch backend,
Arch installer/release tooling, and explicitly Arch-only tests.

### Stage 3: separate Arch boot and hardware implementations

- Route kernel inventory, initramfs rebuild, Limine reconciliation, snapshot
  integration, and driver installation through `BootManager` and
  `DriverManager`.
- Convert libalpm actions into adapters for neutral lifecycle markers.
- Preserve existing NVIDIA and live cutover safety.

Exit criterion: shared updater, doctor, Hub, and hardware policy contain no
mkinitcpio, `pkgbase`, multilib, or Arch driver package names.

### Stage 4: split installer and live-image backends

- Separate common disk/TUI policy from the Arch target backend.
- Introduce the target runner and live-image builder contracts.
- Move current pacstrap, arch-chroot, archiso, offline repository, and CachyOS
  behavior into the Arch implementation without changing outcomes.

Exit criterion: the existing Arch ISO and installation gates pass through the
separated architecture.

### Stage 5: establish Debian packaging and repository

- Resolve the component catalog for the first supported Debian release.
- Build required `.deb` packages and meta-packages.
- Publish a signed test repository, release descriptor, and resolved manifest.
- Run install, upgrade, downgrade, removal, ownership, conffile, and trust tests
  in Debian containers before runtime integration.

Exit criterion: a clean Debian base can install and remove the packaged desktop
from the tested repository with no source build on the target.

### Stage 6: implement Debian runtime backends

- Implement `PackageManager` and `RepositoryManager` first.
- Add Debian boot and driver implementations with native integration tests.
- Run updater, doctor, manifest, compositor switch, Extras, and recovery
  conformance suites against Debian.

Exit criterion: an already-installed Debian system can update, verify, repair,
switch compositors, and manage supported hardware through neutral Ryoku
services.

### Stage 7: implement Debian installer and offline image

- Add Debian base bootstrap, target configuration, service enablement, desktop
  install, boot finalization, and offline closure.
- Build the Debian live image with the existing TUI and common installer policy.
- Add real unattended VM installation and first-session verification.

Exit criterion: the published Debian image performs an offline install and boots
to the same Ryoku desktop, with update and doctor operational after first boot.

### Stage 8: adapt backend-derived presentation and release gates

- Replace hard-coded Arch update commands, package statistics, install-date
  fallback, AUR labels, and Arch product copy with neutral backend data.
- Keep layout, components, navigation, motion, and styling unchanged.
- Generate translations from updated source strings.
- Require both backend delivery matrices before a release is advertised for both.

Exit criterion: no shared QML or Go service names a native package manager,
repository format, or initramfs tool, except diagnostic data explicitly returned
by the active backend.

## 10. Risk assessment

| Risk | Likelihood | Impact | Required control |
|---|---|---|---|
| A generic interface hides important native transaction differences | High | Critical | Use semantic requests, backend capabilities, native plans, and typed unsupported results |
| Removal plan changes between review and execution | Medium | Critical | Fingerprint and revalidate every destructive plan; abort on expansion |
| Ryoku update accidentally upgrades the Debian base or kernel | Medium | Critical | Enforce `TransactionScopeRyoku`; test package-source selection and held/newer packages |
| Arch behavior regresses during extraction | High | High | Move Arch first, preserve output fixtures, and keep all current Arch gates blocking |
| Debian package mapping is incomplete or silently skips a requirement | High | High | Component catalog requires an explicit binding, optional decision, or unsupported decision for every component |
| Required Quickshell/compositor versions are absent from supported Debian | High | High | Publish compatible native packages in the Ryoku Debian repository and test ABI/import compatibility |
| Boot tooling and kernel naming produce stale or missing entries | Medium | Critical | Backend-owned kernel inventory, file-level boot plans, VM reboot tests, and boot-space gates |
| NVIDIA package, DKMS, initramfs, and Secure Boot state diverge | High | Critical | One driver plan spans packages, headers, module verification, initramfs, signing status, and rollback |
| Debian non-free firmware policy is undeclared | Medium | High | Declare repository components and consent/policy in supported-platform definition; gate hardware profiles |
| Native package hooks do not reproduce live shell/power cutover | Medium | Critical | Neutral durable lifecycle markers, old-to-new package upgrade tests, and failure retry tests |
| Repository trust bootstrap or key rotation locks out updates | Medium | Critical | Independent keyring bootstrap, dual-key rotation tests, signed metadata validation, no global trust relaxation |
| Offline closure differs from the online repository | Medium | High | Build from one resolved manifest, validate closure, and install-test the exact image payload |
| Debian conffile handling overwrites administrator changes | Medium | High | Model conffile state, use native policy, and limit doctor to provably safe convergence |
| apt autoremove removes unreviewed user software | Medium | Critical | Never execute unreviewed autoremove; compare exact native plan immediately before apply |
| Unsupported Debian derivative is accepted by broad detection | High | High | Require explicit backend/release support; treat `ID_LIKE` only as a hint |
| AUR-only features disappear without an honest product decision | High | Medium | Classify every supplemental component and report unavailable optional features through capabilities |
| Package names leak back into WM providers or QML | Medium | Medium | Providers declare component IDs; repository/package data is rendered from neutral models |
| Split manifests lose the distinction between user removal and failed delivery | Medium | High | Store baselines by logical component ID plus resolved native names and versioned backend manifest |
| Mixed backend artifacts are installed | Low | Critical | Validate backend ID, architecture, repository origin, and artifact metadata before every install |
| Runtime command output changes break parsers | High | High | Parse native machine-readable state where available and isolate unavoidable text parsing inside backend tests |
| Service names or enablement defaults differ on Debian | High | Medium | Backend service map plus clean-install and upgrade tests for every supported release |
| Phase 2 expands into a desktop redesign | Medium | High | Treat QML, Quickshell, shell IPC, compositor configs, and materialization as invariants; change only backend-derived data |

## Acceptance criteria for the architecture

The architecture is ready for implementation planning when all of the following
are agreed:

- one backend is selected explicitly and cannot silently fall back;
- package names are backend resolutions of stable component IDs;
- package and repository responsibilities are separate;
- update scopes preserve the Ryoku/system lane boundary;
- destructive package operations have exact review and revalidation semantics;
- installer common policy names no native package manager or target wrapper;
- boot and driver changes are planned as coherent native transactions;
- Arch retains all current behavior behind the contracts;
- Debian owns native packages, repository trust, installer, boot, firmware, and
  driver policy rather than emulating Arch tools;
- QML and Quickshell remain consumers of neutral Ryoku data and commands;
- both release lanes independently prove that tested bytes are published bytes;
- no target build toolchain is required for a supported install or update.

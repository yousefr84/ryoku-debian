# Backend selection and platform detection map

## Status and scope

This document maps the current code that detects a distribution, infers an
installation mode, records platform-adjacent markers, or assumes Arch Linux.
It is a focused companion to `docs/migration/code-architecture-map.md`,
`docs/migration/arch-dependency-map.md`, and the selection contract in
`docs/migration/backend-design.md`.

This is an inventory, not an implementation. It does not add a selector,
change an installer, or make an existing caller backend-aware.

The central finding is:

- the standalone existing-system installer detects Arch and Debian families;
- the ISO installer is selected as Arch by the image that contains it, not by
  inspecting the target platform;
- the installed runtime distinguishes a source checkout from a packaged
  install and reads Ryoku release/channel metadata, but does not detect or
  validate the host distribution;
- the backend contract has identity types and an Arch provider, but no
  production selector or application consumer; and
- most production code therefore selects Arch implicitly by executing pacman,
  reading pacman paths, or consuming Arch package metadata.

## 1. Current selection paths

| Context | File and function | Current behavior | Migration recommendation |
|---|---|---|---|
| Curl bootstrap | `ryoku-shell-installer/install.sh`, `main` | Rejects `/etc/NIXOS`, then chooses `arch` when `pacman` is on `PATH` or `debian` when `apt-get` is on `PATH`. It checks `x86_64` and systemd. `/etc/os-release` is warning-only, so tool presence wins if it disagrees with `ID`/`ID_LIKE`. | Keep the bootstrap limited to download prerequisites. Have the downloaded installer perform the authoritative selection; do not let command precedence establish installed backend identity. |
| Existing-system installer gate | `ryoku-shell-installer/main.go`, `main` | Calls `detectHostDistro`; accepts only a non-nil Arch- or Debian-family result, then independently requires `x86_64` and a booted systemd. | Replace the local family gate with the shared selector once one exists. Preserve architecture and init-system checks as explicit support constraints, not package-manager inference. |
| Existing-system distro resolution | `ryoku-shell-installer/distro.go`, `detectHostDistro`, `detectDistro` | Reads `/etc/os-release`; maps `ID=arch` or any `ID_LIKE` containing `arch` to `archLinux`, and the equivalent Debian values to `debianLinux`. It mutates the package-global `activeDistro`. Version and derivative support are not checked. | Reuse one strict OS-release parser and registry. Treat `ID_LIKE` as a discovery hint and require an explicit supported derivative/version declaration. Inject the selected backend into the engine instead of mutating `activeDistro`. |
| Existing-system fact collection | `ryoku-shell-installer/detect.go`, `parseOSRelease`, `detect` | Parses `ID`, `ID_LIKE`, and `PRETTY_NAME`; separately records `uname -m`, whether `pacman` exists, and `archLike`. It calls `detectDistro`, then runs many pacman-only probes only when pacman is present. | Return normalized platform facts independently from backend selection. Do not keep both `archLike` and a selected distro as separate decision paths. Package facts should come from the selected backend. |
| Live ISO recognition | `installation/tui/system.go`, `liveDisk` | Treats `/run/archiso` as a live-medium marker only to hide the boot device from the disk picker. It does not select an installer backend. | Keep boot-medium discovery separate from distribution selection. A future live-image launcher should identify the image backend explicitly and pass it to the TUI/backend boundary. |
| Live installer handoff | `installation/tui/system.go`, `startInstall`, `installEnv` | Always launches `ryoku-install` (or the test/developer `RYOKU_BACKEND` override) and sends the common `RYOKU_*` environment. No distro/backend ID is part of the contract. | Preserve the process and progress-sentinel boundary. Select a registered installer backend before launch and include an explicit backend identity in the validated handoff; do not infer it from the executable name. |
| Full-system installer | `installation/backend/ryoku-install`, top-level defaults and `main` | Is explicitly the Arch backend. It sources pacstrap, pacman repository, AUR, mkinitcpio, and Arch boot helpers. `RYOKU_VARIANT` chooses plain Arch versus CachyOS behavior, not Arch versus Debian. | Retain this script as the Arch installer implementation. Put cross-distribution selection outside it; do not add Debian branches throughout these shell libraries. |
| Installed backend contract | `ryoku/backend/identity.go`, `PlatformIdentity`; `ryoku/backend/backend.go`, `SystemBackend` | Defines Arch and Debian IDs plus distribution, architecture, installed-marker, marker-match, channel, and installation-mode fields. Nothing currently populates these fields from the machine. | Use this as the normalized selection result. Keep detected OS facts and installed-marker identity distinct so mismatches can fail closed. |
| Arch backend constructor | `ryoku/backend/arch/provider.go`, `New` | Constructs a partial backend with a statically assigned `BackendID=arch` and `DistributionID=arch`; only read-only package facts are wired. It performs no detection and has no production callers. | Keep provider construction deterministic. A selector should validate the machine first and then call this constructor; the provider must not self-select by probing `PATH`. |

There is currently no runtime path equivalent to “read marker, read
`/etc/os-release`, verify, select one backend.” The desired order is documented
in `docs/migration/backend-design.md`; this table records that it has not yet
been wired.

## 2. Installer detection details

### 2.1 Existing-system installer

| File and function | Current behavior | Migration recommendation |
|---|---|---|
| `ryoku-shell-installer/distro.go`, `archLinux`, `debianLinux` | Stores command vectors, package renames, build dependencies, and the `fromSource` policy in one `distro` value. Debian uses apt for base packages but builds Ryoku from source because no Debian Ryoku repository is defined. | Treat this table as migration evidence, not the final backend. Move native package facts/transactions, repository availability, and component-name resolution behind the shared contracts. |
| `ryoku-shell-installer/distro.go`, `installedPkg`, `installed`, `engine.d` | Queries pacman or `dpkg-query` through the selected global. `engine.d` falls back to `activeDistro`, whose initial value is Arch. | Require an explicitly selected backend on every engine. Remove the silent Arch fallback after all construction paths inject a backend. |
| `ryoku-shell-installer/detect.go`, `detect` | Uses OS release for family selection, but retains separate pacman presence and Arch-family flags. Pacman gates rival-package, blocker-package, installed-desktop, and desktop-environment detection. | Split portable machine discovery from package inventory. Ask backend services for package facts so Debian receives equivalent detection rather than skipping Arch-only branches. |
| `ryoku-shell-installer/main.go`, `needsManjaroAck` | Special-cases `ID=manjaro` because the `[ryoku]` repository targets Arch current; headless mode refuses unless explicitly overridden. | Express supported derivatives and repository compatibility in backend support data. Do not let UI code grow one distribution-name conditional per derivative. |
| `ryoku-shell-installer/engine.go`, plan steps including `stepRepo`, `stepPackages`, `stepBuild`, `stepAUR`, and `stepVerify` | The plan has a limited `fromSource` split, but the packaged route assumes `[ryoku]`, pacman conflict semantics, Arch driver scripts, and AUR. Verification checks the pacman stanza and `ryoku-desktop` package on Arch. | Keep the resumable plan and user journey. Replace each native operation with an installer/backend capability; verify logical components and backend identity rather than one native package/stanza. |
| `ryoku-shell-installer/lifecycle.go`, `statePath`, `loadState`, `markStepDone` | Uses `~/.local/state/ryoku/shell-install-state.json` only as a resume journal. It contains completed steps, backup path, and time, but no selected distro/backend. | Record the selected backend in resumable state before mutations begin and reject a resume if current detection no longer matches. Do not treat this journal as the installed-system marker. |
| `ryoku-shell-installer/lifecycle.go`, `runUninstall` | Uses `activeDistro`, but only Arch removes Ryoku packages and the `[ryoku]` pacman stanza; source-built Debian cleanup relies mainly on backup restore. | Delegate uninstall package/repository work to the recorded installed backend. Preserve the backup-chain restoration as backend-neutral policy. |

### 2.2 Full-system installer

| File and function | Current behavior | Migration recommendation |
|---|---|---|
| `installation/tui/system.go`, `detectHardware` and other `sys*` probes | Detects hardware, disks, locale, time zone, network, and firmware from Linux/systemd interfaces. It does not detect a distribution because it runs inside the Ryoku Arch ISO. | Keep portable probes in the shared TUI layer. Pass their results to the selected installer backend without equating the live image's distribution with target implementation. |
| `installation/tui/system.go`, `offlineRepo` | Detects a usable baked repository at `/usr/share/ryoku/offline/repo` by directory plus `offline.db*`, then sets `RYOKU_ONLINE=0`. The database shape is pacman-specific. | Replace the database glob with image-backend metadata or an installer-backend validation call. Offline policy can remain common; repository validation is native. |
| `installation/backend/ryoku-install`, `RYOKU_VARIANT` default | Reads `/usr/share/ryoku/variant`, falling back to `plain`; accepted behavior elsewhere is `plain` or `cachyos`. This is a kernel/repository variant within the Arch backend. | Preserve variant as a backend-specific option. Do not overload it as the future backend ID. |
| `installation/backend/lib/preflight.sh`, `ryoku_preflight` | Validates root, UEFI, Secure Boot state, disk shape/size, and presence of `system/packages/base.packages`. It assumes later stages are pacstrap-based but does not validate host distro. | Keep destructive safety gates common where possible. Move payload/package-set validation to the selected installer backend. |
| `installation/backend/lib/pacstrap.sh`, `ryoku_pacstrap`; `lib/deploy.sh`, `ryoku_deploy`; `lib/aur.sh`, `ryoku_aur` | Installs Arch with pacstrap, configures/trusts the pacman repository, installs the Ryoku package set, and builds AUR packages. | Keep these files wholly Arch-owned. A Debian installer should have its own debootstrap/apt/dpkg implementation rather than conditional branches here. |
| `installation/backend/lib/chroot.sh`, `ryoku_configure`; `lib/bootloader.sh`, `ryoku_bootloader`; `lib/snapshots.sh`, `ryoku_snapshots` | Uses `arch-chroot`, mkinitcpio/libalpm hook behavior, Arch kernel package metadata, and pacman-triggered snapshot integration. | Route target execution, initramfs/kernel discovery, boot refresh, and transaction snapshot hooks through the installer and boot backends. |

## 3. Installed runtime detection and implicit Arch selection

The runtime does not currently detect a distribution. The following paths
decide adjacent questions or select Arch by using Arch-native operations.

| File and function | Current behavior | Migration recommendation |
|---|---|---|
| `ryoku/cli/internal/sys/repo.go`, `ResolveRepo`, `SourceTracked` | Determines source mode from `RYOKU_REPO`, `~/.local/state/ryoku/repo`, and `RYOKU_CHANNEL` in `~/.config/environment.d/ryoku.conf`. Absence means the packaged path. This is delivery-mode detection, not distro detection. | Keep source/package mode as a separate dimension in `PlatformIdentity`. Do not infer a distro from source mode or from the checkout directory name. |
| `ryoku/cli/internal/sys/release.go`, `RyokuServer`, `PackagedChannel` | Parses the `[ryoku]` `Server` line in `/etc/pacman.conf` and maps known URLs to stable, testing, or a release tag. | Move repository inspection and channel mutation to `RepositoryManager`. Channel is release state, not proof of platform identity. |
| `ryoku/cli/internal/sys/release.go`, `ReadRelease` | Parses `/etc/ryoku-release` into release, codename, channel, version, commit, and date. It does not contain or validate a backend ID. Missing files silently produce an empty value. | Extend the root-owned marker with backend identity and schema/version fields. Selection should distinguish absent legacy marker from malformed or mismatched marker and apply an explicit migration policy. |
| `ryoku/cli/internal/sys/sys.go`, `PkgInstalled`, `InstalledVersion` | Calls `pacman -Q` directly. Every caller therefore assumes Arch even though the helpers have generic names. | Replace with injected `PackageFacts`; query a logical Ryoku desktop component rather than hard-coding the Arch package as identity. |
| `ryoku/cli/internal/updater/version.go`, `Version`, `ReleaseName`, `versionParts` | Chooses checkout metadata when `ResolveRepo` succeeds; otherwise reads `/etc/ryoku-release` and parses the installed pacman version returned by `InstalledVersion`. | Keep checkout-versus-package presentation policy, but take packaged version/release facts from selected backend identity and release metadata. |
| `ryoku/cli/internal/updater/update.go`, `Update` and package stages; `ryokuset.go`, `installedRyokuSet`, `repoServedSet`; `manifest.go`, `pacmanSet` | Executes pacman/yay/vercmp, reads pacman locks and ownership, and models the `[ryoku]` package lane. No platform guard precedes these calls. | Make the CLI composition root select one verified backend before updater construction. Preserve update ordering while replacing native facts and transactions. |
| `ryoku/cli/internal/doctor/doctor.go`, `reconcilers`, `reconcilePacmanLock`; `reconcile_pacman.go`; `reconcile_multilib.go`; `reconcile_manifest.go` | Doctor treats pacman lock/config/database/keyring, multilib, package ownership, orphans, and `.pacnew` artifacts as universal system health. | Keep reconciler policy and ordering; inject normalized package/repository facts and gate native remedies on backend capabilities. |
| `ryoku/wm/reclaim.go`, `pacmanInstalled`, `pacmanRemovalOnce`, `pacmanInstalledSizes` | Compositor switching plans and revalidates package removal with pacman. | Inject the small package planning interface needed by reclaim. The compositor seam must not select the system backend. |
| `ryoku/hub/backend/wm.go`, `pkgAvailable`; `gpuapply.go`, `pacmanInstall`, `pkgInstalled` | Uses pacman for compositor availability and GPU package changes, with AUR helpers in the GPU path. | Keep Hub commands/UI stable and supply backend-neutral availability and driver operations from the service layer. |
| `ryoku/rashin/backend/index.go`, `pacmanVersion` and package inventory sections; `quicktools.go`, package tools | Builds machine knowledge and quick tools from pacman/yay output. | Feed normalized package facts and backend-specific command guidance into Rashin. Do not let the agent infer platform from which command happens to exist. |
| `ryoku/shell/quickshell/shell/modules/launcher/shared/providers/packages/Packages.qml`, package provider command | Calls `gpk` with pacman/AUR providers. `ryoku/hub/quickshell/pages/DictationPage.qml` also requests an AUR install. | Keep QML backend-neutral. Expose package search/install through an existing Go process boundary with backend-derived providers. |
| `ryoku/hub/quickshell/pages/ProfilePage.qml` and `ryoku/shell/quickshell/shell/modules/bar/barstyles/qsbar/modules/ParticleStream.qml` | Read `/var/log/pacman.log` for install date or transaction animation. | Supply normalized installation and transaction-event facts. A log path must not double as platform detection. |
| `system/extras/ryoku-pkg-*`, `ryostore-install`; `system/hardware/drivers/*.sh` | These shipped helpers call pacman, yay, pacman repository tools, mkinitcpio, and Arch package names directly. | Move native operations into Arch backend actuators or replace them with stable Ryoku service commands that dispatch through the selected backend. |

## 4. Arch assumptions outside detection

These are not selectors, but they make “Arch” the effective default whenever no
selector exists.

| Area | File and function | Current behavior | Migration recommendation |
|---|---|---|---|
| Package namespaces | `system/packages/*.packages`; `ryoku/cli/cmd/ryoku-manifest/main.go`, manifest generation | The canonical package sets and release manifest contain Arch package names and AUR lanes. | Introduce logical component IDs at the cross-distribution boundary; keep native names in backend catalogs. |
| Repository paths | `ryoku/cli/internal/sys/release.go`, repository helpers; installer deploy helpers | Reads and writes `/etc/pacman.conf`, `/etc/pacman.d`, `/var/lib/pacman/sync`, and `$arch` repository URLs. | Confine these paths and syntax to the Arch repository implementation. |
| Package database and artifacts | Updater/doctor package helpers; `ryoku/cli/internal/ryotunesrelease/client.go` | Assumes pacman queries, `vercmp`, `.pkg.tar.*`, `.PKGINFO`, `.pacnew`, and `/var/lib/pacman/db.lck`. | Normalize package facts, comparison, artifacts, config-conflict events, and lock state in PackageManager. |
| Kernel/initramfs | `installation/backend/lib/bootloader.sh`; `system/boot/mkinitcpio/`; doctor `reconcile_initramfs.go` and `reconcile_limine_images.go` | Uses mkinitcpio, `/usr/lib/modules/*/pkgbase`, Arch kernel names, libalpm hooks, and Arch image naming. | Keep desired boot policy shared; implement kernel discovery and initramfs/boot refresh per backend. |
| Driver fulfillment | `installation/backend/lib/drivers.sh`; `system/hardware/drivers/{amd,intel,nvidia,vulkan}.sh`; Hub `gpuapply.go` | Hardware probing is mostly portable, but fulfillment is Arch package and mkinitcpio logic. | Keep hardware facts/policy common and move native package/firmware/initramfs work to `DriverManager`. |
| Full-system build | `installation/iso/build.sh`; `offline-repo.sh`; `profiledef.sh` | Builds an archiso image, stages pacman configuration, resolves a pacman/AUR closure, and invokes mkarchiso. | Keep common branding, TUI assets, provenance, signing, and offline requirements; create distinct image backends. |
| Package release | `release/repo/build-repo.sh`, top level; `release/packages/*/PKGBUILD`, `package` functions | Builds signed `.pkg.tar.zst` files with makepkg, publishes a repo-add database, and installs the release marker through `ryoku-desktop`. | Keep the Arch publisher intact behind a build-time backend. Add separate Debian packaging/repository production; share release identity inputs and promotion gates. |
| User-visible identity | Welcome, Updates, Profile, Fastfetch, Rashin, recovery/help source files listed in `docs/migration/arch-dependency-map.md` section 4.4 | Product copy names Arch, pacman, and AUR even where the UI otherwise has no native package responsibility. | Render backend-derived neutral copy or explicit selected-platform copy after selection exists. Update source strings first; generated translations follow. |

The exhaustive command/path inventory remains in
`docs/migration/arch-dependency-map.md`; the rows above identify the places that
affect selection or would immediately consume a selected backend.

## 5. Marker files and state

No current file is an authoritative installed-backend marker.

| Marker or state | File and function | Current behavior | Migration recommendation |
|---|---|---|---|
| `/etc/ryoku-release` | `release/packages/ryoku-desktop/PKGBUILD`, `package`; read by `ryoku/cli/internal/sys/release.go`, `ReadRelease` | Pacman-owned release metadata: `RELEASE`, `NAME`, `CHANNEL`, `VERSION`, `COMMIT`, and `DATE`. It is the best existing root-owned identity anchor, but says nothing about backend/distribution. | Extend this marker, or a schema-compatible root-owned companion, with the installed backend ID. The installer/package must write it atomically and runtime selection must verify it against OS release. |
| `/etc/os-release` | `ryoku-shell-installer/detect.go`, `detect`; `distro.go`, `detectHostDistro`; bootstrap shell sourcing | Supplies distro facts only to the existing-system installer and doctor reports. Installed runtime lifecycle commands do not use it. | Parse it once through shared code. Preserve raw distribution ID/version as facts; do not accept `ID_LIKE` alone as proof of support. |
| `/usr/share/ryoku/variant` | `installation/iso/build.sh`, staging; `installation/backend/ryoku-install`, default initialization | Contains `plain` or `cachyos` and changes Arch kernel/repository behavior. It exists in the live image payload, not as installed backend identity. | Keep it scoped to the Arch image/backend. If target variant must persist, record it as a separate backend option, not the backend ID. |
| `/usr/share/ryoku/.payload` | `installation/iso/build.sh`, staging; `installation/backend/lib/deploy.sh`, `ryoku_deploy_version_skew` | Records ISO payload commit, date, version, and codename so a stale ISO can warn when the repository serves a different desktop build. | Retain as image provenance. Add image backend/schema if needed, but never use payload provenance as the installed target's authoritative backend marker. |
| `/run/archiso` | `installation/tui/system.go`, `liveDisk` | Indicates the running live environment for boot-disk exclusion. | Treat as an Arch image implementation detail. Use explicit image metadata for installer selection. |
| `/etc/NIXOS` | `ryoku-shell-installer/install.sh`, `main` | Early special-case rejection before generic package-manager detection. | Replace distro-specific sentinel growth with registry-based unsupported-platform reporting in the downloaded installer. |
| `/etc/pacman.conf` `[ryoku]` stanza | `ryoku/cli/internal/sys/release.go`, `RyokuServer`; standalone installer engine/lifecycle | Establishes the active packaged channel and is also used as evidence that the package route is configured. | Keep as Arch repository state only. It may corroborate an Arch marker, but it cannot select the backend. |
| Installed `ryoku-desktop` package | `ryoku/cli/internal/sys/sys.go`, `PkgInstalled`; standalone installer `detect`/`verify` | Used as “Ryoku already installed” or packaged-install evidence. The query itself requires pacman. | Replace with marker plus backend package/component facts. A native package's presence should validate installation state after backend selection. |
| `~/.local/state/ryoku/repo` and `~/.config/environment.d/ryoku.conf` | `ryoku/cli/internal/sys/repo.go`, `recordedRepo`, `TrackedChannel`, `ResolveRepo`, `SourceTracked` | Records source-checkout location and source channel. | Preserve as delivery-mode state. It must not override or substitute for root-owned backend identity. |
| `~/.local/state/ryoku/shell-install-state.json` | `ryoku-shell-installer/lifecycle.go`, `statePath`, `loadState`, `markStepDone` | Resume journal for the existing-system installer; deleted after success. | Add selected backend to resumable state for consistency checking, but do not retain it as the installed marker. |
| `/etc/ryoku/default-kernel` | `installation/backend/lib/bootloader.sh`, `ryoku_boot_limine_conf`; doctor boot reconcilers | Records the chosen kernel package ID (`linux` or `linux-cachyos`) for boot policy. | Interpret through the selected BootManager or migrate to a logical kernel ID. It is not a distribution marker. |
| `/var/lib/ryoku/*` service/update markers | Package scriptlets, power/network helpers, updater boot guard | Records one-time service enablement, kill-switch state, power cutover, and pending/healthy boots. | Leave concern-specific. These markers should never participate in backend selection. |

## 6. Release metadata

Release identity and platform identity are currently adjacent but separate.

| Metadata | File and function | Current behavior | Migration recommendation |
|---|---|---|---|
| Root `VERSION` and `CODENAME` | `bin/ryoku-release-version`; release scripts; ISO/repo builders | Define the product version line and release name shared by packages and images. | Keep backend-neutral and feed the same values into every native package/image publisher. |
| Package version and build identity | `release/repo/build-repo.sh`, top-level setup | Calculates `RYOKU_PKGVER`, `RYOKU_RELEASE`, `RYOKU_CHANNEL`, and `RYOKU_NAME`; exports them to PKGBUILDs. | Separate product release identity from native artifact version formatting. Each publisher should map one release identity to its native version rules. |
| Installed release marker | `release/packages/ryoku-desktop/PKGBUILD`, `package` | Writes `/etc/ryoku-release` from build environment and git state. Ownership by the umbrella package keeps it synchronized with the installed Arch release. | Make every native desktop package write the same marker schema, including backend ID and schema version, from shared release inputs. |
| Channel `release.json` | `release/repo/build-repo.sh`, final metadata stage; read by `ryoku/cli/internal/updater/release.go`, `channelServes` | Publishes schema 1 release/name/channel/version/commit/date beside the pacman database. Runtime URLs are hard-coded to the Arch `x86_64` repository layout. | Keep the normalized fields but publish them per backend/repository architecture, or add backend/artifact coordinates. RepositoryManager should resolve their location. |
| Channel `manifest.json` | `ryoku/cli/cmd/ryoku-manifest/main.go`; `release/repo/build-repo.sh`; updater/doctor manifest readers | Describes release lanes using current Arch package names: base, dev, hardware, AUR, first-party, compositor, and provisioned. | Split backend-neutral logical components from backend-native package resolution. Preserve release convergence policy and baseline semantics. |
| ISO payload provenance | `installation/iso/build.sh`, `.payload` generation | Captures the exact checkout commit/version/name baked into the live image. | Keep common provenance and add explicit image/backend schema when multiple image implementations exist. |
| ISO variant marker | `installation/iso/build.sh`, variant staging; `ryoku-install`, variant read | Selects plain versus CachyOS artifacts inside the Arch image. | Keep as an Arch-specific release/image variant. Do not reinterpret old `plain` values as a backend. |
| Download manifest | `bin/ryoku-iso-manifest`, main body | Derives date/tracking ID/architecture/ref from an Arch ISO filename and emits schema 1 metadata with `variant`, checksums, signature, URLs, and release identity. | Add an explicit backend/image kind before publishing Debian media. Avoid filename parsing as the only source of platform identity. |
| Release ledger | `bin/ryoku-release-ledger`; published `releases/index.json` | Aggregates frozen release metadata and per-variant ISO images. Current image variants are plain Arch and CachyOS. | Index images by backend plus variant while retaining one product release ledger. Runtime package selection should not infer its backend from available image entries. |

## 7. Migration constraints exposed by the map

The inventory implies these requirements for later implementation:

1. There must be one production selection point per process composition root,
   not repeated `PATH` probes in package operations.
2. The root-owned installed marker and `/etc/os-release` must be read
   independently and checked for a supported combination.
3. Architecture, installation mode, release channel, image variant, and distro
   family are separate facts. None should be overloaded as the backend ID.
4. A missing legacy marker needs an explicit migration path. A malformed marker
   or marker/OS mismatch must not silently fall back to Arch.
5. `ID_LIKE` can nominate candidates but cannot grant derivative support.
6. Tests should inject a backend or fixture platform identity. Production code
   should select once and pass the result to updater, doctor, installer, WM
   reclaim, Hub package operations, and driver fulfillment.
7. Existing process boundaries remain valid: QML and the shell should continue
   to consume `ryoku`, `ryoku-hub`, and `ryoku-shell` protocols rather than
   selecting a backend themselves.
8. The current Arch installer, release publisher, package helpers, and boot
   mechanics should become Arch implementations, not mixed Arch/Debian files.

## 8. Selection gap summary

| Required fact | Current source | Gap |
|---|---|---|
| Detected distribution ID/version | `/etc/os-release`, existing-system installer only | Not read by installed runtime; version support is not validated. |
| Native architecture | `uname -m` in standalone installer; hard-coded `x86_64` in release/image paths | Not normalized at runtime or reconciled with repository architecture. |
| Installed backend ID | None | `PlatformIdentity.InstalledBackendID` exists, but no marker field or reader populates it. |
| Selected backend | Standalone installer's global `activeDistro`; static `arch.New` constructor | No shared registry/selector and no production consumer of `SystemBackend`. |
| Installation mode | Checkout/env state versus pacman package assumptions | Detected independently in CLI but not combined with platform identity. |
| Release/channel | `/etc/ryoku-release`, `/etc/pacman.conf`, remote `release.json` | Arch repository syntax is embedded in otherwise generic release APIs. |
| Marker agreement | None | `MarkerMatches` exists in the contract but is never calculated or enforced. |

Until these gaps are closed, “backend selection” in the installed product is
effectively the absence of selection followed by Arch-native execution.

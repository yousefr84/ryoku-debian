# System backend code architecture map

## Status and scope

This document maps the current implementation boundaries that matter to the
`SystemBackend` design in `docs/migration/backend-design.md`. It is an
implementation-planning companion to `docs/migration/arch-dependency-map.md`.
It records where behavior lives now, where native package and installer tools
cross into application code, and which existing owners should eventually call
the backend contracts.

This is not a redesign. In particular:

- QML, Quickshell, the desktop shell, the compositor provider protocol, and the
  visual architecture remain in place.
- The existing `ryoku`, `ryoku-hub`, `ryoku-shell`, `ryostore`, `ryogami`, and
  `ryoku-wm-*` process contracts remain the presentation boundary.
- Update ordering, doctor policy, installer choices, and hardware policy remain
  owned by their current application services. Only distribution-specific
  execution and facts move behind a backend.
- Vendored third-party Go packages are outside this map. "All Go packages"
  below means every first-party package directory under the repository's
  `go.mod` module roots.

No `SystemBackend` package exists yet. References below to a future dependency
identify a migration target, not current code.

## 1. Go module and package inventory

The repository has 14 first-party Go modules. Most are deliberately small
command modules. The two reusable library modules are `ryoku-i18n` and
`ryoku-wm`; `ryoku-cli` also contains internal application packages used by its
two commands.

| Module | Directory | First-party packages | Current role | Backend relevance |
|---|---|---|---|---|
| `ryoku-tui` | `installation/tui` | `main` | Bubble Tea ISO installer front end, machine probing, install choices, and the `RYOKU_*` handoff | Installer front end should remain backend-neutral; its installer client boundary changes |
| `ryoku-shell-install` | `ryoku-shell-installer` | `main` | Converts an existing installation into Ryoku, with detection, planning, resumable steps, backup, install, doctor, and verification | A second installer orchestrator and a major PackageManager/RepositoryManager/DriverManager consumer |
| `ryostore` | `ryoku/apps/ryostore/backend` | `main`; `compat` | Ryostore catalog, receipts, product transactions, provider routing, and compatibility versioning | Product policy remains; privileged bundle package work currently crosses through shell actuators |
| `ryovm-fetch` | `ryoku/apps/ryovm/fetch` | `main` | Parallel HTTP downloader with JSON progress for the VM app | No system backend dependency expected |
| `ryovm-mon` | `ryoku/apps/ryovm/mon` | `main` | QEMU monitor and guest-agent statistics and controls | No system backend dependency expected |
| `ryossh` | `ryoku/apps/ryovm/remote` | `main` | SSH and remote-machine data plane used by the VM app | No system backend dependency expected |
| `ryoku-cli` | `ryoku/cli` | `main`; `cmd/ryoku-manifest`; `internal/doctor`; `internal/importer`; `internal/keyboard`; `internal/keyring`; `internal/ryokumanifest`; `internal/ryotunesrelease`; `internal/securitykey`; `internal/sys`; `internal/updater` | User-facing lifecycle CLI, release manifest generator, and shared lifecycle packages | Primary runtime integration point for `SystemBackend` |
| `ryoku-hub` | `ryoku/hub/backend` | `main` | Settings data plane for compositor, display, GPU, CPU, lighting, desktop, and config operations | Driver and compositor package availability paths need backend injection |
| `ryoku-i18n` | `ryoku/i18n` | `i18n`; `catalog` | Shared translation runtime and embedded installer catalog | Stays unchanged |
| `ryoku-palette-bridge` | `ryoku/palette-bridge` | `main` | Watches and serves the desktop palette over HTTP | Stays unchanged |
| `ryoku-rashin` | `ryoku/rashin/backend` | `main` | Agent daemon, vault/index generation, system-query tools, chat, and dashboard | Package inventory/query tools need backend facts; agent and UI logic stay unchanged |
| `ryoku-shell` | `ryoku/shell/ipc` | `main` | Desktop control plane, Quickshell supervision, Unix socket topics, shell services, and update-status polling | Usually remains an indirect client of `ryoku`; should not acquire native package logic |
| `ryogami` | `ryoku/shell/ryogami/daemon` | `main` | Wallpaper daemon, RPC, effects, live wallpaper, scanning, and upscaling | Stays unchanged |
| `ryoku-wm` | `ryoku/wm` | `wm`; `hyprland` command package; `niri` command package | Neutral compositor client/capability library plus the two provider binaries | The neutral library's reclaim planner and native package metadata need PackageManager input |

### 1.1 Local module dependency edges

The current local replacements show the existing sharing boundaries:

- `installation/tui` imports `ryoku-i18n` and `ryoku-wm`.
- `ryoku-shell-installer` imports `ryoku-i18n` and `ryoku-wm`.
- `ryoku/cli` imports `ryoku-i18n` and `ryoku-wm`.
- `ryoku/hub/backend` imports `ryoku-wm` and declares a local `ryostore`
  replacement.
- `ryoku/rashin/backend`, `ryoku/shell/ipc`, and
  `ryoku/shell/ryogami/daemon` import `ryoku-wm`.

This graph makes the CLI the best first composition root for `SystemBackend`.
Putting the backend inside `ryoku-wm` would invert ownership: compositor policy
would then own the system lifecycle abstraction. The reusable compositor
library should instead accept the small package-planning capability it needs.

### 1.2 `ryoku-cli` package ownership

| Package | Responsibility | Lifecycle coupling |
|---|---|---|
| `main` | Command dispatch; `wm`, recovery, tracking, and plugin command entry points | `wm.go` currently performs package install/removal and availability checks |
| `cmd/ryoku-manifest` | Builds the release control manifest from package lists, PKGBUILDs, and compositor caps | Build-time Arch metadata reader; later consumes a backend-neutral component catalog |
| `internal/updater` | Update, rollback, channel, status, snapshots, materialize, boot guard, run state, and release metadata | Highest concentration of runtime package/repository coupling |
| `internal/doctor` | Ordered, idempotent convergence and reporting | Mixes portable policy with package, repository, boot, and hardware implementations |
| `internal/sys` | Command execution, paths, release state, repository config, user edits, and terminal helpers | Current low-level Arch seam, but too broad to become the public backend contract as-is |
| `internal/ryokumanifest` | Pure release manifest data and generation | Mostly portable; native names must be replaced by component IDs at its public edge |
| `internal/ryotunesrelease` | Finds, verifies, compares, stages, and installs external Ryotunes package releases | Artifact inspection/install and version comparison belong behind PackageManager |
| `internal/importer` | Imports supported user configuration | No lifecycle backend dependency expected |
| `internal/keyboard` | Keyboard command and XKB rules | No package backend dependency expected |
| `internal/keyring` | Desktop secret-service/PAM keyring setup | Service and PAM policy, not repository signing; normally stays outside SystemBackend |
| `internal/securitykey` | Security-key enrollment and PAM/auth configuration | Stays outside the package backend unless component availability is added later |

## 2. Main binaries and responsibilities

| Binary | Source entry point | Responsibility and protocol |
|---|---|---|
| `ryoku-tui` | `installation/tui/main.go` | Full-screen live installer. `system.go` probes the machine, emits `RYOKU_*`, starts `ryoku-install`, and parses `@@RYOKU_STEP` and `@@RYOKU_DONE` sentinels. |
| `ryoku-install` | `installation/backend/ryoku-install` | Shell binary, not Go. Root installer orchestrator used by `ryoku-tui`. Included here because it is the TUI's executable backend boundary. |
| `ryoku-shell-install` | `ryoku-shell-installer/main.go` | Standalone existing-system installer. Interactive and headless modes drive the same resumable engine. |
| `ryoku` | `ryoku/cli/main.go` | User lifecycle front door: update, rollback, status, materialize, doctor, verify, boot guard, compositor switching, recovery, and related commands. |
| `ryoku-manifest` | `ryoku/cli/cmd/ryoku-manifest/main.go` | Publish-time generator for the release control `manifest.json`. |
| `ryoku-shell` | `ryoku/shell/ipc/main.go` | Daemon/client pair for the desktop. Supervises Quickshell, owns the shell socket, and publishes state topics. |
| `ryoku-hub` | `ryoku/hub/backend/main.go` | Subcommand JSON/text backend for the Settings QML application. |
| `ryoku-wm-hyprland` | `ryoku/wm/hyprland/main.go` | Hyprland provider for neutral caps, actions, state, watch, apply, outputs, session, schema, binds, environment, and plugins. |
| `ryoku-wm-niri` | `ryoku/wm/niri/main.go` | niri provider implementing the same neutral protocol without the plugin subsystem. |
| `ryogami` | `ryoku/shell/ryogami/daemon/main.go` | Wallpaper daemon/client with line and JSON RPC protocols. |
| `ryostore` | `ryoku/apps/ryostore/backend/main.go` | JSON data plane for catalogs and install-only product transactions. |
| `ryoku-rashin` / `rashin` | `ryoku/rashin/backend/main.go` | Agent service, dashboard, indexer, system tools, and terminal assistant. Invocation name selects daemon CLI or terminal-ask behavior. |
| `ryoku-palette-bridge` | `ryoku/palette-bridge/main.go` | Palette file watcher and HTTP publisher. |
| `ryovm-fetch` | `ryoku/apps/ryovm/fetch/ryovm-fetch.go` | Cancellable VM-image downloader emitting JSON progress. |
| `ryovm-mon` | `ryoku/apps/ryovm/mon/ryovm-mon.go` | QEMU HMP/QGA monitor and live-control helper emitting JSON. |
| `ryossh` | `ryoku/apps/ryovm/remote/ryossh.go` | SSH inventory, probing, connections, keys, Proxmox operations, and tunnel helper. |

The install and packaging definitions build these commands in
`release/packages/*/PKGBUILD`, `installation/iso/build.sh`, and
`ryoku/shell/deploy.sh`. That build wiring is Arch-specific, but the command
protocols listed above are not.

## 3. Updater location and control flow

The updater is owned by `ryoku/cli/internal/updater`. `ryoku/cli/main.go`
dispatches `update`, `rollback`, `snapshots`, `status`, `materialize`, and
`boot-guard` into it.

### 3.1 File map

| File | Responsibility | Backend boundary |
|---|---|---|
| `update.go` | Top-level update stages, lock and disk guards, snapshots, package lanes, stage1 to stage2 handoff, shell cutover, doctor, rollback, snapshots, status, and dev deploy | Package transactions, pending updates, ownership, lock state, snapshot/boot integration |
| `ryokuset.go` | Defines the exact Ryoku package lane, repository-qualified target selection, hold-back comparison, refresh argv, and install argv | PackageManager planning/query/version comparison and RepositoryManager identity |
| `manifest.go` | Fetches release manifest, records applied baseline, plans convergence, verifies drift, and reads installed/explicit package sets | Package inventory must come from PackageManager; policy remains here |
| `channel.go` | Checkout-based update path and source-channel synchronization | Git channel stays here; packaged channel moves use RepositoryManager through `release.go`/`sys` |
| `release.go` | Channel release metadata, ledger, packaged channel switching, and rollback channel selection | RepositoryManager owns channel inspection/ensure; release policy remains here |
| `bootguard.go` | Counts failed boots and restores the previous release or snapshot route | Package rollback and BootManager calls; guard policy remains here |
| `materialize.go` | Copies packaged config, preserves seeds and overlays, applies active provider config | Portable application service; no PackageManager dependency |
| `runstate.go` | Publishes update progress and prompt state for QML consumers | Portable protocol, unchanged |
| `upgradelog.go` | Curates transaction output for terminals | Portable renderer if backend events are normalized |
| `ryotunes.go` | Hooks the independently released Ryotunes update into both update paths | Delegates artifact work to `internal/ryotunesrelease` |
| `art.go`, `commits.go`, `version.go`, `reset.go` | Presentation, commit metadata, version output, and overlay reset | No native package execution except version facts supplied by `sys` today |

### 3.2 Current packaged update sequence

`Update` in `update.go` currently performs this policy sequence:

1. Acquire the update lock, check free space, prime sudo, and take a Snapper
   pre-snapshot.
2. If a source checkout is active, run the Git channel deploy path.
3. Otherwise refresh pacman databases, calculate the installed `[ryoku]` set,
   compare installed and repository versions with `vercmp`, then install the
   exact repo-qualified set.
4. Optionally run the base-system, AUR, and Flatpak lanes for `--system`.
5. `exec` the newly installed `/usr/bin/ryoku` with `--stage2`.
6. In stage2, arm the boot guard, quiesce the shell and power owners,
   materialize configuration, restart the desktop, refresh auxiliary indexes,
   run the freshly installed doctor, and close the snapshot pair.

This ordering belongs to the updater and should not move into SystemBackend.
Package refresh, planning, revalidation, application, artifact installation,
repository channel mutation, snapshot/boot operations, and their native errors
are the replaceable operations.

### 3.3 Status path used by the desktop

`ryoku status --json` is the public machine-readable status contract.
`ryoku/shell/ipc/updates.go` polls it and publishes the result on the
`updates` socket topic. Both
`ryoku/shell/quickshell/shell/services/Updates.qml` and
`ryoku/hub/quickshell/Singletons/Updates.qml` subscribe to that topic.

Therefore the Go updater should consume SystemBackend, while the shell daemon
and QML should continue consuming `ryoku status` and the existing socket frame.
Backend-specific words in JSON fields or UI copy can be normalized later
without moving package logic into QML.

## 4. Doctor location and ownership

Doctor is owned by `ryoku/cli/internal/doctor` and entered through
`doctor.Run` from `ryoku/cli/main.go`.

### 4.1 Coordinator

- `doctor.go` defines `recResult`, the `reconciler` type, the ordered
  `reconcilers()` registry, sequential execution, terminal/JSON output, and a
  number of older reconcilers that still live in the coordinator file.
- `Run` selects mutating or read-only behavior. `--check`, `--report`,
  `--explain`, and `--json` are read-only.
- `report.go` builds the diagnostic bundle; `explain.go` provides optional AI
  explanation; `provision.go` tracks deliver-once provisioned state.
- Each `reconcile_*.go` file owns one narrow convergence concern. The registry,
  rather than filenames or a migration ledger, fixes execution order.

The reconciler contract and ordering remain application policy. Backend calls
replace native probes and mutations inside reconcilers.

### 4.2 Backend-sensitive doctor groups

| Concern | Principal files | Future dependency |
|---|---|---|
| Package and repository health | Package/repo sections in `doctor.go`; `reconcile_pacman.go`; `reconcile_multilib.go`; `reconcile_manifest.go`; `reconcile_shipped_apps.go`; `reconcile_quickshell.go`; `reconcile_ryotunes.go`; `reconcile_asus_aura.go`; `report.go` | PackageManager and RepositoryManager |
| Packaged-install detection and delivery | `reconcile_bootguard.go`; `reconcile_config_tree.go`; `reconcile_rashin_daemon.go`; `reconcile_ryogami_wallpaper.go`; `reconcile_wm_plugins.go` | Backend identity, capabilities, and component inventory. Some files only need backend-neutral remedies or installed-component facts. |
| Boot, initramfs, and snapshots | `reconcile_limine.go`; `reconcile_limine_entries.go`; `reconcile_limine_images.go`; `reconcile_alongside.go`; `reconcile_boot_rw.go`; `reconcile_boot_space.go`; `reconcile_initramfs.go`; `reconcile_snapshots.go` | BootManager, with policy and exact safety gates retained in doctor |
| GPU and hardware | `reconcile_hardware.go`; `reconcile_dgpu_panel.go`; `reconcile_gpu_pin.go`; `reconcile_gpu_pin_panel.go`; `reconcile_ppd_amdgpu.go`; `reconcile_asus_aura.go`; `reconcile_fingerprint.go`; `reconcile_wifi_regdom.go` | DriverManager and backend capabilities only where package, initramfs, service, or native-path semantics differ |
| Desktop/session/config | The remaining `reconcile_*.go` files, including shell config, user edits, MIME, portal, audio, lockscreen, browser, and compositor config | Usually unchanged. Use backend capabilities only when a service name, native path, or package-derived remedy actually differs. |

`doctor.go` currently also knows `/var/lib/pacman/db.lck`, `pacman-key`, the
`[ryoku]` sync database, orphan queries, package ownership, and `.pacnew`.
Those are not generic doctor concepts. The Arch backend should report normalized
lock, repository, orphan, ownership, and config-conflict facts so the same
reconciler policy can render an Arch or Debian result.

## 5. Package-manager command call sites

The current calls are not centralized. The list below identifies active
implementation owners, not comments, translations, tests, or decorative uses
of the word "pacman."

### 5.1 Go runtime callers

| Owner | Files | Current native operations |
|---|---|---|
| CLI updater | `ryoku/cli/internal/updater/update.go`, `ryokuset.go`, `manifest.go`, `bootguard.go` | `pacman -S/-Sy/-Syy/-Syu/-Sl/-Si/-Qi/-Qq/-Qqe/-Qo/-Qu`, `yay -Sua/-Qua`, `vercmp`, pacman process/lock awareness |
| CLI low-level facts | `ryoku/cli/internal/sys/sys.go`, `repo.go`, `release.go` | Installed-package/version facts and direct `/etc/pacman.conf` plus sync-database handling |
| CLI doctor | `ryoku/cli/internal/doctor/doctor.go`, `reconcile_asus_aura.go`, `reconcile_hardware.go`, `reconcile_manifest.go`, `reconcile_multilib.go`, `reconcile_quickshell.go`, `reconcile_ryotunes.go`, `reconcile_shipped_apps.go`, `report.go` | Package install/query/owner/orphan/database/keyring operations and repository edits |
| CLI compositor switch | `ryoku/cli/wm.go` | Availability, installed state, package install, guarded removal, and final transaction |
| External Ryotunes channel | `ryoku/cli/internal/ryotunesrelease/client.go` | Installed/artifact metadata, `pacman -U`, and `vercmp` |
| Neutral WM library | `ryoku/wm/reclaim.go` | Installed facts, simulated `pacman -Rs --print`, installed sizes, and removal-plan revalidation |
| Hyprland provider | `ryoku/wm/hyprland/config_plugins_tier.go` | Installed package version for plugin source/ABI reporting |
| Hub WM backend | `ryoku/hub/backend/wm.go` | Repository availability through `pacman -Si`; reclaim delegates to `ryoku-wm` |
| Hub GPU backend | `ryoku/hub/backend/gpuapply.go`, with package-derived hints in `hwcaps.go` | Core passthrough install, installed checks, AUR wrapper or `yay`/`paru` fallback |
| Standalone existing-system installer | `ryoku-shell-installer/distro.go`, `detect.go`, `engine.go`, `lifecycle.go` | Both pacman and apt command vectors exist, but the Ryoku package/repository path, AUR bootstrap, and uninstall remain Arch-specific |
| Rashin live facts | `ryoku/rashin/backend/index.go`, `quicktools.go` | Pacman inventory, explicit set, version, count, and package details |

`ryoku/rashin/backend/danger.go` and `habits.go` classify package-manager command
names for agent safety/history. They do not perform lifecycle transactions, but
their vocabulary must recognize the selected backend so Debian package commands
receive the same safety tier.

### 5.2 Shell runtime callers

| Owner | Files | Current native operations |
|---|---|---|
| System package actuators | `system/extras/ryoku-pkg-add`, `ryoku-pkg-remove`, `ryoku-pkg-aur-add`, `ryoku-pkg-multilib`, `ryoku-pkg-cachyos`, `ryostore-install` | Official install/removal, AUR install, repository enablement, and bundle package inventory |
| Hardware drivers | `system/hardware/drivers/amd.sh`, `intel.sh`, `nvidia.sh`, `vulkan.sh`, `ryoku-nvidia-guard`; `system/hardware/gpu/ryoku-gpu-lib32` | Vendor package selection/install, multilib, kernel headers/modules, guard queries, and initramfs hook installation |
| Development deploy | `ryoku/shell/deploy.sh` | Local build deployment plus optional `[ryoku]` key/repo setup and external package install |
| Desktop leaf tools | `ryoku/shell/scripts/stash-install.sh`, `ryoku-profile-stats`, `ryoku-sysinfo` | Local artifact `pacman -U` and package counts |
| Power transaction guard | `system/hardware/power/ryoku-power-cutover` | Watches the pacman database lock around package hooks |

The system actuators can become Arch adapter entry points temporarily, but
shared Go application services should not continue discovering them by name.
Their behaviors should be expressed by PackageManager/RepositoryManager and
DriverManager capabilities.

### 5.3 Installer callers

| Layer | Files | Current native operations |
|---|---|---|
| Root installer orchestrator | `installation/backend/ryoku-install` | Orders all install stages |
| Base system | `installation/backend/lib/pacstrap.sh` | Package-list resolution, keyring preflight, `pacstrap`, cache cleanup, and retries |
| Ryoku repository/desktop | `installation/backend/lib/deploy.sh`, `offline.sh`, `cachyos.sh` | Writes pacman configs/mirrorlists, imports keyrings, syncs databases, installs the desktop, and manages offline/CachyOS repositories |
| Target execution | `installation/backend/lib/chroot.sh`, `bootloader.sh`, `drivers.sh` | `arch-chroot`, package queries, initramfs hooks, vendor drivers, and boot package facts |
| AUR | `installation/backend/lib/aur.sh` | Bootstraps `yay` with `makepkg`, then builds critical and best-effort AUR sets |
| Existing-system installer | `ryoku-shell-installer/engine.go`, `distro.go`, `detect.go`, `lifecycle.go` | Host update/install/remove/query, repository trust, desktop packages, driver scripts, AUR, and uninstall |

### 5.4 Build and release callers

| Layer | Files | Current native operations |
|---|---|---|
| Signed `[ryoku]` repository | `release/repo/build-repo.sh` | Walks PKGBUILDs, runs `makepkg --sign`, re-signs adopted artifacts, and runs signed `repo-add` |
| External Ryotunes import | `release/repo/import-ryotunes.sh`, `refresh-ryotunes.sh` | Verifies upstream artifact identity/checksum, signs packages, updates and verifies the signed repo database |
| Offline ISO repository | `installation/iso/offline-repo.sh` | Resolves/downloads closure with isolated pacman state, builds selected AUR packages, validates package ownership, and runs `repo-add` |
| ISO image | `installation/iso/build.sh` | Stages Arch pacman configuration and invokes the archiso build after prebuilding installer payloads |
| Local install test repo | `installation/tests/build-ryoku-repo.sh` | Generates an ephemeral signing key and drives the normal repository builder |

The future build-time repository abstraction belongs beside `release/repo`, not
inside the runtime PackageManager. Arch keeps PKGBUILD/makepkg/repo-add; Debian
gets its own package and APT repository publisher while release promotion and
manifest policy stay shared.

### 5.5 QML package command leaks

QML generally calls Ryoku services, but four boundaries still expose native
package concepts:

- `ryoku/shell/quickshell/shell/modules/launcher/shared/providers/packages/Packages.qml`
  calls `gpk` and explicitly filters `pacman,aur`.
- `ryoku/hub/quickshell/pages/DictationPage.qml` invokes `gpk` with an AUR
  manager to install/remove `voxtype-bin`.
- `ryoku/hub/quickshell/pages/ProfilePage.qml` falls back to the first entry in
  `/var/log/pacman.log` for an installation date.
- `ryoku/shell/quickshell/shell/modules/bar/barstyles/qsbar/modules/ParticleStream.qml`
  tails `/var/log/pacman.log` to visualize completed package transactions.

These QML files should not import or implement SystemBackend. Their data source
should become neutral: a backend-owned package search/install service, a machine
installation fact, and a normalized transaction event stream respectively.
Decorative Pac-Man workspace styles are unrelated and stay unchanged.

## 6. Installer orchestration

Ryoku currently has two installer products with different targets.

### 6.1 Live ISO installer

The control boundary is already process-based:

1. `installation/tui/main.go` owns the Bubble Tea experience and choice model.
2. `installation/tui/system.go` owns live probes, disk/network/hardware choices,
   maps them to the `RYOKU_*` environment, starts `ryoku-install`, and streams
   progress sentinels.
3. `installation/backend/ryoku-install` validates the environment and calls
   library stages in this order: preflight, offline verification, DNS/mirrors,
   keyring, partition/carve, LUKS, filesystems, mount, pacstrap, configure,
   repository/deploy, network, drivers, bootloader, AUR, snapshots, cleanup.
4. `installation/backend/lib/common.sh` supplies logging, command, write, and
   progress helpers. Each other file owns one install concern.
5. `installation/iso/build.sh`, `profiledef.sh`, `packages.x86_64`, `pacman.conf`,
   `offline-repo.sh`, and `airootfs/` define the Arch live image.

For migration, keep the TUI model and progress protocol. Replace the single
`ryoku-install` selection with an InstallerBackend selected from the live
environment. The current shell backend becomes the Arch implementation. Disk
policy and destructive confirmations stay common; `pacstrap`, `arch-chroot`,
pacman configuration, keyrings, Arch mirrors, mkinitcpio, Limine integration,
and AUR are Arch implementation details.

### 6.2 Existing-system installer

`ryoku-shell-installer` contains a second orchestration engine:

- `detect.go` gathers OS, desktop, boot, package, GPU, and migration facts.
- `distro.go` contains command/name mappings for Arch and Debian-family hosts.
- `engine.go` owns the ordered resumable plan: legacy cleanup, system update,
  tools, payload, backup, repository, conflicts, packages, drivers or source
  build, session, config, AUR, shell, doctor, and verify.
- `de.go`, `hypr.go`, `niri.go`, and `sway.go` import existing desktop state.
- `lifecycle.go` owns resume state and uninstall.

The presence of apt command vectors in `distro.go` does not make the installer
backend-neutral. `engine.go` still assumes the signed `[ryoku]` pacman repo,
Arch package names, AUR, pacman conflict behavior, and Arch driver scripts.
Keep its user journey and resumable engine, but make repository, package,
driver, boot/session prerequisites, and native-name resolution explicit backend
dependencies.

## 7. Hardware and GPU implementation map

Hardware code has three layers that should remain distinct.

### 7.1 Portable detection and runtime policy

- `system/hardware/gpu/ryoku-gpu-detect` reads DRM/sysfs facts and provides the
  shared GPU detector.
- `system/hardware/gpu/ryoku-gpu` applies host render modes.
- `system/hardware/gpu/ryoku-gpu-mux` handles supported mux switching.
- `ryoku/hub/backend/gpu.go`, `gpumode.go`, `gpumux.go`, `gputune.go`,
  `gpupreset.go`, `gpuhook.go`, `gpudomain.go`, and `gpuvm.go` expose runtime GPU
  policy to Settings.
- `ryoku/hub/backend/hwcaps.go` combines hardware and tooling facts into the
  passthrough capability dossier.
- GPU-related doctor reconcilers observe backlight, loaded drivers, render
  pins, panel ownership, power behavior, and NVIDIA health.

Sysfs/DRM detection, mode policy, JSON contracts, and QML presentation can stay
shared when paths and capabilities are genuinely common.

### 7.2 Distribution-specific driver fulfillment

- `system/hardware/drivers/amd.sh`, `intel.sh`, `nvidia.sh`, and `vulkan.sh`
  resolve detected hardware to Arch native package names and install them.
- `system/hardware/drivers/ryoku-nvidia-guard` couples installed kernel/module
  packages to pacman hooks and mkinitcpio recovery.
- `system/hardware/gpu/ryoku-gpu-lib32` enables multilib and installs Arch
  `lib32-*` userspace selected from loaded drivers.
- `installation/backend/lib/drivers.sh` copies those scripts into the target,
  invokes them with `arch-chroot`, then applies the TUI GPU-mode choice.
- `ryoku/hub/backend/gpuapply.go` installs Arch virtualization and Looking Glass
  package sets directly, including AUR fallback.

These are the Arch DriverManager and installer-driver implementation. Debian
must provide native package resolution, multiarch handling, firmware policy,
initramfs hooks, NVIDIA branch policy, and services without changing the Hub GPU
page or shared detection model.

### 7.3 Firmware boundary

Firmware is currently delivered through Arch package lists and first-party
PKGBUILDs rather than a Go hardware service. Relevant owners are
`system/packages/hardware.packages`, the vendor driver scripts, and firmware
packages under `release/packages/`, notably `broadcom-bt-firmware`.
DriverManager should report missing/available firmware components through
logical IDs; repository packaging remains distribution-specific.

## 8. QML and Quickshell boundaries

### 8.1 Entry points

The principal Quickshell roots are:

- Desktop: `ryoku/shell/quickshell/shell/shell.qml`.
- Settings: `ryoku/hub/quickshell/shell.qml`.
- Ryostore: `ryoku/apps/ryostore/quickshell/shell.qml`.
- VM app: `ryoku/apps/ryovm/quickshell/shell.qml`.
- Wallpaper UI: `ryoku/shell/ryogami/wall-ui/shell.qml`.
- Auxiliary surfaces: `keys/shell.qml`, `keys-hint/shell.qml`,
  `reload-cover/shell.qml`, `ryopin/shell.qml`, `ryoshot/shell.qml`, and
  `welcome/shell.qml` under `ryoku/shell/quickshell`.

All shared visuals continue to use `ryoku/ui` and the current QML component
trees. No SystemBackend interface belongs in the QML import graph.

### 8.2 Stable process and socket boundaries

| QML consumer | Boundary | Go owner |
|---|---|---|
| Desktop service singletons | `$XDG_RUNTIME_DIR/ryoku-shell.sock` topics and calls | `ryoku/shell/ipc` |
| Settings pages | `ryoku-hub` subcommands plus the shell socket for shared live topics | `ryoku/hub/backend`, `ryoku/shell/ipc` |
| Compositor switch UI | `ryoku-hub wm preview`, then `ryoku wm use` | Hub preview plus CLI transaction owner |
| Updates page and bar | `ryoku-shell` `updates` topic backed by `ryoku status --json`; update launched as `ryoku update` | CLI updater through shell daemon |
| System Check UI | `ryoku doctor --json` or `ryoku doctor` | CLI doctor |
| Ryostore UI | `ryostore` subcommands; privileged bundles cross through `ryostore-install` today | Ryostore backend plus system actuator |
| Wallpaper surfaces | `$XDG_RUNTIME_DIR/ryogami.sock` and `ryogami` client verbs | Ryogami daemon |
| VM UI | `ryovm`, `ryovm-mon`, `ryovm-fetch`, and `ryossh` command protocols | App helpers |
| Agent surfaces | `ryoku-rashin` command/HTTP protocols | Rashin backend |

These boundaries are useful migration shields. Backend selection and native
tool execution happen behind them. QML changes should be limited to removing
native terminology or consuming normalized fields, not restructuring views.

### 8.3 Boundaries that stay unchanged

- `ryoku/ui`, desktop QML components, animation, styling, and layout.
- The single shell scene and `ryoku-shell.sock` protocol.
- The `ryoku-wm-*` compositor process protocol and provider-owned native config.
- Ryogami wallpaper RPC and UI.
- Palette bridge behavior.
- Ryostore catalog/receipt logic, apart from the package actuator it calls.
- VM and SSH helpers.
- i18n catalogs and runtimes.

## 9. Files that should eventually depend on `SystemBackend`

Dependency here means constructor/field/interface injection into the owning Go
service. It does not mean importing a concrete Arch or Debian implementation.

### 9.1 Direct consumers

| Priority | Existing owner/files | Required backend facets | Ownership rule |
|---|---|---|---|
| 1 | `ryoku/cli/internal/updater/update.go`, `ryokuset.go`, `manifest.go`, `release.go`, `bootguard.go`, and `ryotunes.go` | Identity, Capabilities, PackageManager, RepositoryManager, BootManager | Updater keeps sequencing and user policy |
| 1 | `ryoku/cli/internal/doctor/doctor.go` and the backend-sensitive reconciler files listed in section 4.2 | All facets, but injected per reconciler or helper rather than used as a global | Doctor keeps registry, result model, and convergence policy |
| 1 | `ryoku/cli/internal/sys/repo.go`, package/version portions of `sys.go`, and relevant release facts | Arch adapter implementation or thin bridge during extraction | Generic filesystem/exec/terminal helpers stay in `sys`; native lifecycle facts move out |
| 2 | `ryoku/cli/wm.go` and `ryoku/wm/reclaim.go` | PackageManager safe removal, inventory, size, plan, and revalidation | WM capability model stays neutral; native package solver leaves `ryoku-wm` |
| 2 | `ryoku/hub/backend/wm.go` | Package availability/component resolution | Hub keeps presentation data assembly |
| 2 | `ryoku/hub/backend/gpuapply.go` and package/tooling gathering in `hwcaps.go` | DriverManager, PackageManager, component catalog | GPU policy and capability verdict stay in Hub/shared hardware code |
| 2 | `ryoku/cli/internal/ryotunesrelease/client.go` | Artifact inspection/install and native version comparison | Download origin and checksum policy stay in the Ryotunes client |
| 3 | `ryoku/rashin/backend/index.go` and `quicktools.go` | Read-only package inventory, installed version, and pending updates | Rashin keeps vault and tool schemas |
| 3 | `ryoku/wm/hyprland/config_plugins_tier.go` | Read-only installed component version | Provider keeps plugin ABI/source policy |
| 4 | `ryoku-shell-installer/engine.go`, `distro.go`, `detect.go`, and `lifecycle.go` | Identity, packages, repositories, drivers, and target execution/boot capabilities | Existing-system installer keeps its plan and resume state |

The `SystemBackend` composition root should be explicit. For the CLI, selection
happens once near `ryoku/cli/main.go`, then the selected facets are passed to
updater, doctor, and WM commands. Hub and Rashin are separate binaries and must
perform the same identity-based selection in their own entry points, or call a
stable `ryoku` machine API where that avoids duplicated composition.

### 9.2 Indirect consumers that should not import it

- `ryoku/shell/ipc/updates.go` should keep invoking the stable `ryoku status
  --json` contract. It receives backend-neutral fields indirectly.
- QML files should continue calling `ryoku`, `ryoku-hub`, `ryoku-shell`, and
  other data-plane commands or sockets.
- `installation/tui` should depend on an installer client/contract, not package
  or repository facets directly.
- `ryoku/apps/ryostore/backend` should keep product and receipt policy. Its
  privileged package actuator should become a neutral service boundary rather
  than giving the catalog engine broad SystemBackend access.
- Runtime GPU helpers that only inspect sysfs or apply compositor/session policy
  should not depend on PackageManager.
- `ryoku-i18n`, `ryoku-palette-bridge`, `ryogami`, `ryovm-fetch`, `ryovm-mon`,
  and `ryossh` need no SystemBackend dependency.

### 9.3 Non-Go implementations

The shell installer and release pipeline cannot directly import a Go interface.
They need equivalent process-level boundaries:

- `installation/backend/ryoku-install` becomes the Arch InstallerBackend, or a
  thin entry point to it.
- `installation/backend/lib/*` stays under Arch ownership until portable disk
  policy is extracted from native target operations.
- `system/extras/ryoku-pkg-*` and vendor driver scripts become Arch adapter
  executables or are retired after all callers use backend services.
- `release/repo/build-repo.sh` implements the Arch build-time repository backend.
- Debian installer and repository tools live beside, not inside, those Arch
  implementations.

## 10. Suggested migration order by code ownership

This order follows existing owners and minimizes cross-team churn. Each step
keeps the public command and QML contracts working.

### Step 1: shared backend contracts and component catalog

Owner: system lifecycle architecture.

- Add backend identity, capabilities, logical component IDs, PackageManager,
  RepositoryManager, BootManager, and DriverManager contracts in a reusable Go
  module.
- Define normalized plans, transaction events, errors, package state, repository
  state, and dry-run behavior from `backend-design.md`.
- Do not move updater or doctor policy into this module.

### Step 2: Arch read-only package and repository adapter

Owner: CLI/system backend.

- Extract installed, explicit, available, owner, size, pending, repository, and
  version comparison queries from `internal/sys`, `ryokuset.go`, `manifest.go`,
  doctor, WM reclaim, Hub WM, and Rashin.
- Preserve command output and JSON schemas.
- Use characterization tests around the current pacman parsing before replacing
  call sites.

### Step 3: updater transactions and repository channels

Owner: `ryoku/cli/internal/updater`.

- Route refresh, plan, revalidate, apply, artifact install, and channel mutation
  through the Arch backend.
- Keep snapshots, stage ordering, stage2 handoff, materialize, shell cutover,
  run-state publication, and doctor invocation in updater.
- Move Ryotunes artifact metadata/install behind the same package facet without
  changing its download and verification policy.

### Step 4: doctor lifecycle reconcilers

Owner: `ryoku/cli/internal/doctor`.

- Convert package/repository reconcilers first, then boot, then hardware.
- Leave desktop/config reconcilers alone unless they have a documented native
  path, service, or remedy dependency.
- Retain idempotency, check-only behavior, registry order, and JSON output.

### Step 5: compositor and Hub package operations

Owner: `ryoku/wm`, CLI WM command, and Hub backend.

- Replace pacman simulation and installed-size parsing in `reclaim.go` with the
  PackageManager removal plan/revalidation contract.
- Resolve compositor logical components to backend-native packages outside caps.
- Route Hub package availability and GPU stack installation through selected
  facets. Keep provider protocol and QML unchanged.

### Step 6: hardware driver fulfillment

Owner: `system/hardware` and Hub GPU backend.

- Separate shared detection/policy from Arch package, multilib, firmware,
  initramfs, and service actions.
- Wrap the existing Arch scripts behind DriverManager before adding Debian
  mappings.
- Normalize actionable remedies so QML and doctor stop printing pacman/yay
  commands directly.

### Step 7: installer backend split

Owner: installation.

- Freeze the `RYOKU_*` input and progress-event behavior with contract tests.
- Name the current `installation/backend` implementation as Arch and introduce
  a backend selector at the TUI handoff.
- Separate target runner, base bootstrap, repository trust, desktop install,
  driver install, boot install, and offline source operations.
- Apply the same selected backend facets to `ryoku-shell-installer` while
  retaining its independent existing-system workflow.

### Step 8: release and image backends

Owner: release engineering.

- Keep shared release identity, manifest, promotion, and channel rules.
- Treat `release/packages` plus `release/repo/build-repo.sh` as the Arch package
  and repository implementation.
- Add Debian packaging/repository publication and a Debian live-image backend
  separately. Do not conditionally mix PKGBUILD and Debian package rules in one
  file.

### Step 9: QML terminology cleanup

Owner: desktop UI, after normalized backend data exists.

- Remove explicit `pacman,aur` filters, pacman-log reads, AUR-only dictation
  actions, and Arch-specific update copy by consuming service-provided facts.
- Preserve pages, layouts, animations, shell processes, and visual behavior.

## Implementation boundary summary

The smallest safe dependency graph is:

```text
QML / Quickshell
    -> existing command and socket protocols
        -> updater, doctor, Hub, WM, installer application policy
            -> SystemBackend interfaces
                -> Arch or Debian runtime implementation

release policy
    -> build-time repository interface
        -> Arch PKGBUILD/makepkg/repo-add or Debian package/APT publisher
```

The principal rule for implementation is that a file should depend on
`SystemBackend` only when it needs a distribution fact or action. Files that
own user experience, ordering, convergence decisions, compositor semantics, or
desktop behavior keep that ownership and receive normalized results from the
backend.

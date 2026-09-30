# Arch package-manager usage map

## Status and scope

This document inventories the current executable `pacman` call sites that the
first Arch `SystemBackend` adapter must absorb or deliberately leave in an
Arch-only installer, release, or validation tool. It is a call-site map, not an
implementation plan. No backend exists yet.

The inventory covers first-party production code, installer code, installed
helpers, developer/recovery tools, ISO tooling, and integration gates. It does
not count prose, comments, translations, dry-run messages, or commands shown
only as suggested remedies. `pacman-key`, `pacman-conf`, `pacstrap`, `makepkg`,
and `repo-add` are related Arch dependencies but are not invocations of the
`pacman` executable, so they remain in the repository and installer maps.

Generated `vendor/ryoku-wm/reclaim.go` copies repeat the three calls in the
canonical `ryoku/wm/reclaim.go`. They are recorded once in the canonical table
and listed under generated copies rather than treated as independent owners.

The suggested method names use the contract in
`docs/migration/backend-design.md`:

- `Inventory`, `Query`, `OwnerOf`, `Available`, `PendingUpdates`, and
  `CompareVersions` are read-only package facts.
- `RefreshMetadata`, `Plan`, `Revalidate`, and `Apply` own native transactions.
- `InspectArtifact` and `PlanArtifactInstall` own local package files.
- `LockState`, `RepairStaleLock`, and `CheckHealth` are named here where the
  current contract describes a responsibility but does not yet give it a
  method signature.
- `RepositoryManager` is named where the operation is repository state rather
  than package database semantics.
- `InstallerTarget` and `ReleaseBackend` identify operations that should not be
  pulled into the installed runtime `SystemBackend`.

## Query pattern index

| Query shape | Current meaning | Occurrence groups | Suggested interface fact |
|---|---|---|---|
| `pacman -Q <name>` / `-Qq <name>` | Installed check, including provider resolution in some shell helpers | `internal/sys`, standalone installer, WM, Hub, drivers, Ryostore, installer target, recovery | `Query(names)` with `Installed`; use resolved provider records when the caller needs provider ownership |
| `pacman -Qq` | All installed package names | updater, manifest, installer detection, Ryostore, drivers, Rashin, sysinfo | `Inventory().Installed` |
| `pacman -Qqe` | Explicitly installed names | manifest verification and Rashin | `Inventory().ManuallyInstalled` |
| `pacman -Qen` | Explicit native-repository packages | profile statistics | `Inventory()` filtered by manual state and non-supplemental source |
| `pacman -Qm` / `-Qmq` | Foreign package list | profile statistics and Quickshell provenance | `Inventory()` filtered by supplemental or unknown source |
| `pacman -Qdq <name>` | Installed as a dependency | shipped-app ownership | `Query(names).ManuallyInstalled == false` |
| `pacman -Qtdq` | Orphaned dependencies | doctor and diagnostics | `Inventory().Orphans` or a typed orphan list |
| `pacman -Qi <name...>` | Installed metadata, version, and installed size | updater, WM reclaim, Rashin | `Query(names)` |
| `pacman -Si <name>` | Available package metadata | updater, WM/Hub availability, drivers, Ryostore, build validation | `Available(names)` |
| `pacman -Sl` / `-Slq <repo>` | Repository package names and versions | updater and doctor | `Available` filtered by `PackageSource`, or `RepositoryManager.Packages(source)` |
| `pacman -Qu` | Pending upgrades against already-synced metadata | updater | `PendingUpdates(scope)` |
| `pacman -Qo` / `-Qoq <path>` | Installed owner of a filesystem path | updater, doctor, deploy, recovery tests | `OwnerOf(path)` |
| `pacman -Qip <file>` | Local artifact name, version, and architecture | Ryotunes release client | `InspectArtifact(path)` |
| `pacman -Qlp <files...>` | File list and owners declared by local artifacts | offline repository verification | `InspectArtifact(path).Files`; release-only use |
| `pacman -Ql <name>` | Files owned by an installed package | integration validation | `Query(name).Files` or a typed installed-file-list method |
| `pacman -T <deps...>` | Unsatisfied dependency expressions | standalone installer | `Plan(install)` or a typed `ResolveRequirements` result |
| `pacman -Sp <names...>` | Resolve an install transaction without applying it | offline installer and ISO closure builder | `Plan(install)`; `InstallerTarget.Plan` or `ReleaseBackend.Plan` off-runtime |
| `pacman -Ss <pattern>` | Search repository names/providers | ISO package validation | `Available`/provider search; release-only use |
| `pacman -Dk` | Package database consistency check | doctor report | `CheckHealth()` |

Version parsing currently appears in four forms: parsing field two from
`pacman -Q`, parsing `Version` from `-Qi` and `-Si`, parsing column three from
`-Sl`, and comparing with `vercmp`. The backend should return versions in
`PackageRecord` and own `CompareVersions`; callers should not parse localized
native output.

Related package queries do not invoke `pacman` directly but need the same
backend facts:

| File and function | Command used | Purpose | Suggested interface method |
|---|---|---|---|
| `ryoku/cli/internal/updater/update.go:1698`, `pendingUpdates` | `checkupdates` | List distribution-repository updates using a private synced database | `PendingUpdates(scope=System)` |
| `ryoku/cli/internal/updater/update.go:1728`, `aurUpdates` | `yay -Qua` | List supplemental/AUR updates without installing | `PendingUpdates(scope=Supplemental)` |
| `ryoku/rashin/backend/index.go:454`, `packagesBody` | `checkupdates` | Count pending distribution updates in indexed facts | `PendingUpdates(scope=System)` |
| `ryoku/rashin/backend/quicktools.go:112`, `toolSystemQuery` | `checkupdates` | Agent system-query tool lists pending updates | `PendingUpdates(scope=System)` |
| `ryoku/shell/quickshell/shell/modules/launcher/shared/providers/packages/Packages.qml:217` | `gpk search ... --manager pacman,aur` | Search installable native and supplemental packages for the launcher | Backend-neutral package search/catalog method; no manager names in QML |

## 1. `ryoku/cli` callers

### 1.1 Shared facts and updater

| File and function | Command used | Purpose | Suggested interface method |
|---|---|---|---|
| `ryoku/cli/internal/sys/sys.go:47`, `PkgInstalled` | `pacman -Q <name>` | Generic installed-package predicate used throughout updater and doctor | `Query([name])` |
| `ryoku/cli/internal/sys/sys.go:58`, `InstalledVersion` | `pacman -Q ryoku-desktop` | Read the installed desktop version | `Query([desktop.core])` |
| `ryoku/cli/internal/updater/ryokuset.go:91`, `installedRyokuSet` | `pacman -Slq ryoku` | List names served by the current Ryoku repository | `Available(source=Ryoku)` or `RepositoryManager.Packages` |
| `ryoku/cli/internal/updater/ryokuset.go:95`, `installedRyokuSet` | `pacman -Qq` | Intersect all installed names with the Ryoku repository set | `Inventory()` |
| `ryoku/cli/internal/updater/ryokuset.go:112`, `repoServedSet` | `pacman -Slq ryoku` | Build the target-channel served-name set | `Available(source=Ryoku)` or `RepositoryManager.Packages` |
| `ryoku/cli/internal/updater/ryokuset.go:137`, `dropOlderServes` | `pacman -Qi <name>` | Read each installed version before deciding whether an ordinary update would downgrade it | `Query([name])` |
| `ryoku/cli/internal/updater/ryokuset.go:142`, `dropOlderServes` | `pacman -Si ryoku/<name>` | Read the exact Ryoku-repository version | `Available([name], source=Ryoku)` |
| `ryoku/cli/internal/updater/ryokuset.go:196`, `vercmp` | `vercmp <installed> <available>` | Apply pacman version ordering to the hold-back decision | `CompareVersions(a, b)` |
| `ryoku/cli/internal/updater/ryokuset.go:212`, `refreshDBArgs` | `sudo pacman -Sy --noconfirm` or `-Syy` | Refresh metadata; force refresh after a channel move | `RefreshMetadata(sources, force)` |
| `ryoku/cli/internal/updater/ryokuset.go:232`, `ryokuInstallArgs` | `sudo env ... pacman -S --needed --noconfirm --overwrite <glob> ryoku/<names...>` | Apply the exact Ryoku lane without a system upgrade, permitting selected downgrades on channel moves | `Plan` + `Apply`, scope `Ryoku` |
| `ryoku/cli/internal/updater/ryokuset.go:247`, `systemLanePending` | `pacman -Qu` | List pending distribution updates after the Ryoku metadata refresh | `PendingUpdates(scope=System)` |
| `ryoku/cli/internal/updater/manifest.go:299`, `pacmanSet` through `PacmanInstalled` | `pacman -Qq` | Manifest installed set | `Inventory().Installed` |
| `ryoku/cli/internal/updater/manifest.go:300`, `pacmanSet` through `PacmanExplicit` | `pacman -Qqe` | Manifest explicit/user-added set | `Inventory().ManuallyInstalled` |
| `ryoku/cli/internal/updater/update.go:460`, `splitMetaRemove` | `sudo pacman -Rdd --noconfirm <split-meta>` | Remove obsolete exact-pinned compositor meta before a cross-release downgrade | `Plan` + `Apply` for a guarded migration removal |
| `ryoku/cli/internal/updater/update.go:493`, `unownedFiles` | `pacman -Qo <path>` | Distinguish unowned conflict files from package-owned files before deletion | `OwnerOf(path)` |
| `ryoku/cli/internal/updater/update.go:524`, `systemUpgradeArgs` | `sudo env ... pacman -Syu --noconfirm --overwrite <glob>` | User-requested distribution upgrade, kernel included | `Plan` + `Apply`, scope `System` |
| `ryoku/cli/internal/updater/update.go:532`, `channelSwitchArgs` | `sudo env ... pacman -S ... ryoku-desktop` | Move the packaged desktop to another Ryoku channel in either direction | `Plan` + `Apply`, scope `Ryoku`, `AllowDowngrade` |
| `ryoku/cli/internal/updater/update.go:1075`, `prowlPacmanOwned` | `pacman -Qo <path>` | Decide whether Prowl is package-managed or should self-update | `OwnerOf(path)` |
| `ryoku/cli/internal/updater/update.go:1103`, `clearStalePacmanLock` | checks `/var/lib/pacman/db.lck` and `pgrep -x pacman` | Remove only an abandoned native transaction lock | `LockState()` + `RepairStaleLock()` |
| `ryoku/cli/internal/updater/update.go:1675`, `latestAvailable` | `pacman -Sl ryoku` | Read the available version for one Ryoku package for status | `Available([name], source=Ryoku)` |
| `ryoku/cli/internal/updater/bootguard.go:164`, `revertRelease` | `pacman -Syy --noconfirm` | Force metadata refresh after tracking the previous frozen release | `RefreshMetadata(source=Ryoku, force=true)` |
| `ryoku/cli/internal/updater/bootguard.go:171`, `revertRelease` | `env SNAP_PAC_SKIP=y pacman -S ... ryoku-desktop` | Reinstall the previous Ryoku release after failed boots | `Plan` + `Apply`, scope `Ryoku`, `AllowDowngrade` |

### 1.2 Doctor

| File and function | Command used | Purpose | Suggested interface method |
|---|---|---|---|
| `ryoku/cli/internal/doctor/doctor.go:826`, `reconcilePacmanLock` | checks `/var/lib/pacman/db.lck` and the `pacman` process | Report or delete a stale package database lock | `LockState()` + `RepairStaleLock()` |
| `ryoku/cli/internal/doctor/doctor.go:1072`, `reconcileRyokuSyncDB` | `LC_ALL=C pacman -Sl ryoku` | Load and signature-check the cached Ryoku sync database | `CheckHealth(source=Ryoku)` |
| `ryoku/cli/internal/doctor/doctor.go:1090`, `reconcileRyokuSyncDB` | `sudo pacman -Sy --noconfirm` | Re-fetch metadata after dropping a bad Ryoku sync database | `RefreshMetadata(source=Ryoku, force=true)` |
| `ryoku/cli/internal/doctor/doctor.go:1113`, `reconcileIconFont` | `sudo pacman -S --needed --noconfirm ttf-material-symbols-variable` | Deliver a required icon font to older/dev installs | `Plan` + `Apply(install component)` |
| `ryoku/cli/internal/doctor/doctor.go:1157`, `reconcileDevResidue` | `pacman -Qoq /usr/bin/<name>` | Detect user-home binaries shadowing Ryoku-owned system binaries | `OwnerOf(path)` |
| `ryoku/cli/internal/doctor/doctor.go:3786`, `reconcileOrphans` | `pacman -Qtdq` | Report orphaned dependency packages | `Inventory().Orphans` |
| `ryoku/cli/internal/doctor/doctor.go:3948`, `pkgOwnsFile` | `pacman -Qo <path>` | Find unowned seeded paths that block package adoption | `OwnerOf(path)` |
| `ryoku/cli/internal/doctor/reconcile_asus_aura.go:20`, `installAsusAura` | `sudo pacman -S --needed --noconfirm asusctl` | Install the optional ASUS Aura provider | `Plan` + `Apply(install component)` |
| `ryoku/cli/internal/doctor/reconcile_hardware.go:224`, `removeKepler580` | `sudo pacman -R --noconfirm nvidia-580xx-dkms` | Remove an incompatible NVIDIA driver before restoring Nouveau | `DriverManager.Plan` then package `Apply(remove)` |
| `ryoku/cli/internal/doctor/reconcile_manifest.go:49`, `installManifestPkgs` | `sudo pacman -S --needed --noconfirm <names...>` | Best-effort manifest convergence for newly added packages | `Plan` + `Apply`, component-resolved names |
| `ryoku/cli/internal/doctor/reconcile_multilib.go:55`, `reconcileMultilibRepo` | `sudo pacman -Sy multilib` | Sync metadata after re-enabling the multilib repository | `RepositoryManager.Ensure` + `RefreshMetadata` |
| `ryoku/cli/internal/doctor/reconcile_multilib.go:65`, `installedLib32` | `pacman -Qq` | Find installed `lib32-*` packages that require multilib | `Inventory()` plus backend capability/component classification |
| `ryoku/cli/internal/doctor/reconcile_quickshell.go:74`, `reconcileQuickshell` | `sudo pacman -S --needed --noconfirm quickshell` | Replace a stale foreign Quickshell build with the repository build | `Plan` + `Apply(install component)` |
| `ryoku/cli/internal/doctor/reconcile_quickshell.go:149`, `pkgOwning` | `pacman -Qoq <binary>` | Identify the installed package owning Quickshell | `OwnerOf(path)` |
| `ryoku/cli/internal/doctor/reconcile_quickshell.go:157`, `pkgIsForeign` | `pacman -Qmq` | Decide whether that owner is outside sync repositories | `Inventory()` and `PackageRecord.Source` |
| `ryoku/cli/internal/doctor/reconcile_ryotunes.go:65`, `reconcileRyotunes` | `pacman -Qoq /usr/bin/ryotunes` | Verify that the system Ryotunes binary is package-owned | `OwnerOf(path)` |
| `ryoku/cli/internal/doctor/reconcile_shipped_apps.go:71`, `appInstalledAsDep` | `pacman -Qdq <name>` | Detect an app installed as a dependency rather than explicitly | `Query([name]).ManuallyInstalled` |
| `ryoku/cli/internal/doctor/reconcile_shipped_apps.go:78`, `installShippedApps` | `sudo pacman -S --needed --noconfirm <names...>` | Deliver missing install-once applications | `Plan` + `Apply(install components)` |
| `ryoku/cli/internal/doctor/reconcile_shipped_apps.go:84`, `markAppsExplicit` | `sudo pacman -D --asexplicit --quiet <names...>` | Protect adopted shipped apps from orphan cleanup | `Plan` + `Apply(mark manually installed)` |
| `ryoku/cli/internal/doctor/report.go:103`, `gatherReport` | `pacman -Q <diagnostic names...>` | Capture installed versions of the desktop/compositor stack | `Query(names)` |
| `ryoku/cli/internal/doctor/report.go:104`, `gatherReport` | `pacman -Qtdq` | Capture orphan names for diagnostics | `Inventory().Orphans` |
| `ryoku/cli/internal/doctor/report.go:105`, `gatherReport` | `pacman -Dk` | Capture package database consistency diagnostics | `CheckHealth()` |

The channel reconciler in `doctor.go` also reads and writes
`/etc/pacman.conf`, invokes `pacman-key`, and deletes
`/var/lib/pacman/sync/ryoku.*`. Those operations belong to
`RepositoryManager`, even though only its `pacman -Sl` and `pacman -Sy` calls
appear in this executable inventory.

### 1.3 Local artifacts and compositor switching

| File and function | Command used | Purpose | Suggested interface method |
|---|---|---|---|
| `ryoku/cli/internal/ryotunesrelease/client.go:149`, `defaultInstalledVersion` | `pacman -Q <name>` | Read installed Ryotunes version | `Query([component])` |
| `ryoku/cli/internal/ryotunesrelease/client.go:164`, `defaultInspect` | `LC_ALL=C pacman -Qip <artifact>` | Validate local artifact name, version, and architecture | `InspectArtifact(path)` |
| `ryoku/cli/internal/ryotunesrelease/client.go:241`, `defaultInstall` | privileged `pacman -U --noconfirm <staged-artifact>` | Install a verified local native package | `PlanArtifactInstall` + `Apply` |
| `ryoku/cli/internal/ryotunesrelease/client.go:289`, `defaultVercmp` | `vercmp <installed> <release>` | Enforce upgrade-only external releases | `CompareVersions(a, b)` |
| `ryoku/cli/wm.go:294`, `cmdWmUse` | `sudo pacman -S --needed --noconfirm <incoming-meta>` | First attempt to install the selected compositor variant | `Plan` + `Apply(install component)` |
| `ryoku/cli/wm.go:299`, `cmdWmUse` | `sudo pacman -Rdd --noconfirm <outgoing-meta>` | Compatibility fallback for old mutually-conflicting metas | `Plan` + `Apply` for a guarded migration removal |
| `ryoku/cli/wm.go:302`, `cmdWmUse` | `sudo pacman -S --needed --noconfirm <incoming-meta>` | Retry install after removing the obsolete conflicting meta | `Revalidate` + `Apply` |
| `ryoku/cli/wm.go:438`, `removePreviousCompositor` | `sudo pacman -Rns --noconfirm <reviewed-targets...>` | Apply the already-reviewed compositor removal cascade | `Revalidate(plan)` + `Apply(plan)` |
| `ryoku/cli/wm.go:522`, `packageAvailable` | `pacman -Si <meta>` | Check whether the selected compositor exists on this channel | `Available([component])` |
| `ryoku/cli/wm.go:529`, `packageInstalled` | `pacman -Qq <meta>` | Check whether an outgoing compositor meta is installed | `Query([component])` |

## 2. Installer callers

### 2.1 Existing-system installer (`ryoku-shell-installer`)

The command vectors in `distro.go:37-44` are executed through the functions
listed below. This is an existing partial distro seam, but it exposes argv and
Arch package names rather than the semantic backend contract.

| File and function | Command used | Purpose | Suggested interface method |
|---|---|---|---|
| `ryoku-shell-installer/distro.go:152`, `installedPkg` | `pacman -Qq <name>` | Generic installed check used by detection, planning, and uninstall | `Query([component])` |
| `ryoku-shell-installer/detect.go:275`, `detect` | `pacman -Qq` | Discover rival `illogical-impulse-*` packages | `Inventory()` |
| `ryoku-shell-installer/engine.go:518`, `stepLegacy` | `sudo pacman -R --noconfirm omarchy-keyring` | Remove a retired rival repository keyring package | `Plan` + `Apply(remove)` |
| `ryoku-shell-installer/engine.go:525`, `stepSysupgrade` via `archLinux.updateCmd` | `sudo pacman -Syu --noconfirm` | Bring the existing Arch system current before conversion | `Plan` + `Apply`, scope `System` |
| `ryoku-shell-installer/engine.go:730`, `stepConflicts` via `removeArgs` | `sudo pacman -R --noconfirm <rival/blocker names...>` | Remove rival shells and packages that block the desktop transaction | `Plan` + `Apply(remove)` |
| `ryoku-shell-installer/engine.go:746`, `stepConflicts` | `sudo pacman -Rdd --noconfirm <zsh-frameworks...>` | Break an obsolete provider conflict before installing Ryoku's replacement | Guarded migration `Plan` + `Apply` |
| `ryoku-shell-installer/engine.go:818`, `stepRepo` | `sudo pacman -Sy` | Sync the newly-added Ryoku repository | `RepositoryManager.Ensure` + `RefreshMetadata` |
| `ryoku-shell-installer/engine.go:900`, `stepPackages` through `desktopPacmanArgs` | `sudo pacman -Syu --needed --noconfirm --overwrite <glob> <desktop-set...>` | Install the desktop and base package set while completing a system upgrade | `InstallerTarget.Plan` + `Apply` |
| `ryoku-shell-installer/engine.go:927`, `dropSatisfied` | `pacman -T <requirements...>` | Keep only dependencies not already satisfied by an installed provider | `Plan(install)` or `ResolveRequirements` |
| `ryoku-shell-installer/engine.go:996`, `stepDrivers` | `sudo pacman -Syu --noconfirm` | Refresh a resumed install before vendor driver scripts run | `RefreshMetadata` plus system `Plan` + `Apply` |
| `ryoku-shell-installer/engine.go:1336`, `stepAUR` | `sudo pacman -U --noconfirm <built-yay-artifacts...>` | Bootstrap the AUR helper from locally built packages | `InspectArtifact` + `PlanArtifactInstall` + `Apply` |
| `ryoku-shell-installer/lifecycle.go:139`, `runUninstall` | `sudo -n pacman -R --noconfirm <Ryoku names...>` | Remove the installed Ryoku package set | `Plan` + `Apply(remove components)` |

`sudoArgv` in `engine.go:418` wraps every guarded pacman command with
`OMARCHY_ALLOW_DIRECT_PACMAN=1`; the Arch adapter needs a native execution
environment hook rather than exposing that variable to callers.

### 2.2 Full-system shell installer

| File and function | Command used | Purpose | Suggested interface method |
|---|---|---|---|
| `installation/backend/lib/aur.sh:73`, `ryoku_aur` bootstrap | `sudo pacman -S --needed --noconfirm base-devel git curl` | Install the AUR build toolset in the target | `InstallerTarget.Plan` + `Apply` |
| `installation/backend/lib/bootloader.sh:247`, `chroot_has_pkg` | `arch-chroot /mnt pacman -Q <name>` | Select boot finalization based on target package presence | `InstallerTarget.Query` |
| `installation/backend/lib/deploy.sh:129`, `ryoku_deploy_packages` | `arch-chroot /mnt pacman -Qq tlp` | Avoid installing `asusctl` when target TLP conflicts | `InstallerTarget.Query` |
| `installation/backend/lib/deploy.sh:161`, `ryoku_deploy_packages` | target `pacman --config <offline> -S --needed <desktop-set...>` | Install the desktop from the baked offline repository | `InstallerTarget.Plan` + `Apply`, offline source |
| `installation/backend/lib/deploy.sh:166`, `ryoku_deploy_packages` | same command with `--overwrite '*'` | Retry a half-extracted offline target and adopt its files | `InstallerTarget.Revalidate` + recovery `Apply` |
| `installation/backend/lib/deploy.sh:186`, `ryoku_deploy_packages` | `arch-chroot /mnt pacman -Sy --noprogressbar` | Sync target repositories for an online install | `InstallerTarget.RefreshMetadata` |
| `installation/backend/lib/deploy.sh:187`, `ryoku_deploy_packages` | `arch-chroot /mnt pacman -S --needed <desktop-set...>` | Install the online Ryoku desktop set | `InstallerTarget.Plan` + `Apply` |
| `installation/backend/lib/deploy.sh:189`, `ryoku_deploy_packages` | same command with `--overwrite '*'` | Retry online installation after a partial extraction/file conflict | `InstallerTarget.Revalidate` + recovery `Apply` |
| `installation/backend/lib/deploy.sh:220`, `ryoku_deploy_version_skew` | `arch-chroot /mnt pacman -Si ryoku-desktop` | Compare the ISO payload stamp with the repository desktop version | `InstallerTarget.Available` |
| `installation/backend/lib/offline.sh:94`, `ryoku_offline_pacman` | `arch-chroot /mnt pacman --config <offline> <args...>` | Execute every offline target query/transaction against only the baked repository | Arch `InstallerTarget` executor |
| `installation/backend/lib/offline.sh:186`, `ryoku_offline_aur` | target `pacman --config <offline> -Sp <name>` | Filter baked supplemental packages to those resolvable offline | `InstallerTarget.Plan(install)` |
| `installation/backend/lib/offline.sh:190`, `ryoku_offline_aur` | target `pacman --config <offline> -S --needed <names...>` | Install the resolvable baked supplemental set | `InstallerTarget.Apply` |

The wrapper `ryoku_offline_pacman` at `offline.sh:94` is the common executor
for the four offline `-S`/`-Sp` occurrences above. Pacstrap, keyring setup,
repository file generation, and target chroot selection belong to the Arch
installer backend, not to the runtime adapter.

## 3. Other runtime modules and installed helpers

### 3.1 Window-manager library, Hub, and Rashin

| File and function | Command used | Purpose | Suggested interface method |
|---|---|---|---|
| `ryoku/wm/reclaim.go:204`, `pacmanInstalled` | `pacman -Q <name>` | Filter provider-declared packages to installed outgoing-only targets | `Query(names)` |
| `ryoku/wm/reclaim.go:254`, `pacmanRemovalOnce` | `pacman -Rs --print --print-format %n --assume-installed ... <targets...>` | Ask the native solver for the exact unused-dependency removal cascade | `Plan(remove, RemoveUnusedDependencies=true)` |
| `ryoku/wm/reclaim.go:275`, `pacmanInstalledSizes` | `pacman -Qi <names...>` | Sum installed size for the compositor removal preview | `Query(names).InstalledSize` |
| `ryoku/wm/hyprland/config_plugins_tier.go:690`, `packageVersion` | `pacman -Q <name>` | Report compositor/header package versions in the plugin roster | `Query([component])` |
| `ryoku/hub/backend/wm.go:174`, `pkgAvailable` | `pacman -Si <meta>` | Mark a compositor target available in Settings | `Available([component])` |
| `ryoku/hub/backend/gpuapply.go:303`, `pacmanInstall` | `pacman -S --needed --noconfirm <names...>` | Install GPU passthrough packages; caller is already privileged | `DriverManager.Plan`, then package `Apply` |
| `ryoku/hub/backend/gpuapply.go:310`, `pkgInstalled` | `pacman -Q <name>` | Check passthrough package presence | `Query([component])` |
| `ryoku/rashin/backend/index.go:438`, `packagesBody` | `pacman -Qq` | Count all installed packages in the generated system vault | `Inventory()` |
| `ryoku/rashin/backend/index.go:443`, `packagesBody` | `pacman -Qqe` | List explicit packages in the generated system vault | `Inventory().ManuallyInstalled` |
| `ryoku/rashin/backend/index.go:535`, `pacmanVersion` | `pacman -Q <name>` | Add selected installed versions to indexed system facts | `Query([name])` |
| `ryoku/rashin/backend/quicktools.go:108`, `toolSystemQuery` | `pacman -Qi <name>` | Agent system-query tool returns package details | `Query([name])` with neutral rendering |
| `ryoku/rashin/backend/quicktools.go:110`, `toolSystemQuery` | shell `pacman -Qq \| wc -l` | Agent system-query tool returns installed package count | `Inventory()` |

Canonical `reclaim.go` is copied into
`ryoku/cli/vendor/ryoku-wm`, `ryoku/hub/backend/vendor/ryoku-wm`,
`ryoku/rashin/backend/vendor/ryoku-wm`, `ryoku/shell/ipc/vendor/ryoku-wm`, and
`ryoku/shell/ryogami/daemon/vendor/ryoku-wm`. Updating the canonical dependency
and regenerating vendors should replace all copies; they are not five separate
backend call sites.

### 3.2 Extras, drivers, and hardware helpers

| File and function | Command used | Purpose | Suggested interface method |
|---|---|---|---|
| `system/extras/ryoku-pkg-add:18`, script body | `sudo pacman -Syu --needed --noconfirm <names...>` | Official-repository package actuator used by Extras and app setup | `Plan` + `Apply(install)`, with system-upgrade policy made explicit |
| `system/extras/ryoku-pkg-remove:14`, script body | `sudo pacman -Rs --noconfirm <names...>` | Remove extras plus newly unused dependencies | `Plan` + `Revalidate` + `Apply(remove)` |
| `system/extras/ryoku-pkg-multilib:30`, script body | `sudo pacman -Sy` | Refresh after enabling multilib | `RepositoryManager.Ensure` + `RefreshMetadata` |
| `system/extras/ryoku-pkg-cachyos:67`, script body | `sudo pacman -Sy` | Refresh after enabling CachyOS | `RepositoryManager.Ensure` + `RefreshMetadata` |
| `system/extras/ryoku-pkg-cachyos:71`, script body | `sudo pacman -S --needed --noconfirm cachyos-keyring` | Adopt the repository keyring package after bootstrap trust | `RepositoryManager.EnsureTrust` plus package `Apply` |
| `system/extras/ryostore-install:83`, `load_pkgset` | `pacman -Qq` | Cache installed names once for bundle status | `Inventory()` |
| `system/extras/ryostore-install:87`, `pkg_installed` | `pacman -Qq <name>` | Live installed check after bundle mutation | `Query([component])` |
| `system/extras/ryostore-install:92`, `pkg_official` | `pacman -Si <name>` | Route a bundle item to official repositories instead of AUR | `Available([component])` and `PackageSource` |
| `system/extras/ryostore-install:100`, `pkg_owners` | `pacman -Qq <item>` | Resolve an installed provider to real package names for removal | `Query([requirement])` returning provider records |
| `system/hardware/drivers/amd.sh:43`, `pkg_installed` | `pacman -Qq <name>` | Filter already-installed AMD packages | `Query(names)` |
| `system/hardware/drivers/amd.sh:45`, `install_pkgs` | `[sudo] pacman -S --needed --noconfirm <missing...>` | Install AMD driver components | `DriverManager.Plan` + package `Apply` |
| `system/hardware/drivers/intel.sh:43`, `pkg_installed` | `pacman -Qq <name>` | Filter already-installed Intel packages | `Query(names)` |
| `system/hardware/drivers/intel.sh:45`, `install_pkgs` | `[sudo] pacman -S --needed --noconfirm <missing...>` | Install Intel driver components | `DriverManager.Plan` + package `Apply` |
| `system/hardware/drivers/vulkan.sh:43`, `pkg_installed` | `pacman -Qq <name>` | Filter installed generic Vulkan components | `Query(names)` |
| `system/hardware/drivers/vulkan.sh:45`, `install_pkgs` | `[sudo] pacman -S --needed --noconfirm <missing...>` | Install generic Vulkan components | `DriverManager.Plan` + package `Apply` |
| `system/hardware/drivers/nvidia.sh:58`, `pkg_installed` | `pacman -Qq <name>` | Filter installed NVIDIA components | `Query(names)` |
| `system/hardware/drivers/nvidia.sh:60`, `install_pkgs` | `[sudo] pacman -S --needed --noconfirm <missing...>` | Install selected NVIDIA components | `DriverManager.Plan` + package `Apply` |
| `system/hardware/drivers/nvidia.sh:116`, `prebuilt_for` | `pacman [--config <target-conf>] -Si <candidate>` | Test whether a kernel-specific NVIDIA package is available | `Available([driver component])` through the target/backend context |
| `system/hardware/drivers/nvidia.sh:132`, `have_module_pkg` | `pacman -Qq` filtered by NVIDIA module-name regex | Detect any installed NVIDIA module package | `Inventory()` plus driver component classification |
| `system/hardware/drivers/ryoku-nvidia-guard:76`, `nvidia_module_pkg` | `pacman -Qq` filtered to module packages | Select the installed module package for kernel compatibility checks | `Inventory()` plus driver component classification |
| `system/hardware/drivers/ryoku-nvidia-guard:84`, `nvidia_dkms_pkg` | `pacman -Qq` filtered to DKMS package names | Decide whether an initramfs rebuild can recover a missing module | `Inventory()` plus driver component classification |
| `system/hardware/gpu/ryoku-gpu-lib32:91`, `main` | `[sudo] pacman -Syu --needed --noconfirm <lib32 names...>` | Enable/install 32-bit graphics support | `DriverManager.Plan32BitGraphics` + system package `Apply` |

### 3.3 Shell, recovery, and applications

| File and function | Command used | Purpose | Suggested interface method |
|---|---|---|---|
| `ryoku/shell/scripts/stash-install.sh:350`, `install_pacman` | `pkexec pacman -U --noconfirm <artifact>` | Install a local Arch package dropped into the stash | `InspectArtifact` + `PlanArtifactInstall` + `Apply` |
| `ryoku/shell/scripts/ryoku-sysinfo:71`, script body | `pacman -Qq` | Print total installed package count | `Inventory()` |
| `ryoku/shell/scripts/ryoku-profile-stats:51`, script body | `pacman -Qen` | Count explicit native-repository packages | `Inventory()` filtered by manual/source state |
| `ryoku/shell/scripts/ryoku-profile-stats:52`, script body | `pacman -Qm` | Count foreign packages | `Inventory()` filtered by source |
| `ryoku/apps/ryovm/bin/ryovm:141`, `cmd_setup` fallback | `sudo pacman -S --needed --noconfirm spice-gtk libisoburn` | Install host VM viewer/ISO tools if the Ryoku package actuator is absent or fails | `Plan` + `Apply(install components)` |
| `bin/ryoku-track:36`, script body | `sudo pacman -S --needed --noconfirm <build-toolset...>` | Prepare an Arch source-channel development checkout | `Plan` + `Apply`, development scope |
| `bin/ryoku-recovery:132`, script body | `sudo pacman -Syu --needed --noconfirm <base-set...>` | Reinstall the base package set during destructive recovery | Recovery-specific `Plan` + `Apply`, scope `System` |
| `bin/ryoku-recovery:191`, script body | `pacman -Q ryoku-desktop` | Preserve packaged-channel identity after a source redeploy | `Query([desktop.core])` or backend identity |
| `bin/ryoku-recovery:209`, script body | `sudo pacman -S --needed --noconfirm ryogami matugen` | Restore two required runtime components after recovery | `Plan` + `Apply(install components)` |

`ryoku/shell/deploy.sh` is the checkout/recovery deployment implementation. Its
package calls are developer-channel callers, not installed-package runtime:

| File and function | Command used | Purpose | Suggested interface method |
|---|---|---|---|
| `ryoku/shell/deploy.sh:120`, `check_renderer` | interactive `sudo pacman -S quickshell` | Offer to install a missing renderer during deploy | Development `Plan` + `Apply` |
| `ryoku/shell/deploy.sh:474`, top-level plugin build | `pacman -Q qt6-base` | Stamp the locally built QML plugin with its Qt version | `Query([runtime.qt])` |
| `ryoku/shell/deploy.sh:687`, `_pac_ryotunes` | `sudo pacman -Syu --needed --noconfirm --overwrite <glob> ryotunes` | Install/update the packaged external app during deploy | `Plan` + `Apply`; avoid coupling a component install to system upgrade |
| `ryoku/shell/deploy.sh:690`, top-level deploy | `pacman -Q ryotunes` | Report installed Ryotunes version | `Query([app.ryotunes])` |
| `ryoku/shell/deploy.sh:698`, top-level deploy | `pacman -Qo <path>` | Find unowned files before retrying package adoption | `OwnerOf(path)` |
| `ryoku/shell/deploy.sh:702`, top-level deploy | `pacman -Q ryotunes` | Report version after conflict recovery | `Query([app.ryotunes])` |

## 4. ISO, release, and validation-only invocations

These uses are Arch build/validation infrastructure. They should consume an
Arch `ReleaseBackend`, `InstallerTarget`, or test fixture, not the installed
runtime `SystemBackend`.

### 4.1 Offline ISO repository builder

All `${PAC[@]}` calls below are `pacman` as root or `sudo pacman`, with an
isolated `--config`, `--dbpath`, and usually `--cachedir`.

| File and function/phase | Command used | Purpose | Suggested owner |
|---|---|---|---|
| `installation/iso/offline-repo.sh:129`, initial sync | `pacman -Sy` | Sync throwaway build databases | `ReleaseBackend.RefreshMetadata` |
| `offline-repo.sh:141`, name validation | `pacman -Si <name>` | Verify a concrete package name exists | `ReleaseBackend.Available` |
| `offline-repo.sh:143`, provider diagnosis | `pacman -Ss ^<name>$` | Diagnose a virtual/provider-only name | `ReleaseBackend.SearchProviders` |
| `offline-repo.sh:144`, provider diagnosis | `pacman -Sp <name>` | Confirm that the virtual requirement resolves | `ReleaseBackend.Plan` |
| `offline-repo.sh:156`, closure resolution | `pacman -Sp --print-format %n <set...>` | Resolve the full dependency closure | `ReleaseBackend.Plan` |
| `offline-repo.sh:171`, closure download | `pacman -Sw --needed <set...>` | Download the closure without installing | `ReleaseBackend.Fetch(plan)` |
| `offline-repo.sh:185`, required NVIDIA variants | `pacman -Sw --needed <variant>` | Fetch each conflicting required driver separately | `ReleaseBackend.Fetch` |
| `offline-repo.sh:189`, optional NVIDIA variants | `pacman -Sw --needed <variant>` | Best-effort fetch optional driver variants | `ReleaseBackend.Fetch` |
| `offline-repo.sh:346`, `bake_aur_set` | `pacman -Sw --needed <runtime-deps...>` | Fetch official runtime dependencies of built supplemental packages | `ReleaseBackend.Fetch` |
| `offline-repo.sh:387`, cache repair | `pacman -Sw --needed <set...>` | Re-download corrupt closure artifacts | `ReleaseBackend.Fetch` |
| `offline-repo.sh:389`, cache repair | `pacman -Sw --needed <driver>` | Re-download corrupt driver artifacts | `ReleaseBackend.Fetch` |
| `offline-repo.sh:454`, `verify_offline_closure` | `pacman -Sy` against the baked file repository | Load the assembled offline repository | `ReleaseBackend.RefreshMetadata` |
| `offline-repo.sh:476`, `verify_offline_closure` | `pacman -Sp --print-format %n <pacstrap-set...>` | Prove the target install set resolves offline | `ReleaseBackend.Plan` |
| `offline-repo.sh:501`, `verify_offline_closure` | `pacman -Qlp <artifact-files...>` | Detect cross-package file conflicts in the baked closure | `ReleaseBackend.InspectArtifact(...).Files` |
| `offline-repo.sh:521`, retry loop | `pacman -Sy` | Refresh after an incomplete mirror snapshot | `ReleaseBackend.RefreshMetadata` |
| `offline-repo.sh:522`, retry loop | `pacman -Sw --needed <set...>` | Re-fetch closure before re-verification | `ReleaseBackend.Fetch` |

### 4.2 Developer and integration gates

| File and function/phase | Command used | Purpose | Suggested owner |
|---|---|---|---|
| `bin/ryoku-dev-verify-pkgbuild-deps:76`, dependency loop | `pacman -Si <dependency>` | Verify every PKGBUILD dependency resolves in sync repositories | Arch release validation, `Available` semantics |
| `installation/tests/container-install.sh:46` | `pacman -Sy --noconfirm --needed <keyrings...>` | Prepare the Arch integration container | Arch test fixture |
| `installation/tests/container-install.sh:49` | `pacman -Syu --noconfirm --needed jq gnupg libarchive` | Install integration tools | Arch test fixture |
| `installation/tests/container-install.sh:52` | `pacman -Syu --noconfirm --needed <toolchain...>` | Install build tools | Arch test fixture |
| `installation/tests/container-install.sh:108` | `pacman -Sy --noconfirm` | Sync the locally configured test repository | `RefreshMetadata` contract test |
| `installation/tests/container-install.sh:110` | `pacman -S --noconfirm ryoku-desktop` | Install the packaged desktop under test | `Plan`/`Apply` contract test |
| `installation/tests/container-install.sh:115` | `pacman -Qq ryoku-desktop-hyprland` | Assert the default variant is installed | `Query` contract test |
| `installation/tests/container-install.sh:286` | `pacman -Qo <path>` | Assert packaged ownership | `OwnerOf` contract test |
| `installation/tests/container-install.sh:292` | `pacman -Ql ryoku-desktop` | Assert the desktop package file list | Package/artifact file-list test |
| `installation/tests/container-install.sh:313` | `pacman -Qo <artifact>` | Assert installed payload ownership | `OwnerOf` contract test |
| `installation/tests/container-install.sh:341` | `pacman -S --needed --noconfirm ryoku-desktop-niri` | Verify compositor variants coexist | `Apply(install component)` contract test |
| `installation/tests/container-install.sh:342` | `pacman -Qq ryoku-desktop-niri` | Assert niri variant installed | `Query` contract test |
| `installation/tests/container-install.sh:343` | `pacman -Qq ryoku-desktop-hyprland` | Assert hyprland variant survived | `Query` contract test |
| `installation/tests/container-install.sh:346` | `pacman -Ql ryoku-desktop-niri` | Assert compositor package file isolation | Package file-list test |
| `tests/controllers.sh:74`, package loop | `pacman -Si <name>` | Verify controller packages remain available | Arch availability fixture |

The CI jobs also bootstrap their Arch containers directly. These are execution
environment setup calls, not product adapter consumers:

| File and job step | Command used | Purpose | Suggested owner |
|---|---|---|---|
| `.github/workflows/release-ledger.yml:24` | `pacman -Sy --noconfirm --needed git` | Bootstrap source checkout tooling | Arch CI image setup |
| `.github/workflows/release-ledger.yml:30` | `pacman -Syu --noconfirm --needed rclone jq` | Install release-ledger tooling | Arch CI image setup |
| `.github/workflows/refresh-ryotunes.yml:35` | `pacman -Syu --noconfirm --needed <release tools...>` | Install import/sign/publish tooling | Arch release job setup |
| `.github/workflows/publish-repo.yml:100` | `pacman -Sy --noconfirm --needed git` | Bootstrap repository source checkout | Arch release job setup |
| `.github/workflows/publish-repo.yml:148` | `pacman -Syu --noconfirm --needed <build tools...> rclone` | Install package-build and publication tools | Arch release job setup |
| `.github/workflows/publish-repo.yml:289` | `pacman -Sy --noconfirm --needed git` | First keyring-sensitive checkout bootstrap attempt | Arch release job setup |
| `.github/workflows/publish-repo.yml:298` | `pacman -Sy --noconfirm --needed git` | Retry checkout bootstrap after keyring repair | Arch release job setup |
| `.github/workflows/publish-repo.yml:321` | `pacman -Sy --noconfirm --needed git` | Bootstrap publication stage checkout | Arch release job setup |
| `.github/workflows/publish-repo.yml:328` | `pacman -Syu --noconfirm --needed rclone jq libarchive github-cli` | Install publication-stage tooling | Arch release job setup |
| `.github/workflows/install-test.yml:39` | `pacman -Sy --noconfirm --needed git` | Bootstrap integration-test checkout | Arch test job setup |
| `.github/workflows/install-test.yml:140` | `pacman -Sy --noconfirm --needed archlinux-keyring` | Refresh the container keyring before install | Arch test fixture |
| `.github/workflows/install-test.yml:141` | `pacman -Su --noconfirm --needed $RYOKU_PKGS` | Upgrade/install the package set under test | Arch `Plan`/`Apply` integration test |
| `.github/workflows/build-iso-reusable.yml:192` | `pacman -Sy --noconfirm archlinux-keyring` | Refresh the ISO builder keyring | Arch ISO job setup |
| `.github/workflows/build-iso-reusable.yml:196` | `pacman -Su --noconfirm --needed <ISO tools...>` | Install the ISO build toolchain | Arch ISO job setup |

Other shell tests create stub executables named `pacman` or assert logged
command text; they do not invoke the host package manager and therefore are not
additional package-manager callers.

## 5. Caller chains

The direct executable sites above are reached through these product-level
callers. These chains are the migration seams; changing only the leaf command
would leave Arch output parsing and policy in the caller.

| Product caller | Current path to pacman | Backend boundary |
|---|---|---|
| `ryoku update`, `status`, `track`, rollback, and boot guard | `ryoku/cli/main.go` -> `internal/updater` -> `internal/sys`/direct commands | Inject `SystemBackend.Packages` and `Repositories` at the CLI composition root |
| `ryoku doctor`, `verify`, and `debug` | `main.go` -> `internal/doctor` -> direct commands, updater manifest helpers, and `sys.PkgInstalled` | Keep reconciler policy; inject package/repository facts and plans |
| `ryoku wm use` | `main.go` -> `wm.go` -> `ryoku-wm.Reclaim` plus direct install/remove | Resolve provider component IDs, then use package plan/revalidate/apply |
| Settings compositor sheet | Hub QML -> `ryoku-hub wm preview` -> `hub/backend/wm.go` -> `ryoku-wm.Reclaim` | Hub should receive neutral availability and removal-plan data |
| Settings GPU passthrough | Hub QML -> `ryoku-hub gpu apply` -> `gpuapply.go` | `DriverManager` owns component choice; `PackageManager` applies the plan |
| Full ISO installer | `ryoku-tui` -> `installation/backend/ryoku-install` -> `lib/deploy.sh`, `offline.sh`, drivers, AUR, bootloader | Separate Arch `InstallerTarget`; do not route target chroot operations through runtime globals |
| Existing-system installer | `ryoku-shell-install` engine -> `distro` argv vectors and remaining direct pacman calls | Replace argv abstraction with injected semantic installer backend |
| Ryostore bundles | Ryostore backend/Hub -> `ryostore-install` -> `ryoku-pkg-add`, `ryoku-pkg-aur-add`, `ryoku-pkg-remove` | Route bundle component IDs through package source resolution and transaction plans |
| VM setup | `ryovm setup` -> package actuator -> direct pacman fallback | Call a neutral component-install entry point; remove the fallback command |
| Driver install and guard | installer, doctor, or Hub -> `system/hardware/drivers/*.sh` and GPU helpers | `DriverManager` selects components; target/runtime package manager queries and applies |
| Rashin package facts | indexer/agent tool -> direct pacman probes | Inject a read-only package facts service and render neutral records |
| Stash local package | shell action -> `stash-install.sh` -> `pkexec pacman -U` | Identify artifact through `InspectArtifact`, then plan and apply with the selected backend |
| Package launcher provider | QML `Packages.qml` -> `gpk` with manager filter `pacman,aur` | Supply backend-neutral search/list results; QML must not encode manager names |

## 6. Non-command pacman dependencies

These sites do not start `pacman`, but they consume its files, process state, or
output protocol and therefore still belong in the adapter migration surface.

| File and owner | Native dependency | Purpose | Suggested interface owner |
|---|---|---|---|
| `ryoku/cli/internal/sys/release.go` | `/etc/pacman.conf` and `/var/lib/pacman/sync` | Read/change the Ryoku channel and delete stale sync metadata | `RepositoryManager` |
| `ryoku/cli/internal/doctor/doctor.go`, `reconcileRyokuChannel` and `reconcilePacnew` | `/etc/pacman.conf`, `.pacnew`, and the Ryoku keyring | Heal repository configuration and native config-file upgrades | `RepositoryManager` plus neutral config-conflict findings |
| `ryoku/cli/internal/doctor/report.go` | `/etc/pacman.conf` and `/var/log/pacman.log` | Include native repository/log state in a support report | Backend diagnostic report data |
| `system/hardware/power/ryoku-power-cutover:960` | `/var/lib/pacman/db.lck` | Wait for a package transaction to release its database before session cutover | Package transaction events or `LockState()` |
| `ryoku/shell/quickshell/shell/modules/bar/barstyles/qsbar/modules/ParticleStream.qml:1034` | tails `/var/log/pacman.log` | Drive update particles from native transaction log writes | Backend `EventSink`; QML should not tail a native log |
| `ryoku/hub/quickshell/pages/ProfilePage.qml:47` | first timestamp in `/var/log/pacman.log` | Fallback estimate for installation date | Neutral system-install identity/fact |
| `ryoku/shell/deploy.sh:665-673` | reads/appends/rewrites `[ryoku]` in `/etc/pacman.conf` | Point a checkout/recovery box at the desired package source | `RepositoryManager` in the development workflow |
| `system/extras/ryoku-pkg-multilib` and `ryoku-pkg-cachyos` | mutate `/etc/pacman.conf` under a shared lock | Enable optional Arch repositories before refreshing them | `RepositoryManager.Ensure` |
| `bin/ryoku-dev-verify-pkgbuild-deps:44` | reads `/var/lib/pacman/sync/*.db` archives | Collect virtual provides for PKGBUILD dependency validation | Arch `ReleaseBackend` metadata reader |

## 7. Adapter implications

The first Arch adapter must cover more than install and installed-check calls:

1. `Inventory` must return installed, explicit/manual, source/foreign, orphan,
   provider, version, architecture, and installed-size facts in one consistent
   model. The current code reconstructs these through `-Qq`, `-Qqe`, `-Qm`,
   `-Qdq`, `-Qtdq`, and `-Qi`.
2. `Available` must preserve source identity. The updater deliberately asks for
   `ryoku/<name>` rather than an unqualified name, and the NVIDIA and Ryostore
   paths distinguish a concrete repository package from a virtual/provider or
   supplemental package.
3. Removal needs a native solver plan, a state fingerprint, revalidation, and
   exact application. The compositor flow already previews `pacman -Rs
   --print`, but applies a separately assembled `-Rns` command.
4. Local artifacts require both metadata and file-list inspection. Ryotunes
   needs identity/version/architecture, the stash needs native artifact
   recognition, and the ISO verifier needs declared files.
5. Repository health, trust, metadata refresh, and source configuration must be
   separate from package application. Current callers mix `/etc/pacman.conf`,
   sync database deletion, `pacman-key`, and `pacman -Sy`.
6. Installer-target and release-build operations need the same semantic data
   shapes but a different executor and authority boundary. An installed-system
   adapter must not acquire `arch-chroot`, isolated build databases, download
   caches, or offline-closure publication.
7. Package-manager output must stop crossing presentation boundaries. Doctor,
   updater status, Rashin, Hub, and the WM removal preview currently parse or
   expose pacman-shaped text.

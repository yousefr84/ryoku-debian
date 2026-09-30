# CLI backend integration map

## Status and scope

This document audits the installed-runtime boundary around `ryoku/cli` before
any `SystemBackend` wiring is introduced. It maps current ownership and future
dependency direction only. It does not add an interface, select a provider,
change command behavior, or authorize an implementation patch.

The audit covers:

- the two `main` packages under `ryoku/cli`;
- the CLI's internal updater, doctor, system, and Ryotunes packages;
- `ryoku-wm` only where `ryoku wm use` directly relies on its package-removal
  planner;
- the recovery and source-track scripts only where the CLI directly delegates
  a command to them.

Hub, Rashin, the installed shell, the installers, release jobs, and general
system scripts are separate process or authority boundaries. They are mentioned
only to prevent the CLI composition root from absorbing them.

This map uses the contracts already present in `ryoku/backend`. The broader
semantics in `backend-design.md` are still the target. In particular, the
current `PackageTransactions`, `RepositoryManager`, and `BootManager` contracts
do not yet express every refresh, update-scope, artifact, diagnostic, snapshot,
or plan/revalidation operation identified below. A row naming a facet assigns
semantic ownership; it does not claim that today's method set is sufficient.

## Audit method

The repository asks semantic and structural searches to use Prowl first.
`prowl-agent overview` was attempted, but the executable was unavailable in the
audit environment. The fallback was targeted `rg` searches limited to
`ryoku/cli`, followed by reads of the located files and the direct
`ryoku/wm/reclaim.go` dependency. The existing migration maps were used to
cross-check the result.

## Runtime boundary at a glance

```text
shell / Hub / human / systemd
             |
             v
       ryoku/cli/main.go
  startup, parse, dispatch, exit status
             |
      +------+--------+
      |      |        |
      v      v        v
   updater  doctor   wm command
   policy   policy   switch policy
      |      |        |
      +------+--------+
             |
     future narrow backend facets
 PackageFacts | PackageTransactions | RepositoryManager
             BootManager | DriverManager
             |
       selected provider
```

There is no runtime backend edge today. `ryoku/cli/go.mod` imports
`ryoku-i18n` and `ryoku-wm`, but not `ryoku-backend`. Native dependencies are
created implicitly at their call sites through `os/exec`, `internal/sys`, and
package-level function variables.

## 1. CLI entry points and startup lifecycle

### 1.1 Installed lifecycle CLI

`ryoku/cli/main.go:main` is the installed `ryoku` process root. Its startup is:

1. scrub inherited Quickshell crash state with `scrubQuickshellCrashEnv`;
2. initialize translations with `i18n.Use("")`;
3. require a command argument;
4. dispatch directly from a `switch` over `os.Args[1]`;
5. render any returned error through `die` and exit nonzero.

Dispatch has three shapes:

| Shape | Commands | Current owner |
|---|---|---|
| Internal application package | `update`, `materialize`, `reset`, `rollback`, `boot-guard`, `snapshots`, `status`, `version`, `deploy`; `doctor`, `verify`, `debug`; `keyring`, `security-key`, `keyboard`, `import` | The corresponding `internal/*` package |
| Main-package command | `wm`, `recovery`, `track`, `plugin` | `wm.go`, `recovery.go`, `track.go`, `plugin*.go` |
| Thin process delegation | `reload` | `internal/sys.Run("ryoku-shell", "reload")` |

The backend-sensitive commands are `update`, packaged `rollback`, packaged
`track`, `boot-guard`, `status`, `doctor`, `verify`, `debug`, and `wm use`.
`materialize` and `reset` are deliberately not package-manager operations.
Commands such as keyring, security-key, keyboard, import, and plugin management
should not be given a broad backend merely because they share the binary.

### 1.2 Release manifest generator

`ryoku/cli/cmd/ryoku-manifest/main.go:main` is a separate build-time command. It
reads Arch-oriented package lists, including `system/packages/aur.packages`, and
emits the release control manifest. It is not an installed-runtime composition
root. Its eventual input is the logical component catalog and backend-resolved
release data, owned by release tooling, not a host-selected runtime backend.

### 1.3 Where dependencies are created now

There is no global dependency object and no startup constructor. Instead:

- `internal/sys.Run`, `RunOut`, `Sudo`, `Has`, `PkgInstalled`, and
  `InstalledVersion` create host command dependencies on demand;
- updater files assemble pacman, yay, Flatpak, snapper, and systemd argv at the
  point of use;
- doctor reconcilers call native tools directly or through `internal/sys`;
- `internal/updater/manifest.go` exposes `PacmanInstalled` and
  `PacmanExplicit` as mutable package globals;
- doctor uses replaceable globals such as `hasPacman`, `appInstalled`,
  `installManifestPkgs`, and `removeKepler580` as test seams;
- `internal/ryotunesrelease/client.go` constructs a client whose function
  fields default to pacman, `vercmp`, sudo, filesystem, and HTTP behavior;
- `ryoku-wm/reclaim.go` has its own package-level pacman seams for installed
  state, removal simulation, and installed size.

These seams make tests possible, but they perform distributed service location.
They are not a composition root and must not become the Debian selection
mechanism.

## 2. Backend-related runtime usage

### 2.1 Update, status, rollback, and channel flows

| File and function | Current responsibility | Current Arch assumption | Future dependency |
|---|---|---|---|
| `internal/updater/update.go:Update` | Owns update lock, free-space guard, sudo lifetime, pre/post snapshots, checkout versus packaged path, stage-one/stage-two handoff, optional system lane, materialize, shell cutover, doctor, and finalization | A packaged path means pacman; system updates mean `pacman -Syu` followed by yay; stale lock is `/var/lib/pacman/db.lck`; the replacement binary is `/usr/bin/ryoku` | Receive selected identity/capabilities plus narrow `PackageTransactions`, `PackageFacts`, `RepositoryManager`, and boot/snapshot capability. Keep all sequencing in updater |
| `internal/updater/ryokuset.go:installedRyokuSet`, `repoServedSet`, `dropOlderServes`, `systemLanePending` | Builds the exact Ryoku lane, protects newer installed packages, and separates Ryoku from distribution updates | `[ryoku]`, `pacman -Slq/-Qq/-Qi/-Si/-Qu`, repo-qualified `ryoku/<name>`, and `vercmp` | `PackageFacts` for inventory, availability, pending updates, and version comparison; `RepositoryManager` for published Ryoku components |
| `internal/updater/ryokuset.go:refreshDBArgs`, `ryokuInstallArgs` | Builds refresh and exact install argv | `pacman -Sy/-Syy`, pacman overwrite glob, `SNAP_PAC_SKIP`, and repository-qualified package names | `RepositoryManager.Refresh` and `PackageTransactions` plan/apply. The Ryoku-only scope remains updater policy |
| `internal/updater/update.go:runSystemLane`, `systemUpgradeArgs`, `runAURUpgrade` | Runs the explicit `--system` lane after the Ryoku lane | Arch system upgrade is `pacman -Syu`; supplemental upgrades are yay/AUR | `PackageTransactions` with a distinct system scope and backend capability for supplemental updates. Flatpak remains its own application ecosystem, outside the native package backend |
| `internal/updater/update.go:unownedFiles`, `dropSplitMetasNotServed`, `clearStalePacmanLock`, `prowlPacmanOwned` | Recovers package conflicts, removes obsolete split metas, heals a stale lock, and distinguishes packaged tools | `pacman -Qo`, `-Rdd`, pacman process name, db lock path | `PackageFacts.OwnerOf`, reviewed `PackageTransactions`, and normalized lock/recovery facts. Raw dependency-bypassing removal must not survive in shared updater code |
| `internal/updater/update.go:Status`, `pendingUpdates`, `systemPackageUpdates`, `aurUpdates` | Builds human and JSON update status used by shell/Hub | `[ryoku]` listing, `checkupdates`/pacman-shaped rows, `yay -Qua`, and Arch-specific help text | `PackageFacts` update candidates plus repository identity/capabilities. Preserve the public `ryoku status --json` boundary |
| `internal/updater/update.go:Rollback` and `internal/updater/bootguard.go:BootGuard` | Chooses release rollback versus snapshot guidance; boot guard restores the prior packaged release after repeated failed boots | Force-refreshes pacman, reinstalls `ryoku-desktop`, assumes Limine/Snapper paths and Arch package semantics | `RepositoryManager` for the frozen channel, `PackageTransactions` for a reviewed downgrade, and `BootManager` for backend-supported boot/snapshot facts. Policy and failure counting stay here |
| `internal/updater/release.go:Track`, `switchToPackageChannel` | Interprets stable/testing/release aliases and switches packaged or source delivery | A packaged channel is the `[ryoku]` `Server` in pacman.conf; the move is a forced pacman refresh/install | `RepositoryManager.SetChannel/Refresh` plus `PackageTransactions`; alias and source-versus-packaged product policy stays here |
| `internal/sys/release.go:RyokuServer`, `PackagedChannel`, `SetPackagedChannel`, `DropRyokuSyncDB` | Parses/mutates the configured channel and invalidates cached metadata | `/etc/pacman.conf`, `[ryoku]`, `$arch`, fixed `x86_64`, and `/var/lib/pacman/sync/ryoku.*` | `RepositoryManager`. Generic atomic/root file helpers may stay in `internal/sys`, but repository format and cache paths may not |
| `internal/sys/release.go:ReadRelease` | Reads `/etc/ryoku-release` | The comments and `Version` field describe a pacman-owned marker/pkgver, though the marker shape is product data | Platform/repository identity. Marker parsing can remain shared after pacman wording and assumptions are removed |
| `internal/updater/manifest.go:PacmanInstalled`, `PacmanExplicit`, `pacmanSet` | Supplies installed/manual sets to manifest convergence and verify | `pacman -Qq/-Qqe` and native names as cross-platform identity | `PackageFacts.Inventory`; logical component mapping is required before comparisons become backend-neutral |
| `internal/updater/ryotunes.go` and `internal/ryotunesrelease/client.go` | Keeps download origin, checksum, upgrade-only policy, root-safe staging, artifact identity checks, and installation | Arch `.pkg.tar.zst`, x86_64 filename rules, pacman `-Q/-Qip/-U`, and `vercmp` | `PackageFacts` for installed version/comparison and package artifact inspection; `PackageTransactions` for reviewed local-artifact install. HTTP and checksum policy stay in the Ryotunes client |

`internal/updater/materialize.go:Materialize`, `reset.go`, `runstate.go`, and the
shell reload/cutover orchestration are CLI application behavior. They must not
move into `SystemBackend` merely because they run after package application.

### 2.2 Window-manager switching

| File and function | Current responsibility | Current Arch assumption | Future dependency |
|---|---|---|---|
| `wm.go:cmdWmUse` | Validates the requested provider, decides keep/remove behavior, installs the target, previews reclamation, confirms, switches the session, and reports outcome | Provider caps contain pacman package names; availability and installation are package-name based | Package component resolution plus narrow `PackageFacts` and `PackageTransactions`. CLI retains prompts, provider/session actions, and switch ordering |
| `wm.go:cmdWmUse` package-install block | Installs incoming compositor packages and handles an old virtual-package conflict | Direct `pacman -S`, emergency `pacman -Rdd`, and known split meta/package names | Reviewed install/remove plans through `PackageTransactions`; logical compositor components replace native names |
| `wm.go:removePreviousCompositor` | Revalidates the displayed removal and executes it after the provider switch | Builds `pacman -Rns` from the reviewed `ReclaimSet` | `PackageTransactions.ApplyRemove` on the exact revalidated plan |
| `wm.go:packageAvailable`, `packageInstalled` | Answers provider package availability/state | `pacman -Si/-Qq` | `PackageFacts.Available/Query` |
| `ryoku/wm/reclaim.go:Reclaim`, `VerifyRemoval` | Computes outgoing-only packages, asks the native solver for its removal cascade and sizes, and rejects a changed plan | Parses `pacman -Rs --print` and `pacman -Qi`; mutates package-level pacman seams | A narrow removal-plan consumer supplied by the caller. `ryoku-wm` keeps set policy but must not select a backend or import a concrete provider |

The compositor provider protocol, provider detection, capabilities unrelated to
packages, config application, session switching, and user confirmation remain
outside the system backend.

### 2.3 Doctor and verify

`internal/doctor/doctor.go:Run` owns argument semantics, read-only versus
mutating mode, ordered reconciler execution, finding severity, terminal/JSON
rendering, report generation, and exit behavior. Those are CLI-owned. Backend
facets replace native probes and mutations inside selected reconcilers; doctor
must not receive a registry or branch on a backend ID.

| Files/functions | Current Arch assumption | Future dependency | Doctor-owned behavior that remains |
|---|---|---|---|
| `doctor.go:reconcilePacmanLock`, `reconcileOrphans`, `reconcilePacnew`, `reconcileConflictingRyokuFiles` | Pacman lock/process, `pacman -Qtdq`, `.pacnew`, pacman ownership, Arch remedy text | `PackageFacts`, normalized native lock/config-conflict/diagnostic facts, and narrowly authorized repair through `PackageTransactions` | Finding policy, check-only behavior, safe-versus-manual repair decision |
| `doctor.go:reconcileRyokuChannel`, `reconcileRyokuSyncDB`; `reconcile_pacman.go` | pacman.conf stanza, Ryoku keyring package, `pacman-key`, sync db files, `pacman -Sl`, and ILoveCandy | `RepositoryManager` for configured/trusted/channel/repair state. ILoveCandy is an explicitly Arch-only cosmetic capability and should be provider-gated or omitted elsewhere | Whether a finding is fixed, warning, or advisory; one-shot migration semantics |
| `reconcile_manifest.go:reconcileManifest`, `manifestVerify`, `Verify` | Native names, pacman inventory, pacman installs, and yay/AUR lane | `PackageFacts.Inventory` and `PackageTransactions`; supplemental capability where available | Baseline semantics, respect for user removal, retired-package policy, best-effort convergence, verify output |
| `reconcile_shipped_apps.go:reconcileShippedApps` | `pacman -Qdq/-S/-D --asexplicit` and “has pacman” as platform detection | `PackageFacts` manual/installed state and `PackageTransactions`. Marking manual state may require a narrow package-state operation not present today | Deliver-once ledger and app selection |
| `reconcile_quickshell.go:reconcileQuickshell` | pacman owner/foreign queries and `pacman -S quickshell` | `PackageFacts.OwnerOf/Inventory` and `PackageTransactions` | Qt/runtime compatibility policy and shell restart guidance |
| `reconcile_multilib.go:reconcileMultilibRepo` | `[multilib]` in pacman.conf, `lib32-*`, and `pacman -Sy multilib` | `DriverManager` for 32-bit graphics requirements plus repository capability. It is not a generic RepositoryManager channel concern | Detecting that the installed workload requires 32-bit support and explaining the result |
| `reconcile_asus_aura.go`, package portions of `reconcile_hardware.go`, `reconcile_fingerprint.go`, `reconcile_qmk.go` | Arch package names, AUR actuators, NVIDIA package removal, pacman hook layout, mkinitcpio | `DriverManager` for driver/firmware recommendations and coherent apply; `PackageTransactions` for non-driver optional components | Hardware probing that is genuinely portable, consent, safety gates, and user-facing findings |
| `reconcile_limine*.go`, `reconcile_initramfs.go`, `reconcile_boot_space.go`, `reconcile_snapshots.go`, `reconcile_boot_rw.go`, `reconcile_alongside.go` | Arch kernel `pkgbase`, `/boot/vmlinuz-*`, mkinitcpio, Limine Arch paths/tools, snap-pac/Snapper integration | `BootManager`, including kernel inventory, boot plans, boot-space validation, initramfs, entries, and snapshot integration | Desired boot policy, check-only presentation, and conservative repair authorization |
| `reconcile_ryotunes.go` | `/usr/bin/ryotunes` ownership is checked with pacman and remedy text is pacman-specific | `PackageFacts.OwnerOf`; artifact update remains owned by the Ryotunes client | Shadow-copy cleanup and user-service convergence |
| `report.go:writeReport` and package/repository sections | Reads pacman.conf and pacman log; embeds raw pacman queries | Backend-neutral diagnostic facts with optional sanitized native detail | Report assembly, redaction, destination, and presentation |

Desktop/session/config reconcilers should receive no backend dependency unless
they actually need native package, repository, boot, driver, or service-map
facts. Debian support is not a reason to fork the reconciler registry.

### 2.4 Low-level package assumptions

`internal/sys/sys.go:PkgInstalled` and `InstalledVersion` are the most widely
shared hidden Arch seam. They shell out to `pacman -Q` and make callers appear
portable when they are not. They should be replaced at consumers by injected
`PackageFacts`; they should not be taught an `if debian { dpkg ... }` branch.

The generic parts of `internal/sys` remain useful and CLI-owned: process
execution, terminal wiring, filesystem existence, XDG paths, atomic writes,
and terminal styling. A backend may use its own reviewed command runner, but
`internal/sys.Run(name, args...)` is too broad to be a backend contract.

### 2.5 Delegated source and recovery commands

`track.go:cmdTrack` and `recovery.go:cmdRecovery` choose a local script when a
checkout is available or download the canonical script and execute it. The Go
wrappers do not themselves run pacman. Their direct delegates do:

- `bin/ryoku-track` installs an Arch development toolset for source tracking;
- `bin/ryoku-recovery` performs Arch base repair, queries
  `ryoku-desktop`, and restores packages.

Download/source selection and the explicit destructive recovery UX remain
CLI-owned. Native package actions eventually belong behind a development or
recovery-scoped package service. They must not be hidden by adding Debian
branches to these Go wrappers, and installer-target recovery must not be routed
through the installed-host backend.

## 3. Composition boundary analysis

### 3.1 Natural CLI composition point

`ryoku/cli/main.go:main` is the natural process-level candidate because it
already owns startup, command selection, error propagation, and the initial
runtime consumers. The future flow can occur once per process:

```text
parse enough command intent to know backend need
  -> detect installed platform once
  -> create a process-local registry
  -> explicitly register supported providers
  -> construct backend-neutral dependencies
  -> look up exactly identity.BackendID
  -> construct and consistency-check one SystemBackend
  -> pass only required facets to the selected command owner
```

The exact code location is intentionally not selected here. Two constraints
matter:

1. Help and commands such as `materialize`, `reset`, `reload`, plugin, import,
   keyboard, keyring, and security-key should not fail merely because package
   backend composition is unavailable when they do not need it.
2. A backend-dependent command must compose once before its service starts; it
   must not let updater, doctor, or WM independently detect or select a distro.

This favors a small command-aware startup boundary around dispatch, not a
package-global singleton initialized by `init`.

### 3.2 Dependencies to wire

The initial CLI wiring set is:

- a `PlatformDetector` supplied for the installed host;
- a process-local `Registry` with explicit provider registration;
- the backend factory dependencies, currently `PackageFacts`;
- the selected `SystemBackend` identity and available facets;
- narrow handoff objects/functions for updater, doctor, and WM rather than the
  registry itself;
- context/cancellation and a CLI-compatible progress/event sink when mutation
  contracts grow to support them;
- the existing privilege and terminal behavior adapted behind provider-owned
  mutations, without passing arbitrary command strings from commands.

Today only the Arch `PackageFacts` implementation is present, and
`arch.New` exposes only identity plus that facet. Transactions, repositories,
boot, and drivers are nil. Production CLI composition must therefore fail
honestly for a required missing facet; it must not silently fall back to the
current pacman path.

### 3.3 What must remain outside CLI composition

- Provider-native pacman/apt/dpkg parsing, locks, package names, trust files,
  repository formats, boot paths, and driver mappings stay in providers.
- Updater ordering, Ryoku-versus-system lane policy, stage-two handoff,
  materialize, run-state, live shell cutover, and user messages stay in updater.
- Doctor's reconciler order, idempotency, check-only semantics, finding model,
  remedies, JSON contract, and report policy stay in doctor.
- WM provider protocol, compositor config, session operations, prompts, and
  outgoing/incoming component policy stay in the WM layer.
- Source checkout Git operations, HTTP release discovery, checksum policy,
  Flatpak operations, keyring/PAM, security keys, import, plugins, and keyboard
  configuration stay with their existing owners.
- Hub and Rashin are separate processes. They must compose at their own roots
  or continue across stable CLI/IPC boundaries; they must not import CLI
  internals or reuse the CLI registry as a service locator.
- The ISO and existing-system installers need target-aware composition. Their
  target identity and authority are not the installed CLI's host identity.
- `ryoku-manifest` and repository publication need release-side resolution, not
  runtime provider selection.
- QML and `ryoku-shell` continue consuming backend-neutral command/JSON/socket
  contracts, especially `ryoku status --json`.

## 4. Migration risks and controls

| Risk | Where it exists now | Required control |
|---|---|---|
| Direct pacman calls survive behind innocent helpers | `internal/sys`, updater, doctor, WM, Ryotunes, and `ryoku-wm/reclaim.go` | The runtime exit condition is that native commands occur only in provider code; audit both direct `exec` and helper calls |
| Commands grow `if arch`/`if debian` branches | `main.go` dispatch, `Update`, doctor reconcilers, `cmdWmUse` | Select once at the root and inject semantics/capabilities. Commands branch on product intent or capability, never distro ID |
| Presence of `pacman` is mistaken for platform identity | `hasPacman`, `sys.Has("pacman")`, pacman-box early returns | Use detected/validated `PlatformIdentity`; PATH is neither selection nor support evidence |
| Ryoku update accidentally becomes a distro upgrade | `runRyokuUpgrade`, `runSystemLane`, channel switching | Preserve separate Ryoku and system transaction scopes and keep `--system` explicit |
| Backend swallows application policy | updater stage sequence, doctor registry, WM switch orchestration | Pass narrow facts/plans/actions upward; do not move sequencing, prompts, or convergence decisions into providers |
| Removal plan differs from apply | `wm.Reclaim` simulates `-Rs`, while `removePreviousCompositor` constructs `-Rns` | Carry one fingerprinted native plan through display, revalidation, and exact apply; abort on expansion or mismatch |
| Raw dependency-bypassing removal is normalized | split-meta and old WM conflict recovery use `-Rdd` | Treat these as explicit migration hazards requiring reviewed provider semantics, not generic remove behavior |
| Repository and package responsibilities stay entangled | refresh argv, pacman.conf editing, sync-db deletion, package apply | `RepositoryManager` owns source/trust/channel/metadata; package facets own inventory and transactions |
| Native output remains a control protocol | update conflict parsing, doctor error parsing, WM removal parsing, Ryotunes metadata parsing | Parse native data only inside the provider and return typed facts/plans/errors/events |
| Arch filesystem layout leaks into Debian commands | pacman db/log paths, `pkgbase`, mkinitcpio, Limine paths, pacman hooks, `.pacnew` | Move layout interpretation into Package/Repository/Boot/Driver providers; doctor renders neutral findings |
| AUR becomes a universal backend concept | update system lane, manifest AUR lane, QMK/fingerprint helpers | Model supplemental availability as a capability/source. Core delivery cannot silently become best-effort on Debian |
| Native package names remain product identity | manifests, compositor caps, quickshell, driver and optional-package reconcilers | Resolve stable logical component IDs through the selected backend before package operations |
| Composition becomes global mutable state | current package-level test seams make that tempting | Construct once per process and pass dependencies explicitly; tests inject facets or an already composed backend |
| Unsupported or incomplete backend falls through to pacman | Arch is the only working runtime path today | Missing registration, identity mismatch, or missing required facet is an actionable startup error for that command |
| Non-backend commands become unavailable | eager composition before all dispatch | Compose only for the command path that requires system lifecycle services, while still selecting at most once |
| Installer/release authority leaks into runtime | recovery, source tracking, manifest generator are adjacent to runtime code | Keep target execution, offline closure, build databases, publication, and release artifacts outside installed-host composition |
| Public consumers learn native vocabulary | status JSON and doctor JSON feed shell/Hub | Preserve stable neutral schemas; native detail may appear only as explicitly diagnostic data |

## 5. CLI-owned invariants during migration

The following behavior is not an adapter responsibility and must remain stable
while native calls move:

- one command front door and existing exit/error behavior;
- `ryoku update` stage ordering and the packaged-binary stage-two handoff;
- the separation between Ryoku updates and the opt-in system update;
- update lock, free-space guard, durable run-state, snapshot policy, shell
  cutover, materialize, doctor, and finalization order;
- source checkout behavior as a separate delivery mode;
- manifest baseline semantics, including respect for user removals;
- doctor ordering, read-only modes, idempotency, JSON findings, and report UX;
- exact user review before compositor removal;
- the `ryoku status --json` boundary consumed by the shell and Hub;
- no native package-manager concepts in QML or compositor configuration.

## 6. Suggested integration sequence

This is dependency order, not an implementation plan:

1. Compose and inject read-only identity and `PackageFacts` for status,
   manifest/verify, ownership, installed state, and availability.
2. Route repository inspection, refresh, channel changes, and repair through
   `RepositoryManager` without moving channel policy out of updater/doctor.
3. Route Ryoku, system, manifest, artifact, and WM plans through
   `PackageTransactions`, preserving exact scopes and review/revalidation.
4. Route boot-sensitive updater/doctor facts and actions through `BootManager`.
5. Route driver and 32-bit graphics package policy through `DriverManager`.
6. Remove the remaining runtime pacman/yay/native-file parsing from shared CLI
   code only after Arch behavior passes the existing command and delivery gates.

The safe endpoint is not “the CLI knows both pacman and apt.” It is “the CLI
owns workflows and consumes backend-neutral facts and plans, while exactly one
selected provider owns native system lifecycle behavior.”

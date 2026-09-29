# Arch Linux dependency map

## Purpose and scope

This document audits the repository as it exists on the
`debian-backend-audit` branch. It identifies coupling that a future Debian
backend must replace, adapt, or deliberately retain. It does not propose an
implementation and no migration change is part of this audit.

The audit covers runtime behavior, installation, release engineering, package
metadata, boot and hardware policy, tests, development tools, and user-facing
surfaces. A reference such as `path:line` points to the current implementation.

The central finding is that Ryoku's desktop is largely portable Linux software,
but its delivery and lifecycle are an Arch product. The full-system installer,
package manifests, signed repository, updater, doctor, compositor switching,
kernel and initramfs management, GPU package selection, and several Settings
features all speak pacman directly. Debian support therefore needs a backend
boundary around system lifecycle operations. Repackaging only the QML shell
would not produce a supported Debian Ryoku system.

## Executive dependency map

| Layer | Arch coupling | Debian disposition |
|---|---|---|
| Desktop UI and shared QML | Low, with a small set of package and branding exceptions | Keep most files unchanged; adapt the listed package/update surfaces |
| Shell daemon and compositor providers | Mostly portable Linux/Wayland; package operations leak into compositor switching and plugin inspection | Keep protocol and compositor logic; replace package-query/removal seams |
| Hub backend | Direct pacman and AUR calls in compositor and GPU workflows | Introduce backend operations for availability, install, remove, and package ownership |
| `ryoku` CLI | Deeply coupled to pacman databases, CLI output, package versions, hooks, lock files, `.pacnew`, AUR, and Arch boot tooling | Requires a distro backend; this is the largest runtime migration unit |
| Full-system installer | Arch-native from live medium through target install | Preserve disk/TUI concepts but provide a Debian target and live-image backend |
| Hardware policy | Detection is mostly portable; installation and initramfs integration are Arch-native | Keep sysfs/PCI detection; replace package selection, hooks, and initramfs actions |
| Release pipeline | 34 PKGBUILDs, `makepkg`, `repo-add`, pacman keyring, Arch containers, `.pkg.tar.zst` artifacts | Add a parallel Debian build/sign/repository lane; do not translate artifacts in place |
| Update and delivery contract | `[ryoku]` pacman repository is the authoritative product channel | Preserve the contract and manifests, but implement it with Debian package/repository semantics |
| Boot and snapshots | Limine plus mkinitcpio, libalpm hooks, `snap-pac`, and Arch kernel layout | Split generic Limine/Btrfs intent from Arch initramfs and hook implementation |

## 1. Package management

### 1.1 Pacman is a product-level dependency

Pacman is not confined to installation. It is part of normal desktop operation:

- `ryoku update` discovers the installed Ryoku set with `pacman -Slq`, `-Qq`,
  `-Qu`, and `vercmp`, then installs repo-qualified targets. See
  `ryoku/cli/internal/updater/ryokuset.go:34-181` and
  `ryoku/cli/internal/updater/update.go:1673-1736`.
- The CLI treats `/etc/pacman.conf` and `/var/lib/pacman/sync` as the channel
  configuration and local repository state. See
  `ryoku/cli/internal/sys/release.go:37-190`.
- Update recovery understands pacman's lock at `/var/lib/pacman/db.lck`, package
  ownership through `pacman -Qo`, sync database signature failures, file
  conflicts, and pacman-specific output. See
  `ryoku/cli/internal/updater/update.go:417-539,1076,1105` and
  `ryoku/cli/internal/updater/upgradelog.go:89-144,525-526`.
- The control manifest computes installed, explicit, missing, user-removed, and
  retired package state from `pacman -Qq` and `pacman -Qqe`. See
  `ryoku/cli/internal/updater/manifest.go:270-308`.
- `ryoku update --system` uses pacman for the distribution upgrade, `yay -Sua`
  for foreign/AUR upgrades, and `checkupdates` for the pending system view. See
  `ryoku/cli/main.go:7-102` and
  `ryoku/cli/internal/updater/update.go:378,539,1703-1736`.
- Compositor installation, removal planning, installed size calculation, and
  package availability are pacman transactions and pacman output parsers. See
  `ryoku/cli/wm.go:273-303,421-523` and `ryoku/wm/reclaim.go:25-306`.
- `ryoku doctor` installs packages, marks packages explicit, detects orphans,
  queries file ownership, repairs the `[ryoku]` repository, removes stale sync
  databases, repairs multilib, and classifies `.pacnew` files. Representative
  entry points are `ryoku/cli/internal/doctor/reconcile_manifest.go:50-62`,
  `reconcile_shipped_apps.go:72-85`, `reconcile_multilib.go:13-117`, and
  `doctor.go:1005-1114,3642-3787`.
- The Hub directly queries or invokes pacman for compositor availability and GPU
  passthrough packages. See `ryoku/hub/backend/wm.go:151-174` and
  `ryoku/hub/backend/gpuapply.go:68-95,201-310`.
- The shell's package provider asks `gpk` specifically for the `pacman,aur`
  managers. See
  `ryoku/shell/quickshell/shell/modules/launcher/shared/providers/packages/Packages.qml:203-217`.
- User-facing helper scripts are thin pacman wrappers:
  `system/extras/ryoku-pkg-add`, `ryoku-pkg-remove`,
  `ryoku-pkg-multilib`, `ryoku-pkg-cachyos`, and `ryostore-install`.
  `system/extras/README.md:23-48` states that package presence, routing, and
  transactions are decided by `pacman -Qq`, `pacman -Si`, pacman, and an AUR
  helper.
- Developer and recovery paths also assume pacman. See `bin/ryoku-track:36`,
  `bin/ryoku-recovery:60-209`, and `ryoku/shell/deploy.sh:107-120,637-710`.

This coupling cannot be handled by command-name substitution alone. The code
depends on pacman concepts with no direct one-to-one Debian equivalent:
repo-qualified targets (`ryoku/pkg`), explicit versus dependency-installed
packages, `-Rs` cascade planning, `-Rdd`, sync databases, alpm hooks, pacman
version ordering, foreign packages (`-Qm`), `.pacnew`, and transaction output.

### 1.2 Package manifests are Arch namespaces

The complete package model under `system/packages/` is Arch-authored:

- `base.packages` is passed to pacstrap and contains Arch package names for the
  base OS, kernel, boot stack, desktop, applications, fonts, containers, gaming,
  and developer-facing tools. The entire file is a migration input, not a
  distro-neutral manifest (`system/packages/base.packages:1-323`).
- `dev.packages` names Arch toolchain packages (`go`, `nodejs`, `npm`, `python`,
  `python-pip`, `python-pipx`, `mise`) and is also pacstrapped
  (`system/packages/dev.packages:1-11`).
- `hardware.packages` uses pacman sections and virtual-provider semantics for
  `amd-ucode`, `intel-ucode`, `vulkan-radeon`, `vulkan-intel`, `vulkan-swrast`,
  `nvidia-utils`, and their `lib32-*` variants
  (`system/packages/hardware.packages:1-51`).
- `aur.packages` is explicitly a list of AUR PackageBases. It currently names
  `yay-bin`, `phinger-cursors`, `catppuccin-cursors-mocha`, `apple_cursor`,
  `nvibrant-bin`, `localsend-bin`, `voxtype-bin`, `ttf-fraunces-variable`,
  `zen-browser-bin`, and `pam-fprint-grosshack`
  (`system/packages/aur.packages:1-70`).
- `cachyos.packages` names a second Arch repository layer:
  `linux-cachyos`, its headers and keyring/mirrorlists, `cachyos-settings`,
  Ananicy packages, `scx-scheds`, and `proton-cachyos-slr`
  (`system/packages/cachyos.packages:1-47`).

Every entry must be classified for Debian as one of: same-name package,
renamed package, split/merged package, Ryoku-built Debian package, optional
feature, or unavailable feature. Reusing `base.packages` directly would silently
install wrong names or skip required runtime capabilities.

### 1.3 AUR and makepkg

The AUR is used in four distinct ways:

1. Online full-system installs bootstrap `yay-bin` by cloning its AUR repository
   and running `makepkg -si`, then use `yay -S` for the AUR set
   (`installation/backend/lib/aur.sh:3-124`).
2. Offline ISO builds clone every PackageBase, run `makepkg -s`, collect runtime
   dependencies from `.PKGINFO`, and merge the resulting packages into the
   offline pacman repository (`installation/iso/offline-repo.sh:228-362`).
3. Normal system updates use `yay -Sua`, while doctor uses `yay -S` to converge
   manifest AUR entries (`ryoku/cli/internal/updater/update.go:539` and
   `ryoku/cli/internal/doctor/reconcile_manifest.go:57-62`).
4. Feature workflows install AUR-only software on demand, including GPU
   passthrough packages, dictation, VM tools, and Extras bundles. See
   `ryoku/hub/backend/gpuapply.go:68-95`,
   `ryoku/hub/quickshell/pages/DictationPage.qml:110-121`, and
   `ryoku/apps/ryovm/bin/ryovm:141-153`.

Debian has no AUR lane. Each AUR dependency therefore needs a deliberate source:
Debian archive, Ryoku Debian repository, upstream binary, source build during
release, Flatpak, or removal. Building untrusted package recipes on the installed
target should not be carried over accidentally.

### 1.4 PKGBUILD and pacman artifact assumptions

There are 34 first-party `release/packages/*/PKGBUILD` recipes. Together they
define file placement, runtime dependencies, build dependencies, conflicts,
provides, install scripts, and meta-package composition. Important coupling
includes:

- `release/packages/ryoku/PKGBUILD:16-19` hard-depends on `pacman`,
  `pacman-contrib`, and `snapper`, with `yay` optional.
- `release/packages/gpk/PKGBUILD:18-20` hard-depends on pacman.
- `release/packages/ryoku-desktop/PKGBUILD:20-237` is the umbrella dependency
  graph and optional-feature catalog; its `package()` also lays system files,
  hooks, user units, configs, and hardware helpers.
- `ryoku-desktop-hyprland` and `ryoku-desktop-niri` provide the virtual
  `ryoku-desktop-compositor` capability
  (`release/packages/ryoku-desktop-{hyprland,niri}/PKGBUILD`). Pacman's virtual
  provider and conflict behavior is part of compositor switching.
- Package install scripts such as
  `release/packages/ryoku-desktop/ryoku-desktop.install` and
  `release/packages/ryoku-keyring/ryoku-keyring.install` are alpm lifecycle
  scripts and need Debian maintainer-script equivalents where the behavior is
  still required.
- Shell stash installation recognizes `.pkg.tar.*` and `.PKGINFO`, then invokes
  `pkexec pacman -U` (`ryoku/shell/scripts/stash-install.sh:37-58,346-366`).
- Ryotunes discovery and verification encode `.pkg.tar.zst`, pacman epoch/pkgrel
  naming, `.PKGINFO`, `vercmp`, and `pacman -U` throughout
  `ryoku/cli/internal/ryotunesrelease/`.

The payload contents of many PKGBUILDs are reusable, but the recipe language and
lifecycle semantics are not. Debian packaging must reproduce ownership, modes,
triggers, conflicts, alternatives, dependencies, and upgrade behavior rather
than merely archive the same files.

## 2. Installer coupling

### 2.1 Full-system backend

The full installer is an Arch installer end to end:

- `installation/backend/ryoku-install` orchestrates an Arch sequence and calls
  pacstrap, chroot configuration, pacman repository setup, GPU package scripts,
  AUR installation, and an Arch boot finalization (`ryoku-install:8-216`).
- `installation/backend/lib/pacstrap.sh` assembles `base.packages`,
  `dev.packages`, profile hardware sections, optional CachyOS packages, and
  Broadcom DKMS, starts `pacman-init.service`, runs `pacstrap -K`, retries with
  pacman options, and writes the fstab (`pacstrap.sh:1-170`).
- All target commands use `arch-chroot /mnt`, including account/system setup,
  services, package installation, driver scripts, AUR builds, initramfs, and
  bootloader work. The densest users are `chroot.sh`, `deploy.sh`, `drivers.sh`,
  `aur.sh`, `snapshots.sh`, and `bootloader.sh`.
- `mirrors.sh` ranks Arch mirrors with reflector and the Arch mirror-status API,
  writes `/etc/pacman.d/mirrorlist`, and tunes `/etc/pacman.conf`
  (`installation/backend/lib/mirrors.sh:1-175`).
- `deploy.sh` appends the `[ryoku]` stanza, copies the live mirrorlist, installs
  the Ryoku keyring, runs `pacman-key --populate ryoku`, installs
  `ryoku-keyring` and `ryoku-desktop`, and then materializes config
  (`installation/backend/lib/deploy.sh:46-113`).
- `offline.sh` constructs an `[offline]` pacman configuration at
  `/etc/pacman.d/ryoku-offline.conf`, uses a file repository, manipulates target
  pacman sync state, and funnels all offline installs through pacman
  (`installation/backend/lib/offline.sh:89-160`).
- `cachyos.sh` edits pacman architecture and repository stanzas and populates the
  CachyOS keyring (`installation/backend/lib/cachyos.sh:10-110`).

The following installer concerns are largely distro-neutral and can remain as
shared policy after their command execution is abstracted: TUI navigation and
input contract, hardware probing, UEFI/Secure Boot checks, disk discovery,
partitioning, LUKS, Btrfs/ext4 creation, alongside-install geometry, locale and
timezone intent, user creation intent, hostname, and generated fstab/crypttab
data. The current shell implementation often combines those concerns with
`arch-chroot`, so reuse may be conceptual rather than file-for-file.

### 2.2 Archiso live image

The live image is an archiso profile, not a generic ISO definition:

- `installation/iso/profiledef.sh:4-34` uses archiso variables, an `arch`
  install directory, `bios.syslinux` and `uefi.systemd-boot` boot modes, an
  archiso pacman config, and a squashfs airootfs.
- `packages.x86_64` includes `base`, `linux`, `linux-firmware`, `mkinitcpio`,
  `mkinitcpio-archiso`, both ucode packages, `arch-install-scripts`, reflector,
  and Arch package names for the live toolchain
  (`installation/iso/packages.x86_64:1-93`).
- The committed airootfs contains archiso mkinitcpio hooks and presets, a
  temporary pacman keyring mount, `pacman-init.service`, and Archiso kernel
  command-line options. See `installation/iso/airootfs/etc/mkinitcpio*`,
  `installation/iso/airootfs/etc/systemd/system/*pacman*`, and
  `installation/iso/efiboot/loader/entries/*.conf`.
- `build.sh` stages the archiso profile, wires a local `[ryoku]` pacman
  repository, optionally pins Arch Linux Archive snapshots, prebuilds Go/QML
  artifacts, bakes an offline pacman closure, and invokes `mkarchiso`
  (`installation/iso/build.sh:4-153,155-238,255-289`).
- `offline-repo.sh` uses isolated pacman databases and caches, downloads a full
  Arch dependency closure, builds AUR packages with makepkg, validates
  `.pkg.tar.*` contents, generates the database with `repo-add`, and simulates
  the pacstrap transaction (`installation/iso/offline-repo.sh:10-525`).

None of `archiso`, `pacstrap`, `arch-chroot`, or `mkarchiso` should be retained in
a Debian image backend. The useful shared assets are the installer binary,
translations, brand assets, kiosk concept, staged repository snapshot, and the
high-level offline-install requirement.

### 2.3 Existing Debian-aware precedent

`ryoku-shell-installer/` already contains a limited distro abstraction for
converting an existing system into a source-deployed Ryoku shell:

- `distro.go:10-35` centralizes package manager argv and package renames.
- `distro.go:37-56` defines pacman and `apt-get`/`dpkg-query` implementations.
- `distro.go:63-109` maps a subset of Arch package names to Debian names and
  skips known absences.
- `detect.go:200-212` selects the backend from `/etc/os-release`.

This is useful evidence that the desktop can run on Debian, but it is not the
future full-system backend. Debian is marked `fromSource`, has no signed Ryoku
Debian repository, skips boot/update features, and parts of detection and engine
logic remain pacman-specific (`detect.go:25-50,261-303` and
`engine.go:747,928,1337-1340`). Its rename table is an audit seed, not a verified
complete package map for a reproducible installed system.

## 3. Release pipeline

### 3.1 Package build and repository generation

`release/repo/build-repo.sh` is entirely pacman repository tooling:

- It discovers `release/packages/*/PKGBUILD` (`build-repo.sh:59`).
- It requires `makepkg`, `repo-add`, and GnuPG (`build-repo.sh:48-56`).
- It builds with `makepkg --force --clean --nodeps --noconfirm --sign`, sets
  `PKGEXT=.pkg.tar.zst`, and reads `makepkg --packagelist`
  (`build-repo.sh:69-107,136-142`).
- It verifies or creates detached signatures for every package
  (`build-repo.sh:156-210`).
- It runs `repo-add -s -k`, verifies the signed `ryoku.db`, and materializes
  symlinked database names for object storage (`build-repo.sh:213-231`).
- It generates `release.json` and a distro-independent-looking
  `manifest.json`, but that manifest currently enumerates Arch package lanes
  (`build-repo.sh:233-253`, `ryoku/cli/cmd/ryoku-manifest/main.go:51-74`).

`release/repo/build-toolchain.packages` is itself an Arch package list and
assumes `base-devel` provides makepkg/fakeroot. It defines all build-time Arch
names needed because builds use `--nodeps` (`build-toolchain.packages:1-84`).

### 3.2 Signing and trust

Cryptographic intent is reusable, but its trust integration is Arch-specific:

- `ryoku-keyring` packages `ryoku.gpg`, `ryoku-trusted`, and `ryoku-revoked` for
  pacman's keyring (`release/packages/ryoku-keyring/`).
- Installation imports that key into `/etc/pacman.d/gnupg` and calls
  `pacman-key --populate ryoku` (`installation/backend/lib/deploy.sh:82-113`).
- Pacman configuration requires signed packages and databases in the installed
  system. The live ISO temporarily uses `SigLevel = Never` only for its staged
  local `[ryoku]` source (`installation/iso/pacman.conf:23-39`).
- The package repository signs individual `.pkg.tar.zst` files and the pacman
  database. The ISO is separately detached-signed with GPG
  (`.github/workflows/build-iso-reusable.yml:377-402`).

A Debian lane may retain the release identity, GPG secret handling, detached ISO
signature, immutable-artifact promotion, and verify-before-publish rule. It
must use Debian archive metadata and apt trust instead of pacman keyring files
and signed `ryoku.db`.

### 3.3 CI and publication

The main package workflow explicitly runs inside `archlinux:latest` because
pacman, makepkg, and repo-add are required
(`.github/workflows/publish-repo.yml:64-71`). It:

1. installs build dependencies with pacman;
2. imports the release private key into an ephemeral GnuPG home;
3. builds and signs the pacman repository;
4. stores the exact repository as a workflow artifact;
5. tests it in Arch and CachyOS containers with populated pacman keyrings;
6. uploads packages first and pacman databases/signatures last with rclone.

See `.github/workflows/publish-repo.yml:100-249,259-362`.

Other Arch-bound workflow lanes include:

- ISO build: `.github/workflows/build-iso-reusable.yml:149-226`.
- Install tests: `.github/workflows/install-test.yml:34-141`.
- Ryotunes repository refresh: `.github/workflows/refresh-ryotunes.yml:3-43`.
- Release ledger environment: `.github/workflows/release-ledger.yml:21-30`.
- PKGBUILD dependency verification in `.githooks/pre-push:93-107` and
  `bin/ryoku-dev-verify-pkgbuild-deps`.
- Package delivery checks that inspect PKGBUILDs in
  `bin/ryoku-dev-verify-delivery` and `.githooks/pre-commit`.

Generic workflow behavior can stay: version/codename calculation, changelog and
release-note harvesting, artifact promotion, R2 transport, checksums, manifests,
and the rule that tested bytes are published bytes.

## 4. System assumptions

### 4.1 Arch-specific filesystem and database paths

| Path or shape | Consumers | Why it is coupled |
|---|---|---|
| `/etc/pacman.conf` | installer, updater, doctor, Extras, deploy/recovery | Repository, channel, multilib, CachyOS, and UI defaults are edited in pacman syntax |
| `/etc/pacman.d/mirrorlist` | ISO, mirror ranking, target deployment | Arch mirror discovery and `Include` syntax |
| `/etc/pacman.d/gnupg` | installer, ISO, package tests | Pacman keyring layout and `pacman-key` lifecycle |
| `/etc/pacman.d/hooks` | installer, NVIDIA, Limine | Local libalpm hook masks and triggers |
| `/usr/share/libalpm/hooks` | `ryoku-desktop` package, shell-installer cleanup | Packaged libalpm hook location |
| `/var/lib/pacman/sync` | updater, doctor, dependency verifier | Pacman sync databases and `ryoku.db` cache |
| `/var/lib/pacman/db.lck` | updater, doctor, power cutover | Pacman transaction lock semantics |
| `/var/cache/pacman/pkg` | Btrfs layout, installer, ISO tests | Pacman cache receives its own subvolume and cleanup rules |
| `/var/log/pacman.log` | Profile install date and particle stream | Pacman log format and location (`ProfilePage.qml:47`, `ParticleStream.qml:1030-1034`) |
| `*.pacnew` / `*.pacsave` | updater renderer and doctor | Pacman config-upgrade artifacts |
| `*.pkg.tar.zst`, `.PKGINFO` | release, updater, Ryotunes, stash installer, tests | Arch package format and metadata |
| `/usr/lib/modules/*/pkgbase` | driver selection and boot doctor | Arch kernel package metadata file (`nvidia.sh:156`, `reconcile_limine_images.go:46`) |
| `/boot/vmlinuz-<pkgbase>` and `/boot/initramfs-*.img` | Limine generation, keyboard/initramfs checks | Arch kernel/image naming (`reconcile_limine_images.go:179-184`) |

Paths such as `/etc`, `/usr/bin`, `/usr/share`, `/usr/lib/systemd`,
`~/.config`, `~/.local/share`, `/sys`, `/proc`, and `/dev` are normal Linux/FHS
or systemd interfaces and are not inherently Arch-specific. They still need
package-ownership review because Debian may install the same component under a
different multiarch or policy path.

### 4.2 Kernel, initramfs, and boot chain

The default system assumes Arch's `linux` and `linux-headers` packages plus
`mkinitcpio` (`system/packages/base.packages:12-22`). The optional CachyOS lane
adds `linux-cachyos` and `linux-cachyos-headers`
(`system/packages/cachyos.packages:11-15`).

Arch-specific behavior includes:

- mkinitcpio `HOOKS` syntax and the custom `ryoku-gpu-trim` install hook
  (`system/boot/mkinitcpio/ryoku.conf:1-22` and
  `system/boot/mkinitcpio/install/ryoku-gpu-trim`).
- Masking/restoring mkinitcpio and Limine libalpm hooks during installation
  (`installation/backend/lib/chroot.sh:183-234`).
- Choosing `limine-mkinitcpio` or `/usr/bin/mkinitcpio -P` and expecting Arch
  kernel image names (`installation/backend/lib/bootloader.sh:41-53,519-522`).
- CLI keyboard changes that rebuild with `mkinitcpio -P`
  (`ryoku/cli/internal/keyboard/keyboard.go:269-272`).
- Doctor logic for Arch `pkgbase`, mkinitcpio drop-ins, initramfs size, Limine
  image reconciliation, and CachyOS default selection under
  `ryoku/cli/internal/doctor/reconcile_{initramfs,limine,limine_images,boot_space}.go`.
- `snap-pac` pre/post transaction snapshots and `limine-snapper-sync`, both
  tied to pacman transaction and Arch package delivery
  (`installation/backend/lib/snapshots.sh:3-96`).

Limine, UEFI NVRAM handling, Btrfs snapshots, Plymouth, and the desired rollback
experience are not intrinsically Arch-only. Their current glue is. A Debian
backend must select Debian kernel meta-packages, initramfs tooling and hooks,
kernel discovery, package triggers, and boot-image naming as one coherent unit.

### 4.3 Services and daemon names

The installer enables these exact system units in one Arch target command:
`sddm.service`, `NetworkManager.service`, `bluetooth.service`,
`rtkit-daemon.service`, and `power-profiles-daemon.service`
(`installation/backend/lib/bootloader.sh:41`). It also enables
`systemd-timesyncd.service`, `snapper-cleanup.timer`, and optionally
`limine-snapper-sync.service` (`chroot.sh:99`, `snapshots.sh:80-94`).

The `ryoku-desktop` alpm install script repeats first-install convergence for
Bluetooth, rtkit, power-profiles-daemon, and global Ryoku user units
(`release/packages/ryoku-desktop/ryoku-desktop.install:21-64,151-214`).

Most service names are upstream systemd conventions and may match Debian, but
availability, enablement defaults, aliases, and package ownership must be
verified. `rtkit-daemon.service`, SDDM configuration, NetworkManager versus iwd,
and Debian's display-manager integration deserve explicit package tests. Ryoku's
own user and system units can stay if installed to Debian policy-compliant paths.

### 4.4 User-facing Arch identity

Arch is visible outside backend code and must be treated as product copy, not
only plumbing:

- Welcome text calls Ryoku an Arch desktop
  (`ryoku/shell/quickshell/welcome/Welcome.qml:26` and `StepWelcome.qml:17`).
- Updates instruct users to run `sudo pacman -Syu`
  (`ryoku/hub/quickshell/pages/UpdatesPage.qml:559-595`).
- Profile statistics display explicit/AUR/total package counts and use
  `/var/log/pacman.log` as an install-date fallback
  (`ProfilePage.qml:47,763`).
- Fastfetch fixtures and generated/readout content use an Arch tagline.
- The launcher exposes an AUR web engine
  (`ryoku/shell/quickshell/shell/modules/launcher/shared/providers/web/engines.js:11`).
- Rashin system knowledge and quick tools describe/query pacman and yay
  (`ryoku/rashin/backend/index.go:158,260,438-535` and
  `quicktools.go:108-110`).
- Recovery messages, documentation, translations, tests, and help strings
  contain pacman/mkinitcpio/AUR remediation commands. Translation catalog hits
  are generated mirrors of source strings and should follow source changes, not
  be edited as an independent backend.

The decorative workspace style named `pacman` and `PacmanMarker.qml` are game
references, not package-manager dependencies. They can remain unchanged.

## 5. Hardware coupling

### 5.1 Portable detection and policy

The following mechanisms are Linux interfaces and can generally stay:

- GPU enumeration through `/sys/class/drm`, loaded driver names, PCI IDs, and
  `lspci` fallback in `system/hardware/gpu/ryoku-gpu-detect` and the vendor
  scripts.
- Runtime GPU mode, DRM environment, MUX probing, backlight probing, udev events,
  power sysfs, ACPI platform profiles, and NVIDIA runtime information under
  `system/hardware/gpu/`, `display/`, and `power/`.
- TUI hardware detection of CPU, GPU, memory, disks, UEFI, and Secure Boot in
  `installation/tui/system.go:740-851`.
- Udev, modprobe, sysctl, NetworkManager, PipeWire, BlueZ, and systemd policy,
  subject to packaging/path validation.

### 5.2 GPU driver installation

Driver selection is portable in intent but Arch-native in execution:

- AMD installs `mesa vulkan-radeon` through pacman
  (`system/hardware/drivers/amd.sh:36-69`).
- Intel installs `intel-media-driver vpl-gpu-rt vulkan-intel sof-firmware`
  through pacman (`intel.sh:38-73`).
- The generic Vulkan step installs `vulkan-icd-loader` (`vulkan.sh:38-65`).
- NVIDIA classifies GPU generations, selects Arch package branches such as
  `nvidia-open`, `nvidia-open-dkms`, `nvidia-utils`, and AUR legacy
  `nvidia-580xx-*`/`nvidia-470xx-*`, checks installed kernels via Arch `pkgbase`,
  writes mkinitcpio config, enables NVIDIA systemd units, and installs a libalpm
  hook (`system/hardware/drivers/nvidia.sh:5-276`).
- `ryoku-nvidia-guard` queries pacman package names and runs
  `limine-mkinitcpio`, `mkinitcpio`, or dracut after pacman path/package triggers
  (`system/hardware/drivers/ryoku-nvidia-guard:31-134`). Its dracut fallback is a
  useful portability seam, but the trigger and package detection are still Arch.
- 32-bit gaming drivers assume pacman's `[multilib]` repository and Arch
  `lib32-*` packages, then run `pacman -Syu`
  (`system/hardware/gpu/ryoku-gpu-lib32:54-123`). Debian needs multiarch
  (`i386`) semantics instead.
- The installer copies and executes all four vendor scripts inside
  `arch-chroot`, optionally against the baked offline pacman config
  (`installation/backend/lib/drivers.sh:1-29`).
- Hub GPU passthrough installs Arch repository and AUR packages directly
  (`ryoku/hub/backend/gpuapply.go:40-95,201-310`).

A Debian backend should retain detection results and desired capabilities, but
must own the package map, non-free repository policy, kernel-module packaging,
Secure Boot/DKMS implications, multiarch setup, initramfs rebuild, and package
trigger integration.

### 5.3 Firmware handling

Firmware assumptions are encoded as Arch packages:

- Every target receives `linux-firmware` (`base.packages:12-13`). The comments
  assume Arch's split firmware dependency graph pulls Intel, Realtek, MediaTek,
  Atheros, and Broadcom subsets (`base.packages:224-230`).
- CPU microcode is selected as `amd-ucode` or `intel-ucode`, and mkinitcpio's
  `microcode` hook filters it (`hardware.packages:21-29,35-45`).
- Intel audio adds `sof-firmware` at driver-install time (`intel.sh:71-73`).
- Broadcom Bluetooth patchram is a first-party Arch package,
  `broadcom-bt-firmware`, that repacks upstream Debian payload files into
  `/usr/lib/firmware/brcm` (`release/packages/broadcom-bt-firmware/PKGBUILD:36-60`).
- Broadcom Wi-Fi detection adds `broadcom-wl-dkms` to pacstrap
  (`installation/backend/lib/pacstrap.sh:81-89`).
- NVIDIA initramfs trimming relies on Arch firmware contents and mkinitcpio hook
  order (`system/boot/mkinitcpio/ryoku.conf:7-22`).

Debian firmware availability varies by release and enabled archive components.
The backend must explicitly decide `main` versus non-free firmware policy,
microcode packages, Broadcom Wi-Fi/BT sources, SOF firmware, and whether custom
repacking is still necessary. Firmware should not be inferred from Arch's
`linux-firmware` dependency graph.

## 6. What can stay unchanged

### 6.1 QML and Quickshell

Most QML is distro-neutral. It talks to Quickshell APIs, D-Bus services,
`ryoku-shell`, `ryoku-hub`, compositor providers, and Ryoku JSON state. Shared
visual components, design tokens, animations, bar/dock/launcher layout,
wallpaper UI, notifications, OSD, capture UI, lockscreen presentation, and the
`Ryoku.Ui` module can stay unchanged.

The audit found 845 QML files. Only a small set contains meaningful package or
Arch lifecycle coupling. Adapt these surfaces or the backend they call:

- `ryoku/hub/quickshell/pages/UpdatesPage.qml` and
  `Singletons/Updates.qml`: pacman-oriented system updates and copy.
- `ryoku/hub/quickshell/pages/ProfilePage.qml`: pacman log and AUR statistics.
- `ryoku/hub/quickshell/pages/DictationPage.qml`: GPK AUR install.
- `ryoku/hub/quickshell/CompositorSwitchSheet.qml`: presentation text assumes a
  pacman transaction, although it correctly delegates execution to the CLI.
- `ryoku/shell/quickshell/shell/modules/launcher/shared/providers/packages/Packages.qml`
  and `gpk.js`: GPK backend and explicit `pacman,aur` manager filter.
- `ryoku/shell/quickshell/shell/modules/bar/barstyles/qsbar/modules/ParticleStream.qml`:
  tails `/var/log/pacman.log`.
- Welcome and selected informational copy that says Arch.

`PacmanMarker.qml`, the `pacman` workspace animation, and unrelated strings such
as geometry `Arch` are not backend coupling.

Quickshell itself is a runtime dependency, not an Arch-specific architecture.
The Debian question is how to build and package a compatible Quickshell version.
The shell's QML imports and process model do not need a redesign.

### 6.2 Go services

The core service architecture can stay:

- `ryoku/shell/ipc`: supervision, socket protocol, wallpaper, clipboard, lock,
  keyring prompter, and shell actions are overwhelmingly distro-neutral. The
  only direct Arch mention in non-test Go is descriptive AUR copy for Voxtype
  (`actions.go:393`).
- Most of `ryoku/hub/backend`: settings persistence, hardware inspection,
  OpenRGB, display/input data, and provider integration are reusable. Exceptions
  are `wm.go`, `gpuapply.go`, package hints in `hwcaps.go`, and package aliases in
  `apps.go`.
- Most of `ryoku/wm`: compositor detection, actions, output state, schema,
  config generation, watching, and session environment are reusable. Exceptions
  are `reclaim.go`, package fields in `caps.go`, pacman availability/version
  checks, and package-driven Hyprland plugin tiers.
- `ryoku/rashin/backend` can run on Debian, but its system index, quick tools,
  setup hints, and danger vocabulary currently expose pacman/yay/mkinitcpio.

The `ryoku` CLI is also Go, but it cannot stay unchanged. Its materializer,
socket/client behavior, config overlay, i18n, and much of its run-state UI are
reusable; its updater, package manifest reconciliation, package-based channel
tracking, rollback guidance, doctor package checks, boot reconcilers, and
compositor package switching require a distro backend.

### 6.3 Desktop shell and configs

These areas are expected to remain substantially unchanged:

- `ryoku/ui/` shared QML module and shaders.
- Most of `ryoku/shell/quickshell/` and `ryoku/lockscreen/qylock/`.
- `ryoku/hyprland/` Lua and `ryoku/niri/` KDL, provided Debian packages compatible
  compositor/runtime versions and all invoked helper binaries.
- `ryoku/wm/` provider contract and most provider behavior.
- Application configs under `ryoku/apps/`, except installer/package hints and
  helpers that invoke pacman or AUR tools.
- Assets, wallpapers, translations infrastructure, theme templates, systemd user
  units, and Go IPC protocols.
- Generic hardware policy operating through sysfs, D-Bus, udev, NetworkManager,
  PipeWire, BlueZ, logind, and systemd.

Unchanged source still needs Debian packaging and dependency verification.
"Can stay unchanged" means no distro logic is present in the component, not that
it will work if its executable, QML module, icon theme, portal, codec, helper, or
service dependency is absent.

## 7. Required backend boundaries exposed by the audit

This is a dependency decomposition, not an implementation plan. A future
backend needs explicit ownership of at least these operations:

1. Package query: installed, version, architecture, explicit/manual state,
   ownership, available version, repository/source, orphan state, and installed
   size.
2. Package transaction: refresh, install, upgrade selected Ryoku packages,
   full-system upgrade, remove with reviewed dependency plan, download-only,
   local-file install, and lock handling.
3. Repository/channel: configure trust, configure stable/testing/frozen source,
   fetch release metadata, identify packages served by Ryoku, and recover stale
   metadata.
4. Manifest convergence: map distro-neutral capabilities to distro package
   names without carrying Arch's AUR lane into Debian by default.
5. Package lifecycle: pre/post transaction cutover, config-file policy,
   ownership adoption, service enablement, and boot/initramfs triggers.
6. Kernel and boot: enumerate installed kernels, locate images/modules, rebuild
   initramfs, integrate Limine, choose default, and validate boot-space safety.
7. Hardware packages: GPU generation to package set, firmware/microcode, DKMS,
   multiarch graphics, and Secure Boot behavior.
8. Installer target: bootstrap base filesystem, execute target commands, select
   mirrors/archive components, install the desktop meta-package, and support an
   offline closure.
9. Release artifacts: build native packages, sign repository metadata, generate
   indexes, test install/upgrade/remove, and publish atomically.

The existing delivery contract should remain the invariant above both backends:
the repository is the source of truth, user edits survive, shipped config is
materialized deterministically, every package/config reaches users, updates
converge old installations, and the bytes tested are the bytes published.

## 8. Coupling inventory by repository area

This inventory is intended to prevent less-obvious Arch dependencies from being
missed when the backend work is scoped.

| Area | Arch-coupled files or groups | Coupling |
|---|---|---|
| Package definitions | `system/packages/*.packages` | Arch names, repos, virtual providers, AUR, CachyOS, multilib |
| First-party packaging | all `release/packages/*/PKGBUILD`; `*.install`; `ryoku-keyring/*` | makepkg recipe, alpm lifecycle, pacman dependencies/trust |
| Repository tooling | `release/repo/build-repo.sh`, `build-toolchain.packages`, `import-ryotunes.sh`, `refresh-ryotunes.sh` | `.pkg.tar.zst`, `.PKGINFO`, makepkg, repo-add, pacman versions, GPG pacman repo |
| Full installer | `installation/backend/ryoku-install`; `lib/{pacstrap,mirrors,chroot,deploy,offline,cachyos,aur,drivers,bootloader,snapshots}.sh` | pacstrap, arch-chroot, pacman, Arch mirrors, AUR, mkinitcpio, libalpm hooks |
| Live image | `installation/iso/**` | archiso profile, pacman config/keyring, Arch packages, mkarchiso, offline pacman closure |
| CLI package lifecycle | `ryoku/cli/internal/{updater,doctor,ryotunesrelease,sys}/`; `ryoku/cli/wm.go` | pacman state/transactions/output, yay, checkupdates, vercmp, `.pacnew`, Arch boot layout |
| WM package lifecycle | `ryoku/wm/reclaim.go`, `caps.go`, compositor `caps.go`, `hyprland/config_plugins_tier.go` | pacman package sets, removal solver, installed sizes and versions |
| Hub | `ryoku/hub/backend/{wm,gpuapply,hwcaps,apps}.go`; package/update/profile/dictation QML | pacman/AUR installs and user guidance |
| Shell | `ryoku/shell/deploy.sh`, `scripts/{stash-install.sh,ryoku-sysinfo,ryoku-profile-stats,ryostage}`, package provider QML/JS, update particle source | pacman repo setup, package counts/files, AUR/GPK, pacman log |
| Extras | `system/extras/ryoku-pkg-*`, `ryostore-install` | official-vs-AUR routing, multilib, CachyOS repo/keyring, pacman install/remove |
| GPU drivers | `system/hardware/drivers/*.sh`, `gpu/ryoku-gpu-lib32` | Arch package names, pacman, AUR legacy branches, multilib, mkinitcpio/alpm hooks |
| Boot | `system/boot/mkinitcpio/**`, Limine package integration, related doctor files | mkinitcpio hook language, Arch kernel metadata/names, pacman hooks |
| Package hooks | `system/hardware/power/94-*.hook`, `95-*.hook`; packaged destinations in `ryoku-desktop/PKGBUILD` | libalpm trigger format and transaction timing |
| Recovery/development | `bin/ryoku-recovery`, `bin/ryoku-track`, `bin/ryoku-dev-verify-pkgbuild-deps`, delivery hook checks | pacman install/channel detection, Arch build dependencies, PKGBUILD validation |
| Standalone installer | `ryoku-shell-installer/` | partial Debian seam exists, but remaining pacman/AUR conversion logic and source-only Debian delivery |
| CI | `publish-repo.yml`, `build-iso-reusable.yml`, `install-test.yml`, `release-ledger.yml`, `refresh-ryotunes.yml`, relevant package/install test workflows | Arch containers, pacman setup, makepkg/repo-add/mkarchiso, pacman trust and artifacts |
| Tests | `installation/tests/`, package/installer scripts under `tests/`, updater/doctor/WM package tests | Pacman transcripts, Arch package fixtures, `.PKGINFO`, hook paths, repo layout |
| Documentation and i18n | README/guides/help strings and generated `ryoku/i18n/catalog/*` | Arch product identity and remediation commands; update source strings then regenerate |

## 9. Migration risk summary

The highest-risk coupling is transaction semantics, not package-name mapping.
The current system relies on pacman for atomic dependency resolution, removal
planning, package ownership, version ordering, hook execution, config-file
preservation, signed repository verification, and a stable machine-readable-enough
CLI. Those behaviors participate in live shell cutover and boot safety.

The second risk is kernel/driver coherence. NVIDIA selection, DKMS, firmware,
initramfs contents, Limine entries, snapshots, and package hooks form one chain.
A mixed implementation that changes package installation but retains Arch
kernel discovery or hooks can produce an unbootable or login-looping system.

The lowest-risk area is the visible desktop. QML, Quickshell composition, theme
logic, shell IPC, assets, and compositor configuration are mostly independent of
the base distribution. Keeping that layer stable while replacing lifecycle
backends is consistent with the repository's existing architecture and with the
partial distro abstraction already demonstrated by `ryoku-shell-installer`.

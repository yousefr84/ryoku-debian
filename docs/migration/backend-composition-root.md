# Backend composition root

## Status and intent

This document defines the responsibilities and boundaries of a future runtime
composition root for `SystemBackend`. It connects the existing platform
detection, registry, provider registration, dependency, and factory contracts
without prescribing source changes or choosing a package location.

The composition root is the process-level boundary at which concrete backend
providers may be named. Consumers below that boundary receive backend-neutral
contracts. They do not detect a distribution, look up a provider, or branch on
Arch and Debian themselves.

## Current state

The backend package already contains the individual pieces needed for
composition, but no production entry point currently joins them. Registry
lookup and Arch registration are exercised by backend tests; installed runtime
consumers still follow their existing Arch-native paths. This document does not
change that behavior.

### `PlatformDetector`

`backend.PlatformDetector` is the detection contract. Its one responsibility is
to inspect platform evidence and return a `PlatformIdentity` or an error. The
current `arch.PlatformDetector` implements that contract by reading
`/etc/os-release` and accepting Arch.

A detector owns interpretation of detection inputs. It must not initialize a
registry, register a provider, create backend dependencies, look up a factory,
or construct a `SystemBackend`. Detection failure or an unsupported platform is
an explicit result, not permission to fall back to Arch.

### `PlatformIdentity`

`PlatformIdentity` is the backend-neutral record carried from detection into
selection. It has fields for the backend ID, distribution identity, native and
repository architectures, installed backend marker agreement, release channel,
and installation mode.

The identity owns facts, not behavior. It must not read the host, decide which
provider packages to register, contain package-manager policy, create
dependencies, or construct services. Its `BackendID` is the registry lookup
key; the other fields remain independent facts and must not be inferred from
that key.

### `Registry`

`Registry` stores `BackendFactory` values by `BackendID`. Its zero value is
usable. Registration refuses to replace an existing factory, and lookup reports
an unregistered ID explicitly.

The registry owns the ID-to-factory mapping and its uniqueness rules. It must
not import or name Arch, Debian, or any future provider; discover providers;
perform platform detection; create dependencies; invoke factories; or choose a
fallback. The composition root decides which provider registration functions
to call and which detected ID to look up.

### `BackendFactory`

`BackendFactory` is the construction boundary stored in the registry. It
accepts `BackendDependencies` and returns a `SystemBackend`. The current shared
dependency set contains `PackageFacts`; it can evolve as backend-neutral
services become available.

A factory owns provider-specific assembly of the backend aggregate from
injected dependencies. It must not detect the platform, read process-global
state to replace missing dependencies, register itself, select a different
provider, or orchestrate application startup. The aggregate it returns must be
consistent with the selected identity and expose only capabilities that the
provider actually implements.

### `arch.Register`

`arch.Register` is the Arch provider's registration hook. It associates
`BackendArch` with a factory and adapts `BackendDependencies` to `arch.New`.

It owns only the Arch registration declaration. It must not create the
registry, perform detection, choose Arch, build shared dependencies, call the
factory, or change registry duplicate handling. Calling it is an explicit
composition-root decision; provider packages must not rely on import-time side
effects.

### `arch.New`

`arch.New` is the Arch provider constructor. Today it builds a `SystemBackend`
with Arch identity and the injected `PackageFacts` implementation; unsupported
facets remain absent.

It owns assembly of the Arch aggregate and its Arch-specific implementations.
It must not detect the host, initialize or query the registry, create shared
process dependencies, select another backend, or coordinate consumers. Adding
facets later must preserve the distinction between provider construction and
process orchestration.

## Future composition flow

Each process that directly needs a `SystemBackend` has one composition flow:

```text
PlatformDetector
        |
        v
PlatformIdentity
        |
        v
Registry initialization
        |
        v
Backend registration
        |
        v
Dependency wiring
        |
        v
BackendFactory lookup
        |
        v
SystemBackend
```

The stages have the following meanings:

1. The composition root invokes its supplied `PlatformDetector` once and stops
   on an error, unsupported platform, or invalid identity.
2. The resulting `PlatformIdentity` is the authoritative selection input. Its
   `BackendID` identifies the requested factory; no later probe may silently
   replace it.
3. The composition root creates an empty `Registry` for that process.
4. It explicitly invokes the supported provider registration hooks. Registering
   a provider makes it available; it does not select or construct it.
5. It creates the backend-neutral dependencies required by factories. Native
   package-manager implementations remain provider-owned and are not
   reimplemented in the root.
6. It looks up exactly the factory named by `PlatformIdentity.BackendID`.
   Missing registration is a startup error, not a fallback opportunity.
7. It invokes the factory with the wired dependencies and obtains one
   `SystemBackend`. The constructed aggregate must agree with the detected
   identity; disagreement fails composition.
8. It passes the aggregate or the narrow facets needed by each consumer. The
   consumers do not retain the registry and do not repeat selection.

This order separates availability from selection: all supported providers may
be registered, while only the identity returned by detection is constructed.
It also keeps testing explicit: a test may inject a detector, dependencies, or
an already composed backend without probing the test host.

## Ownership rules

- The detector reports platform facts. It does not construct backends.
- The registry stores factories by neutral ID. It does not know backend
  implementations or decide which backend is active.
- Backend providers register and construct their own implementation. They do
  not perform detection or application startup orchestration.
- Factories consume injected dependencies. They do not discover substitutes
  from global state or create shared application infrastructure.
- The composition root owns orchestration only: detection invocation, registry
  lifetime, explicit registrations, dependency wiring, exact lookup,
  construction, consistency checks, and handoff.
- Application services retain workflow policy, sequencing, presentation, and
  convergence decisions. They consume `SystemBackend` facets and never use the
  registry as a service locator.
- Backend implementations retain native lifecycle semantics. The composition
  root never contains pacman, apt, repository, boot, driver, or installer
  policy.

## Candidate locations

No implementation location is selected by this design. The eventual choice
must account for every process that needs direct backend access and avoid
copying selection policy.

### `ryoku/cli`

The CLI is close to the largest initial set of runtime consumers: updater,
doctor, and window-manager commands. Its main entry point already owns CLI
startup and can naturally compose once before command dispatch.

The limitation is scope. Hub, Rashin, and other binaries have separate process
roots, while installers have a different target-oriented lifecycle. Locating
all composition in the CLI could encourage other processes to duplicate it,
import CLI internals, or shell out merely to obtain an in-process backend. This
location is strongest if only the CLI directly consumes `SystemBackend` and
other processes remain behind stable CLI or IPC boundaries.

### Installer layer

The installer already owns orchestration for creating a target system and must
eventually choose target-specific lifecycle behavior. It is therefore a
plausible root for installer backend composition, especially where live-image
identity and target identity must remain distinct.

It is not a natural owner for installed runtime composition. Installer
detection answers questions about a target or installation plan, while runtime
detection answers questions about the current installed system. Making the
installer layer shared runtime infrastructure would blur those authorities and
risk coupling normal commands to live-image or target-execution concerns.

### Dedicated runtime package

A dedicated runtime package could provide one reusable composition boundary for
multiple Go binaries. It would centralize registry population, selection
invariants, dependency wiring, and error semantics while leaving each binary's
entry point responsible for when composition occurs.

The risk is turning the package into a global service locator or a second
application layer. It must not absorb updater, doctor, installer, release, or
native package-manager policy. It also may not eliminate the need for a distinct
installer composition boundary, because installed-host and target-system
lifecycles have different inputs and authority.

The location decision should be made only after direct runtime consumers and
process boundaries are settled. More than one process entry point may invoke a
shared composition facility without creating more than one selection point per
process.

## Migration constraints

- Existing Arch behavior remains unchanged while composition is introduced.
  The Arch provider is extracted behind neutral contracts without changing
  package, repository, update, doctor, boot, driver, installer, or release
  outcomes.
- The generic identity, registration, dependency, factory, and aggregate
  boundaries must be complete before a Debian provider is added. Debian must be
  addable through its own detector/provider implementation and explicit
  registration, without changing backend contracts or existing consumers.
- No package-manager logic belongs in the composition root. Native commands,
  transaction semantics, locks, repository formats, trust, and package names
  remain inside the selected backend implementation.
- Distro-specific branching must not spread across consumers. Consumers request
  backend-neutral facts and actions; provider selection is performed once at
  the process composition root.
- Unknown, unsupported, mismatched, or unregistered identities fail explicitly.
  None may silently select Arch or construct a partial substitute.
- Existing command, JSON, socket, QML, and installer process boundaries remain
  stable unless a separate migration explicitly changes their contracts.

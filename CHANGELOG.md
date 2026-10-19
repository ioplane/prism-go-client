# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- Client-side filters with the same name are evaluated as separate AND predicates with their own allowed values, preserving value/base-path OR matching, entity order and original metadata.

- Kubernetes environment providers reject a missing credential reference with a configuration error before accessing informers, instead of panicking or falling back to another provider. Valid references and original Secret lookup errors are preserved.

- Environment lookups continue to the next provider for wrapped ErrNotFound errors and return the canonical missing-key error after exhaustion. Hard errors and successful nil values still stop lookup, preserving provider order and original hard-error identity.

- A nil RoundTripper option restores the client's private default transport, preserving its TLS/proxy settings instead of falling back to the process-global transport. A typed-nil *http.Transport returns an explicit constructor error; external RoundTripper implementations with working nil receivers remain supported.

- Nil credentials, client options and loggers return constructor errors instead of panicking across shared, v3, v4 and Konnector entrypoints.

- Nusights retry documentation matches the existing 10-second minimum and 300-second maximum waits. Secretdir tests use isolated temporary directories and restore environment variables, allowing read-only source mounts.

- V4 error classification tolerates absent Status or Body fields promoted through nil embedded pointers while preserving the original error cause and classification from available fields.

- Invalid v4 ETag and list metadata shapes, including nil embedded pointers to promoted fields, return their documented empty values without reflection panics; ETag copies populate the destination without mutating shared source metadata.

- V3 ListAll pagination retains the first page, advances by raw response metadata, and stops at the reported total without repeated first-page or extra empty requests. Subnet client-side filters apply to every page. Missing or inconsistent pagination metadata and entity counts return an error instead of a partial inventory or panic; host listing retains its historical single-request contract.

- V4 Operation.Wait interrupts poll timers on local cancellation, terminates FAILED tasks without messages and CANCELED tasks, and keeps CANCELING pending. Missing error messages no longer panic; returned errors preserve Prism task diagnostics for synchronous callers and retain errors.Is(ErrTaskFailed).
- Concurrent v4 Wait callers share one poller and entity fetch; completed outcomes are stable without extra requests. Result/error slices and affected references are isolated from caller mutation, including successful operations with no returned entities. Local cancellation never cancels or deletes remote tasks or VMs.

- V3 and v4 client caches coalesce concurrent creation per key, share constructor errors, retain the previous client when replacement fails, and prevent an in-flight constructor from republishing after Delete. Functional options use the first successful publication until endpoint change or Delete.
- V3 cache retirement and failed shared constructor authentication close only SDK-owned idle transports; caller-supplied transports remain caller-owned. Trust bundle options no longer modify the caller's slice backing array.
- V4 client option errors preserve their cause through errors.Is/errors.As.

- Session cookies are synchronized and concurrent explicit 401 responses share one refresh per expired session version. Filtered requests use the same single-replay path; non-replayable bodies and transport errors never trigger a retry. Waiting callers can cancel independently. Initial unauthenticated request headers are preserved; session cookies are replaced only after an explicit 401.

- File uploads use independent replay readers over the caller-owned regular file, with Content-Length matching the remaining range. Closing an HTTP body no longer closes or advances the source file; non-regular sources are rejected before sending.

- Nil additional filters and malformed entities now return errors instead of panicking; valid filtering and original response-body ownership are preserved.
- The independent nusights module uses patched MongoDB v1.17.10; its module-level vulnerability scan is blocking, including vulnerabilities outside imported packages.
- Standalone SDK dependency advisories are resolved by compatible updates to x/net, AWS S3/eventstream, MongoDB v1 and OAuth2; whole-SDK scanning remains blocking.
- JSON decode errors retain their cause for errors.Is/errors.As in both request paths.
- HTTP non-2xx responses always return an error; filtered responses close the original body exactly once and malformed JSON returns an error without panic.
- Каждый HTTP client получает отдельный clone transport и TLS configuration; создание клиентов больше не изменяет общий `http.DefaultTransport`, его proxy и TLS verification. Неподдерживаемый или nil default transport возвращает ошибку вместо panic.

### Changed

- Make and Compose development containers run as UID 65532 with read-only SDK source mounts and the same separate writable SDK module, build and lint caches. Compose builds require the current commit SHA for image provenance. The existing gofmt checks and all declared test, module and vulnerability gates remain enabled.

- Fork pull request titles and descriptions and new commit messages use English; commit titles retain declarative Conventional Commits.

- Response diagnostics no longer echo server messages, body fields or arbitrary HTTP status text. Consumers must use `errors.As` with `ResponseError.StatusCode` and `errors.Is` with the unwrapped cause instead of matching server message strings.
- Decoded and filtered network JSON bodies have a 64 MiB SDK limit; error bodies have a 64 KiB limit. Oversized responses return `ErrResponseTooLarge`; unfiltered `io.Writer` responses remain streaming. Large inventories must use pagination.

- New commit titles use English declarative noun phrases; owner-authorized message-only history rewrites preserve source trees and backup references.
- The nusights module now also requires Go 1.27.0 with the pinned Go 1.27.2 toolchain. Consumers of that independent module must upgrade their compiler.
- The fork requires Go 1.27.0 and pins the Go 1.27.2 development toolchain.

### Security

- Request-path parse errors and HTTP client transport errors now use constant diagnostics instead of echoing URLs and query values. Migration: replace direct `*url.Error` assertions with `errors.As`; `errors.Is`, direct `net.Error` timeout/temporary classification and `os.IsTimeout` remain supported. Retained URLs and unwrapped causes may contain credentials and must not be logged.

- Secret-directory environment endpoint parse errors now use constant diagnostics instead of echoing endpoint contents, including userinfo and query values. Migration: use `errors.As` with `*url.Error` instead of matching raw parser text; the retained `URL` and unwrapped cause may contain credentials and must not be logged.

- Local and Kubernetes environment endpoint parse errors now use constant diagnostic text and retain the original *url.Error cause. Consumers must replace diagnostic string matching with errors.As; unwrapped URLs/errors may contain credentials or tokens and must not be logged. Valid URL parsing, default ports and credential lookup are unchanged.

- V4 endpoint parse-error text is now constant and constructor initialization errors preserve their original cause. Consumers must replace diagnostic string matching with errors.As for *url.Error or *strconv.NumError, or errors.Is for the underlying cause; unwrapped endpoint/port diagnostics may contain credentials or tokens and must not be logged. Valid endpoint parsing and the default port are unchanged.

- API base-URL configuration logs omit the raw URL and base-URL parse-error text is now constant. Consumers must use errors.As with *url.Error instead of diagnostic string matching; the unwrapped URL/cause may contain credentials or tokens and must not be logged. Valid URL parsing and the original parse cause are preserved.

- Proxy configuration logs omit the raw URL and proxy parse-error text is now constant. Consumers matching diagnostic strings must use errors.As with *url.Error instead; the unwrapped cause may contain credentials and must not be logged.

- API response, decode, copy and close errors use safe diagnostics while retaining causes for programmatic inspection. Non-2xx body parsing and implicit server-message logging are removed.

### Added

- The declared Podman `check` target now blocks on the existing six-linter profile for the root module, hidden `.ci` package and independent `nusights` module.

- Context-aware v3/Konnector constructors and v3/v4 cache entrypoints support cancellation of constructor authentication and independent waiters. Existing entrypoints retain their Background behavior; no new default deadline or generated-v4 transport policy is introduced.

- Reproducible offline performance baselines with fixture assertions and repeated allocation/latency samples; benchmark smoke checks run under the race detector. No production workload or performance tuning is performed.

- The modernization plan distinguishes fixed syntax-invalid JSON from still-open schema-invalid response guards and session/upload/task lifecycle work.

- Fork contribution governance checks new commit and PR titles plus Keep a Changelog structure in Podman; inherited SDK release headings are explicitly baselined.
- The modernization plan orders correctness, recovery, package structure, measured optimization and Rancher adoption through Beads dependencies.
- Blocking complete-SDK vulnerability and module consistency checks; external Keploy fixtures remain explicitly separated from unit coverage.
- Изолированные Podman regression, unit и race checks для fork; существующие тесты с внешним Keploy server перечислены отдельно от unit gate.

## [v0.6.1]
### Added
- Add transparent Basic authentication via API keys in prism-go-client

## [v0.6.0]
### Added
- Add methods for Idempotence Identifiers API in v3
- API key based authentication support in v3
- Add client method to fetch kubeconfig in Karbon
- Add converged client as utility wrapper for v4
### Changed
- Update the underlying struct used for Cluster to swagger-generated in v3
- Switch to go-swagger generated structs for recovery plan
- Change the underlying API/Client used for StorageContainers in v4
- Add defensive code to avoid panics
- Move ClientOptions to environment/types
### Removed
- Remove clients for fc & foundation
- Remove utils package

## [v0.5.5]
### Fixed
- Bugfix: Remove pagination handling in ListAllHost as endpoint doesn't support pagination

## [v0.5.4]
### Fixed
- Bugfix: Add defensive validation in v3 and v4 cache GetOrCreate to prevent panics when ManagementEndpoint has nil Address or empty required fields

## [v0.5.3]
### Added
- Added support for authenticating using API key based authentication.

## [v0.5.2]
### Added
- Added support for creating, deleting, and listing idempotence identifiers.
- added the new v4 storage container client from clustermgmt namespace as it is based on the v4 beta APIs.
- Added support for authenticating using API key based authentication.  

### Changed
- Update the v3 VM structs to include VM vTPM configuration.

### Removed
- Removed the v4 storage container client from storage namespace as it was based on the v4 alpha APIs which are no longer supported.

## [v0.5.1] - 2024-08-09
### Changed
- Updated the v3 Subnet spec struct to match latest swagger spec

## [v0.5.0] - 2024-07-29
### Added
- Added v4 Categories beta APIs to v4 client
- Added v4 VolumeGroups beta APIs to v4 client
- Added a cache for v4 clients in v4 package

### Changed
- Updated v4 API clients from v4 alpha to v4 beta  APIs
- Handle trust bundle in v4 client cache GetOrCreate by setting VerifySSL

## [0.4.0] - 2024-05-03
### Added
- Added support for v4 client creation.
- Added support for getting information about an AZ given a uuid.
- Added support for getting a projection of attributes of entities using the 'groups' API endpoint.
- Added support for creating, deleting, listing, and getting the status of recovery plan jobs.
- Add optional function options for the NewKarbonAPIClient constructor
- Add ClusterRegistration interface in karbon package
- Add ClusterRegistration SetInfo and Cluster Addon SetInfo APIs
- Added support for specifying volume groups by category in a recovery plan create request.
- Added support for specifying primary and recovery clusters in a recovery plan.
- Added WithUserAgent client option for v3 client constructor.
- Added Cache for v3 Clients in v3 package.

### Changed
- Change the MetaService interface methods to take context.Context as a parameter
- Local environment provider now fetches port from `NUTANIX_PORT` environment variable
- Add logic to internal.Client for auto retry once after refreshing auth cookie on a 401 response in case of session auth.

### Removed
- remove the unexported method from the v3 service interface enabling mocking

## [0.3.4] - 2022-11-24
### Changed
- Bugfix: Stop explicit base64 decoding of BinaryData from ConfigMap in Kubernetes env provider

## [0.3.3] - 2022-11-24
### Changed
- Kubernetes env provider can now read the trust bundle from both BinaryData and Data


## [0.3.2] - 2022-11-04
### Changed
- Bugfix: Fix the kubebuilder enum annotations for NutanixTrustBundleKind

## [0.3.1] - 2022-11-03
### Added
- Add `AdditionalTrustBundle` property to the `NutanixPrismEndpoint` struct in environment/credential/types.go
- Add `AdditionalTrustBundle` property to the `ManagementEndpoint` struct in environment/types/types/go
- Add `WithPEMEncodedCertBundle` ClientOption for handling PEM Blocks in the v3 Constructor

### Changed
- Add license header to generated file and add a makefile target for generate
- `NUTANIX_INSECURE` and `NUTANIX_ADDITIONAL_TRUST_BUNDLE` environment variables are used to hydrate environment/local provider
- Store the certpool in the client to allow injecting multiple certificates using the `WithCertificate` option
- Use `hashicorp/go-cleanhttp` as the underlying http client constructor

## [0.3.0] - 2022-09-27
### Added
- Added the "environment" package which provides abstraction for retrieving settings like API endpoints, credentials and their sources to evolve independently from clients
- Add logr based configurable logging for internal.NewClient
- Add `WithCertificate` functional option for v3 client constructor
- Add `WithRoundTripper` functional option for v3 client to add custom interceptors
- Add `WithLogger` functional option for v3 client
- Add support for Nutanix-style credentials to "secretdir" environment provider

### Changed
- The http client has been moved from pkg/nutanix to repo root
- The fc stubs have been moved from pkg/nutanix/fc to fc
- The foundation stubs have been moved from pkg/nutanix/foundation to foundation
- The karbon stubs have been moved from pkg/nutanix/karbon to karbon
- The v3 stubs have been moved from pkg/nutanix/v3 to v3
- The underlying http client is moved from root package to internal
- The root package is renamed from `prism_go_client` to `prismgoclient`
- Modify NewClient constructor to use functional options
- NewRequest and NewAuthRequest methods on internal.Client don't admit context in params
- v3 client constructor now takes functional options as parameters
- v3 client constructor returns error instead of failing silently
- Add context to v3 interface method parameters to explicitly propagate context
- Bugfix in secretdir environment provider for tolerating symlinks

### Removed
- remove internal.NewBaseClient constructor

## [0.2.0] - 2022-06-14
### Added
- Add the fc stubs from nutanix/terraform-provider-nutanix
- Add the foundation stubs from nutanix/terraform-provider-nutanix
- Add the karbon stubs from nutanix/terraform-provider-nutanix
- Added GetCurrentLoggedInUser in pkg/nutanix/v3/v3_service.go

### Changed
- Updated the http client with the latest from github.com/nutanix/terraform-provider-nutanix
- Updated the v3 stubs with the latest from github.com/nutanix/terraform-provider-nutanix
- Updated the utils package with the latest from github.com/nutanix/terraform-provider-nutanix

### Removed
- Remove the compiled binary for the client from the source code
- Remove debug logs from pkg/nutanix/client.go


## [0.1.0] - 2022-06-08
### Added
- Initial copy of the v3 stubs and http client by [@vnephologist](https://github.com/vnephologist).

### Changed
- Change MessageResource.Details type from `map[string]string` and `map[string]interface{}` to `interface{}`


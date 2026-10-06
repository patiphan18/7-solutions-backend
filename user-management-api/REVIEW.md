# Service review

Reviewed the complete service against the reference auth service's layout and patterns, while keeping this standalone challenge's dependencies and REST behavior.

| Area | Review and changes |
| --- | --- |
| Entry point / app | Signal cancellation, New/StartServer pattern, startup checks, server timeouts, worker stop/join and dependency cleanup. Cleanup errors are logged. |
| Bootstrap | Concrete dependencies are constructed here; service wiring uses AuthDeps/NewAuth. Failed construction disconnects MongoDB with an independent cleanup deadline. |
| Config | JWT secret, listen address/port, Mongo URI scheme and database name are validated before constructing dependencies. |
| Model / service | Business validation stays outside delivery. Service dependencies are private. Dependency failures retain their error chain and operation context. |
| Ports / delivery | JSON DTOs and response mapping belong to HTTP delivery rather than driving ports. Password hashes are excluded from response mapping. |
| HTTP | Non-object JSON, unknown fields, multiple JSON values and oversized bodies are rejected. 404/405 return JSON; 405 includes Allow. Logging records status and elapsed time without bodies or credentials. |
| Authentication | HS256, signature, issuer, expiry and deleted-user checks retained. Malformed subjects return 401 before persistence calls. Login enforces bcrypt password byte limits. |
| MongoDB / migrations | BSON remains in repo adapter; email uniqueness is database-enforced; index migration is idempotent. Failed reads return a zero model. Optional real MongoDB integration test covers CRUD, uniqueness and counting in an isolated test database. |
| Worker | Counts every 10 seconds with a bounded database context. Cancellation and closed tick channels stop it; shutdown cancellation is not logged as a database failure. |
| Formatting / tooling | Handwritten compact functions and long literals expanded; gofmt covers source, tests, generated Go and scripts. Make check propagates formatter failure. |
| Proto / Bazel | Versioned source, pinned Bzlmod rules, lint and generation for protobuf/gRPC/Swagger. Generated copies are writable; repeated generation produces matching checksums. |
| Docker / docs | Build entry point and ignored build artifacts reviewed; runtime is non-root. Compose config validates. README reflects current commands, architecture and generated-contract scope. |

Verification includes Go tests, go vet, build, formatting checks, race detection in WSL, Bazel lint/generation and generated-output checksum comparison.

Docker Desktop's Linux engine is unavailable on this machine. A container image build, Compose runtime and the opt-in real MongoDB integration test are not verified. The integration test is explicitly skipped when MONGO_TEST_URI is unset; mocked tests do not prove real persistence behavior.

The runtime now exposes HTTP and gRPC. All auth/user RPCs are implemented, with JWT metadata validation, domain status mapping, limits and timeouts. Generated Swagger covers all RPC methods. Unused generated gateway code, its Go runtime dependency, EventPublisher and placeholder directories were removed. Native REST retains its original response shapes and statuses. Authorization now separates ordinary users and admins in the service layer for both transports. Public registration cannot assign admin roles. Ordinary users can only read, update and delete themselves. Admins can manage all accounts. Roles are read from persistence per authenticated request; legacy records without a role default to user, and unknown roles are rejected. Operator-only cmd/admin provisions a new admin account; duplicate email never promotes an existing account.

The gRPC transport has bufconn client/server tests for all methods, public/private routing, metadata authentication, error masking and message limits. Both listeners participate in bounded graceful shutdown. Blank-line layout is enforced through golangci-lint wsl_v5/whitespace, matching the reference auth rules, alongside gofmt and LF source files.

Additional security fixes: missing accounts perform a dummy bcrypt comparison; placeholder JWT secrets are rejected; Compose binds public ports to loopback. Tests cover ownership, admin-only operations, role injection attempts, role demotion with existing tokens and invalid identities. Passwords and hashes are never returned in API responses.

Production limitations: TLS termination, ingress throttling and authenticated MongoDB must be configured by deployment. ListUsers is intentionally unpaginated to match the challenge and can consume substantial resources on large databases. Reflection is enabled on the local plaintext gRPC server. Dependency scans and unit tests do not constitute a penetration test.

Security verification on 2026-10-05:

- Go minimum and Docker build image are pinned to 1.26.7; the old 1.26.0 scan reported standard-library issues.
- gRPC is pinned to patched stable 1.83.2, following [GO-2026-6443](https://pkg.go.dev/vuln/GO-2026-6443). Compression was upgraded to 1.20.1.
- govulncheck v1.8.0 with Go 1.26.7 reports zero reachable or imported-package vulnerabilities. Module-only GO-2026-5932 concerns the unmaintained x/crypto/openpgp package; this service does not import it and the advisory lists no fixed version. The service uses bcrypt from the same module.
- gosec, unused, staticcheck and govet report no issues. An unchecked int-to-int32 conversion was replaced with a fixed-width expiry field shared by both delivery adapters.
- Duplicate HTTP Authorization headers are rejected; both transports cap request headers at 16 KiB, alongside existing body/message limits.
- Owner/admin, role injection, demotion and unknown-role tests exercise real service authorization through both adapters. The admin CLI and main service build separately.

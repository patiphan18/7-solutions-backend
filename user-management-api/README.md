# User Management API

Implementation of the User Management API portion of https://github.com/7-solutions/backend-challenge. Requires Go 1.26.7 or newer and MongoDB, or Docker Compose.

Run the commands below from `user-management-api`, which is the service root.

## Architecture

```text
user-management-api/
  cmd/main.go                       # process entry point
  cmd/admin/main.go                 # operator-only admin provisioning
  internal/
    app.go                          # New, StartServer, shutdown
    worker.go                       # periodic counting with cancellation
    bootstrap/bootstrap.go          # New: dependency construction, wiring and Close
    config/                         # environment configuration
    model/                          # domain model, errors and shared business rules
    service/                        # registration, login, authentication, CRUD
    port/
      driving/                      # service interfaces and commands
      driven/                       # repository, token and password interfaces
    delivery/
      http/                         # REST handlers and authentication middleware
      grpc/                         # protobuf handlers, mapping and JWT interceptor
    infra/repo/                     # MongoDB repository and BSON mapping
    migration/                      # idempotent unique email index migration
    lib/security/                   # JWT and bcrypt implementations
  proto/usermanagement/v1/          # versioned protobuf source and BUILD.bazel
  pkg/proto/usermanagement/v1/      # generated protobuf and gRPC
  specs/                           # generated Swagger for all RPC methods
  scripts/check.ps1                 # formatting, tests and vet checks
  MODULE.bazel                      # standalone Bazel module and pinned rules
  BUILD.bazel
  .bazelversion
  Dockerfile
  Makefile
  README.md
```

HTTP delivery calls driving ports implemented by the service layer. Services depend on driven ports; bootstrap injects MongoDB and security implementations. Model and service packages import no MongoDB, JWT, bcrypt or HTTP libraries. BSON stays in `infra/repo`; the model represents user IDs as strings.

The proto defines registration, login and all user operations. Bazel generates Go protobuf, gRPC and OpenAPI output. Bootstrap registers a real gRPC server alongside the existing REST server, both using the same service implementation. The native REST router preserves its established response shapes and status codes.

## Run with Docker

Copy `.env.example` to `.env`, replace `JWT_SECRET` with a randomly generated secret of at least 32 bytes, then run:

```sh
docker compose up --build -d
docker compose logs -f api
docker compose down
```

REST API: `http://localhost:8080`; gRPC: `localhost:9090`. MongoDB data persists in the named volume; MongoDB is accessible only inside the Compose network. Compose loads `.env` automatically. Compose publishes API ports only on loopback. The bundled MongoDB configuration is for local development; production needs authenticated MongoDB, TLS termination for both transports and ingress rate limits. The service does not implement TLS or login throttling.

## Run locally (PowerShell)

Start MongoDB on localhost:27017, then:

```powershell
$secretBytes = [byte[]]::new(32)
[System.Security.Cryptography.RandomNumberGenerator]::Fill($secretBytes)
$env:JWT_SECRET = [Convert]::ToBase64String($secretBytes)
$env:MONGO_URI = 'mongodb://localhost:27017'
go mod download
go run ./cmd
```

The Go process reads environment variables; it does not automatically load `.env`. Optional settings: `MONGO_DATABASE=user_management`, `HTTP_ADDR=:8080`, `GRPC_ADDR=:9090`. Startup checks MongoDB connectivity and creates a unique email index before accepting requests. Use Ctrl+C to shut down gracefully.

## Create an admin

Admins are provisioned by an operator with database access, rather than by a public endpoint. Set the service environment variables shown above, then run from PowerShell:

```powershell
$env:ADMIN_NAME = 'Administrator'
$env:ADMIN_EMAIL = 'admin@example.com'
$adminCredential = Get-Credential -UserName $env:ADMIN_EMAIL -Message 'Admin password (8-72 bytes)'
$env:ADMIN_PASSWORD = $adminCredential.GetNetworkCredential().Password
try { go run ./cmd/admin } finally { Remove-Item Env:ADMIN_PASSWORD }
```

This creates a new admin account and rejects duplicate email addresses. It does not elevate existing accounts. Log in using the normal login endpoint to obtain its token. With Compose, the image includes the operator CLI. Prompt for the password as above, then use:

```powershell
$env:ADMIN_PASSWORD = $adminCredential.GetNetworkCredential().Password
try {
    docker compose exec -e ADMIN_NAME -e ADMIN_EMAIL -e ADMIN_PASSWORD api /admin
} finally { Remove-Item Env:ADMIN_PASSWORD }
```

The database remains private to the Compose network.

## Endpoints

| Method | Path | Authentication | Result |
| --- | --- | --- | --- |
| POST | `/auth/register` | Public | Create user, 201 |
| POST | `/auth/login` | Public | JWT, 200 |
| POST | `/users` | Admin JWT | Create user, 201 |
| GET | `/users` | Admin JWT | All users, 200 |
| GET | `/users/{id}` | Owner or admin JWT | User, 200 |
| PATCH | `/users/{id}` | Owner or admin JWT | Update name/email, 200 |
| DELETE | `/users/{id}` | Owner or admin JWT | Delete user, 204 |

## Generate and use a JWT

Examples below use a POSIX shell; in PowerShell use `curl.exe` with appropriate JSON quoting or `Invoke-RestMethod`.

```sh
curl -X POST http://localhost:8080/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"name":"Alice","email":"alice@example.com","password":"password123"}'
```

Example response (IDs/timestamps vary):

```json
{"id":"507f1f77bcf86cd799439011","name":"Alice","email":"alice@example.com","role":"user","createdAt":"2026-10-04T00:00:00Z"}
```

```sh
curl -X POST http://localhost:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"password123"}'
```

```json
{"access_token":"<signed-jwt>","token_type":"Bearer","expires_in":3600}
```

Set `TOKEN` to the returned `access_token` and `ID` to your own user ID. Listing users and creating users through `/users` require an admin token:

```sh
TOKEN='<signed-jwt>'
ID='507f1f77bcf86cd799439011'
curl "http://localhost:8080/users/$ID" -H "Authorization: Bearer $TOKEN"
curl -X PATCH "http://localhost:8080/users/$ID" -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"Alice Updated"}'
curl -X DELETE "http://localhost:8080/users/$ID" -H "Authorization: Bearer $TOKEN"
```

Admin-only examples, using the token from an admin login:

```sh
ADMIN_TOKEN='<admin-signed-jwt>'
curl http://localhost:8080/users -H "Authorization: Bearer $ADMIN_TOKEN"
curl -X POST http://localhost:8080/users -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Bob","email":"bob@example.com","password":"password456"}'
```

List returns an array (including `[]` when empty); GET and PATCH return the same user shape as registration. DELETE has no response body. Errors use `{"error":"message"}`: 400 invalid input/ID, 401 missing/invalid/expired JWT or invalid credentials, 403 insufficient permissions, 404 user missing, 409 duplicate email, 500 unexpected database failure.

## Design decisions

- Standard `net/http` routing and logging with `slog`; HTTP method, path, status and elapsed time are logged without bodies or credentials. Unknown endpoints and unsupported methods return JSON errors (404/405); unsupported methods include Allow.
- MongoDB adapter implements a repository interface, allowing unit tests to substitute an in-memory mock. MongoDB ObjectIDs are generated by the persistence adapter and mapped to domain strings. Password hashes are excluded from JSON.
- Email is trimmed and lowercased; a database unique index handles duplicate races. Name is trimmed and limited to 100 bytes. Passwords must contain 8–72 bytes and are hashed with bcrypt.
- JWTs use HS256 only, a required expiry of one hour, issuer validation and user ID subject. Middleware checks the user still exists, so deleting a user invalidates their tokens. There is no refresh or logout endpoint.
- Registration always creates a `user`. Users can read, update and delete their own account; only `admin` can list users, create users through the protected endpoint or manage other accounts. Roles are read from MongoDB for each authenticated request, so demotion takes effect on the next request with an existing token. Records without a role are treated as ordinary users. Neither registration nor update accepts role assignments.
- HTTP request/response DTOs and JSON field names stay in delivery; driving ports use transport-independent commands. Response mapping explicitly excludes password hashes.
- Updates use PATCH and allow name/email only. Non-object JSON, unknown fields, multiple JSON values and bodies over 8 KiB are rejected. MongoDB operations have a five-second context timeout.
- Login enforces the same password byte limits as registration; malformed token subjects are rejected before repository calls. Config validates the HTTP listen address, Mongo URI scheme, database name and JWT secret before opening dependencies.
- A background goroutine counts users every 10 seconds; cancellation stops it during graceful shutdown. HTTP requests receive a shutdown grace period of 10 seconds.
- Docker support, interface abstraction, validation and graceful shutdown are included. A running gRPC listener, generated protobuf/gRPC and Swagger are supplied; the separate lottery design exercise is outside this implementation. List returns all users as required; large deployments should add pagination.

## gRPC

The gRPC server listens on `GRPC_ADDR` (default `:9090`). Register and Login are public. CreateUser, GetUser, ListUsers, UpdateUser and DeleteUser require metadata `authorization: Bearer <token>`. These methods enforce the same owner/admin rules as REST; forbidden operations return PermissionDenied. Both transports use the same application service and MongoDB repository. The gRPC interceptor validates JWT, applies a five-second deadline, maps domain errors to gRPC status codes, and logs method/code/duration. Shutdown stops HTTP and gRPC together, with a ten-second grace period and forced gRPC stop if necessary.

Use grpcurl with reflection against the local plaintext server:

```sh
grpcurl -plaintext localhost:9090 list
grpcurl -plaintext -d '{"name":"Alice","email":"alice@example.com","password":"password123"}' \
  localhost:9090 usermanagement.v1.UserManagementService/Register
grpcurl -plaintext -d '{"email":"alice@example.com","password":"password123"}' \
  localhost:9090 usermanagement.v1.UserManagementService/Login
grpcurl -plaintext -H 'authorization: Bearer <token>' -d '{"id":"<user-id>"}' \
  localhost:9090 usermanagement.v1.UserManagementService/GetUser
```

## Protobuf and Bazel

The build follows the reference auth service: Bzlmod dependencies, versioned proto directories, `buf_proto_lint_test`, `go_proto_compile`, `go_grpc_compile`, and `gateway_openapiv2_compile`. This service has its own Bazel root, so targets start with `//proto/...` rather than the reference monorepo's `//services/auth/proto/...`.

Requires Bazel 9.2.0 (or Bazelisk), Go/gofmt, Bash and Make on Linux/WSL. No system protoc or separately installed generation plugins are required; Bazel supplies them.

```sh
make lint-proto
make gen-proto
```

Generation checks lint, builds the contract and copies Bazel outputs into `pkg/proto/usermanagement/v1/` and `specs/`, then runs gofmt on generated Go files. Generated files and `MODULE.bazel.lock` are checked in. Do not edit generated files manually.

On this machine, the available WSL distro is `Ubuntu-22.04`. From a WSL terminal:

```sh
cd /mnt/c/Users/Admin/Desktop/test/7-solutions-backend/user-management-api
make gen-proto
```

Swagger is generated for all RPC methods. The running REST router retains its original JSON arrays and 201/204 status codes; no generated gateway is included. In particular, the proto ListUsersResponse wraps users in an object, while REST GET /users returns an array.

## Verification

```sh
go test ./... -cover
go vet ./...
go build -o .cache/user-management-api ./cmd
```

Tests use Go's standard testing package and mock repository interactions. They cover registration, hashing, normalization, login, CRUD, conflicts, invalid input, missing users, expired tokens, incorrect algorithms/signatures/issuers, deleted users and database errors. Unit tests do not require MongoDB. To check the real adapter, run Compose and execute the API examples above.

The MongoDB integration test creates and drops its own uniquely named test database. It checks persistence, CRUD, duplicate emails, counting and idempotent index creation. It is skipped unless `MONGO_TEST_URI` is set:

```powershell
$env:MONGO_TEST_URI = 'mongodb://localhost:27017'
go test ./internal/infra/repo -run TestMongoIntegration -v
```

For concurrency checks on Linux/WSL with a C compiler: `go test -race ./...`. Worker tests verify that cancellation stops an in-flight database count; lifecycle tests reject invalid config and canceled startup contexts.

See [REVIEW.md](REVIEW.md) for the review scope, fixes and verification limitations.

For Make users: `make fmt`, `make test`, `make vet`, `make build`, `make run`, or `make check` (POSIX shell). On PowerShell, run `./scripts/check.ps1` to verify formatting, tests and vet. Tests are colocated with their packages according to Go conventions.

Blank-line style follows golangci-lint `wsl_v5` and `whitespace`, using the same default spacing rules enabled by the reference auth service. `.golangci.yml` enables these spacing checks and the gofmt formatter. Generated code is excluded from spacing lint. Run `make fmt` to apply formatting and spacing fixes, or `make lint` to check without edits. This requires golangci-lint v2 in PATH (or set `GOLANGCI_LINT=/root/go/bin/golangci-lint`). Go source uses LF line endings. The PowerShell check script runs Go checks; use `make lint` for blank-line verification.

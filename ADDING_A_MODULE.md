# Adding a New Module

This document describes the process for adding a new module (handler) to the API.
Modules are the HTTP-facing units assembled by `Build` in
`internal/app/app.go`. Handlers are plain structs that depend on repository
interfaces (never concrete stores), which keeps them decoupled and testable.

## Required: core handler module

### 1. Create the module package

Create `handler/<name>/` with two files, mirroring `handler/tickets/`.

**`<name>.go`** — the handler struct and its HTTP methods:

```go
type TicketHandler struct {
    Tickets contracts.TicketRepository
}

func (h *TicketHandler) Create(w http.ResponseWriter, r *http.Request) { ... }
```

Conventions used across existing modules:

- Read the caller with `middleware.GetUserID(r.Context())`.
- Read URL parameters with `r.PathValue("...")`.
- Use `common.DecodeJSON`, `common.RespondJSON`, and `common.RespondError`
  from `handler/common/common.go` for request/response handling.

**`get_handlers.go`** — the `Deps` struct, `GetHandlers` factory, and route
registration (see `handler/tickets/get_handlers.go`):

```go
type Deps struct {
    Tickets contracts.TicketRepository
}

func GetHandlers(deps Deps) *TicketHandler {
    return &TicketHandler{Tickets: deps.Tickets}
}

func (h *TicketHandler) Register(mux *http.ServeMux) {
    mux.HandleFunc("POST /tickets", h.Create)
}
```

Dependencies are always **interfaces**, never concrete store types.

### 2. Wire it into `Build`

Edit `internal/app/app.go`:

1. Import the new package.
2. Instantiate it: `xHandler := x.GetHandlers(x.Deps{...})`.
3. Mount its routes. Two existing patterns:

**Sub-mux module** (tickets, admin, notifications) — register on a dedicated
mux, then wrap with the auth/ban/body middleware chain:

```go
mux.Handle("/api/tickets", middleware.NewChain(
    apiAuth,
    apiNotBanned,
    jsonBody,
).Then(http.StripPrefix("/api", ticketMux)))
```

**Top-level module** (whoami, auth) — register directly on the main mux via a
`Routes(mux, middleware)` method.

### 3. Tests

Follow `handler/tickets/`:

- **`helpers_test.go`** — copy the shared helpers (`authContext`, `authReq`,
  `decodeJSONBody`) and add a `testXHandler(t)` factory that builds the
  handler with the testutil mock.
- **`<name>_test.go`** — invoke handler methods directly with
  `httptest.NewRecorder()` and `req.SetPathValue(...)`; cover success,
  validation, and store-failure branches (via `SetErr(...)` on the mock).

### 4. Integration coverage (optional but conventional)

In `integration/http_test.go`:

- Add the new paths to `TestHTTP_Unauthenticated`.
- Add a full-flow test using `s.login(t)` that drives the endpoint
  end-to-end through the real `app.Build()` stack.

## Optional: when you need a new store

If the module needs data access that none of the existing repositories cover
(User, Community, Admin, Ticket, Notification):

### 5. Add a migration

Create a new `.sql` file in `migrations/` (see `001_init.sql`, etc.).
Migrations apply in filename order.

### 6. Implement the store

Create `store/<name>.go`:

```go
type XStore struct {
    pool DBTX
}

func NewXStore(pool DBTX) *XStore {
    return &XStore{pool: pool}
}
```

- `DBTX` (`store/db.go`) is the pgx subset used by all stores; tests
  substitute a `pgxmock` pool against it.
- Methods execute raw SQL through `s.pool` and return the store's domain
  structs (see `store/tickets.go`).
- Add `store/<name>_test.go` using `pgxmock` — reuse the shared `newMock(t)`,
  `errDB`, and `testTime` helpers defined in `store/admin_test.go`.

### 7. Add the contract

Declare the new repository interface in `handler/contracts/contracts.go`
(e.g. `contracts.TicketRepository`). The handler depends on this interface;
the concrete store implements it.

### 8. Add the test mock

Implement the contract in `internal/testutil/` (add to `mock_stores.go` or a
new file). Pattern:

- mutex + map-backed storage,
- exported result/error fields,
- `SetErr(...)` for blanket failure,
- call counters for assertions.

Extend `internal/testutil/testutil_test.go` to cover it. For per-method
failure branches, add a `Fail<X>Store` wrapper like `FailCommunityStore`.

### 9. Wire the new store in `Build`

Instantiate the store once and inject the same instance into every handler or
middleware that needs it — sharing a single store across multiple consumers is
expected (e.g. `userStore` feeds auth, whoami, settings, notifications, and
admin).

## Verification

```sh
go build ./...
go test ./...
go test -tags=integration ./integration/...
```

## Related architecture

See `DESIGN_PATTERNS.md` for the underlying principles (dependency
injection / composition root, repository, interface segregation, adapter).
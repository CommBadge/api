# Software Design Patterns in CommBadge

Pattern inventory across the whole codebase: `api`, `auto_shoutout`,
`notification_handlers`, `twitch_api_handler`, the `discord_mock` / `twitch_mock`
test doubles, and the React `ui`. References are `file:line`. (`token_refresher/`
contains only a README and is not covered.)

## Behavioral patterns

### Strategy (runtime-selected provider)
- **Where:** `ChatProvider` / `ChatSession` interfaces — `auto_shoutout/chat.go:21,28`;
  implementations `IRCChatProvider` (`auto_shoutout/irc.go:16`) and
  `MockChatProvider` (`auto_shoutout/mockchat.go:15`), selected by config in
  `auto_shoutout/main.go:42-47`. Same idea for `Publisher` —
  `twitch_api_handler/publisher.go:12` (Rabbit vs `MockPublisher`, selected in
  `main.go:24` from config).
- **Why:** swap real and fake transports at startup (prod vs dev/test) without
  touching consumers. The handler logic stays provider-agnostic.

### Observer / Producer-Consumer (channels + goroutines)
- **Where:** `ChatSession.Messages() <-chan ChatMessage` fed by
  `ircChatSession.readLoop` (`auto_shoutout/irc.go:119`) and by the polling
  goroutine in `MockChatProvider.poll` (`auto_shoutout/mockchat.go:54`); the
  `monitor` with `cancel` / `done` channels (`auto_shoutout/handler.go:26,99`);
  per-subscription goroutines managed by `twitch_mock` `Scheduler`
  (`twitch_mock/scheduler.go:16`) with a cancel map for teardown.
- **Why:** decouples producer (IRC/queue/HTTP-poll) from consumer. The
  `cancel` / `done` channels guarantee `Close()` always terminates even when a
  delivery is blocked.

### Command / Pub-Sub messaging
- **Where:** `StreamOnlineEvent` / `StreamOfflineEvent` / `Notification`
  marshaled onto a RabbitMQ topic exchange (`twitch.online` / `twitch.offline` /
  `twitch.revoke`) by `twitch_api_handler/publisher.go:53-90`; consumed by both
  `auto_shoutout/consumer.go` and `notification_handlers/consumer.go`.
  EventSub webhooks become these messages (`twitch_api_handler/main.go:35-76`).
- **Why:** webhook receiver, shoutout bot, and notification handlers are
  independent services; events decouple their lifecycles and scaling.

### Idempotent Receiver (replay protection)
- **Where:** `redisReplayGuard.seen` via `SetNX` with 15-min TTL —
  `twitch_api_handler/replay.go:12-32`; invoked from `handle` before processing
  (`twitch_api_handler/main.go:225`). The shoutout side uses the same claim
  idiom: `RedisShoutoutGuard` `StartSession` / `MarkDone`
  (`auto_shoutout/guard.go:43,72`) so only one shoutout posts per stream session.
- **Why:** Twitch redelivers webhooks for ~10 minutes; `SetNX` atomically claims
  a message/session key so duplicates are answered 200 with no side effect. A
  Redis failure returns 503 so Twitch retries (fail-safe, not fail-closed).

### Consumable single-use token (state machine)
- **Where:** OAuth `state` written via `SetState` and atomically consumed via
  `VerifyState` / Redis `GETDEL` in `api/session/redis.go:78-94`, bound to an
  HttpOnly `oauth_state` cookie in `api/handler/auth.go`. The same consume-once
  idiom secures OAuth authorization codes in both mocks —
  `twitch_mock/store.go:214` and `discord_mock/store.go:189` (`ConsumeCode`).
- **Why:** single-use + binding defeats CSRF, replay, and code re-use attacks.

### Token rotation (refresh rotation)
- **Where:** `RotateToken` in `twitch_mock/store.go:264`, `discord_mock/store.go:239`,
  and the api's Twitch/Discord OAuth clients. An old refresh token is
  invalidated the moment it is used to mint a new pair.
- **Why:** a leaked refresh token becomes unusable after the first rotation.

### Circuit Breaker / degraded mode
- **Where:** `DegradedDetector` polls Postgres/Redis and flips the service into
  degraded/halted mode; admin can force it via `SetMaintenance` →
  `DegradedHalter.Halt` (`api/middleware/degraded.go:14-67`,
  `api/handler/admin.go:24,66`).
- **Why:** centralizes "can we serve traffic" into one wrapper that can be
  tripped by either dependency failure or an operator.

### Registry + dispatch (strategy table)
- **Where:** `twitch_mock` keeps an `EventSpec` registry keyed by
  event type + version; `findEventSpec` dispatches to the right event generator
  and `validateCondition` checks payloads (`twitch_mock/events.go:15,124-141`).
  A smaller version: the allowlist `validEventType` in
  `api/handler/notifications.go:47`.
- **Why:** adding a new EventSub type is a data entry in the registry, not a
  rewrite of the request path.

## Structural patterns

### Repository
- **Where:** `UserRepository`, `CommunityRepository`, `TicketRepository`,
  `S3Repository` (`api/handler/auth.go`), `AdminRepository`
  (`api/handler/admin.go:12`), `Targets` (`auto_shoutout/targets.go:23`),
  `SubscriptionRepository` (`api/handler/notifications.go:20`); concrete stores
  in `api/store/`, `auto_shoutout/pg_targets.go`, plus the Redis `Store`s in
  `discord_mock/store.go` and `twitch_mock/store.go`, and `session.Store`
  (`api/session/redis.go`).
- **Why:** hides SQL/Redis behind domain-level data access so handlers are
  testable with fakes and schemas can change without touching business logic.

### Adapter / Anti-Corruption Layer
- **Where:** small local interfaces wrapping third-party surfaces: `Publisher`
  wraps amqp, `replayDeduper` wraps go-redis (`twitch_api_handler/replay.go:16`),
  `ShoutoutGuard` wraps Redis (`auto_shoutout/guard.go:20`), `TwitchEventSubClient`
  (`api/handler/notifications.go:29`) and `DiscordClient` / `TwitchClient` wrap
  OAuth/HTTP APIs, `Poster` wraps the Discord message endpoint
  (`notification_handlers/discord.go:13`).
- **Why:** keeps external models and SDK surface out of the domain; a Redis
  `SETNX` becomes the domain method `seen()`. Swapping implementations (or
  faking them in tests) is a one-file change.

### Dependency Injection + Composition Root / Facade
- **Where:** handlers are plain structs populated in `app.Build(deps)`
  (`api/internal/app/app.go:36-70`); `Deps` is the composition root
  (`app.go:23`) and the return value is a single `http.Handler` (facade).
  Same wiring in `auto_shoutout/main.go` and `notification_handlers`/
  `NewConsumer(cfg, handler, logger)`.
- **Why:** explicit wiring, and every test injects a mock through the same path.

### Decorator / Middleware Pipeline (Chain of Responsibility)
- **Where:** `Middleware func(http.Handler) http.Handler` and `Chain`
  (`api/middleware/chain.go:5-15`) compose `RequireAuth`, `RequireAdmin`,
  `LimitBodySize`, CORS, logging, rate limiting, and the degraded-mode wrapper —
  assembled in `api/internal/app/app.go:167-224`. The mock servers build
  routes the same way (`discord_mock/server.go:20`).
- **Why:** cross-cutting concerns (auth, body caps, throttling, outage mode) are
  layered declaratively around route handlers; per-route chain composition is
  one line.

### Interface Segregation (ISP)
- **Where:** `DiscordVerifier` (`api/handler/settings.go`) exposes only
  `GetUserGuilds` / `GetChannel` vs the broader `DiscordClient`; `DBPinger` /
  `SessionPinger` (`api/handler/health.go`); `SessionFinder` in
  `api/middleware/auth.go:18`; `TwitchProvider` composes two narrow interfaces
  (`api/internal/app/app.go:18`).
- **Why:** settings only needs channel-permission checks, health only needs a
  ping — narrow interfaces mean trivial fakes and no accidental coupling.

### Factory + runtime selection
- **Where:** constructor funcs (`NewIRCChatProvider`, `NewRateLimiter`,
  `NewRedisShoutoutGuard`, `NewRabbitPublisher`, `NewPGTargets`), config-driven
  `if cfg.MockChatURL != ""` branch (`auto_shoutout/main.go:42-47`), and the
  `getenv(key, fallback)` config loaders shared by every service
  (`auto_shoutout/config.go:27`, `twitch_mock/config.go:25`,
  `discord_mock/config.go:23`, `notification_handlers/config.go:19`).
- **Why:** centralizes construction and defers the concrete choice to
  environment configuration.

### Null Object
- **Where:** `noopReplayGuard` (`twitch_api_handler/replay.go:34`),
  `MockPublisher` (`twitch_api_handler/publisher.go:103`), `MockChatProvider`.
- **Why:** debug/test modes get a concrete no-op of the same interface,
  eliminating nil checks in the handler.

### DTO / projection (public view)
- **Where:** `Subscription.Public()` / `Transport.Public()` strip the webhook
  secret (`twitch_mock/store.go:37-48`, `twitch_mock/transport.go:17-22`);
  `PublicGuild` / `publicGuild` (`discord_mock/types.go:56`,
  `discord_mock/api.go:42`); `Session` keeps `DiscordAccessToken` server-side
  only (`api/session/redis.go:23`).
- **Why:** the wire format is a projection of the internal model, so secrets
  and internal fields can never leak by accident.

### Value object + generic helpers
- **Where:** `StreamStatus` (`notification_handlers/state.go:16`) and the
  generic `getJSON[T any]` reader reused across both mock stores
  (`twitch_mock/store.go:170`, `discord_mock/store.go:89`); `paginate`
  (`api/handler/admin.go:28`) centralizes limit/offset parsing.
- **Why:** eliminates duplicated Redis JSON plumbing and keeps pagination
  semantics in one place.

## Concurrency / resource patterns

### Singleton + pooling of shared infrastructure
- **Where:** one `pgxpool.Pool`, one `redis.Client`, one Rabbit channel created
  at startup and reused (`api/store/db.go:10`, `twitch_api_handler/main.go:23-25`).
- **Why:** connection reuse is cheaper and required by rate-limit / guard
  correctness (shared state).

### Idempotent close (`sync.Once`)
- **Where:** `ircChatSession.Close` guarded by `closeOnce` and a select-send in
  `readLoop` (`auto_shoutout/irc.go:119-154,225`); `Close()` blocks on `done`
  so shutdown always completes.
- **Why:** guarantees `stop` / `conn` are released exactly once even if both
  the consumer and the shutdown path call `Close`.

### Graceful shutdown / drain
- **Where:** `Handler.drainMonitors` cancels active monitors and waits with a
  deadline (`auto_shoutout/handler.go:179-202`); `Consumer.Run(ctx)` returns on
  `ctx.Done()` and drains deliveries; `Scheduler` cancels all per-subscription
  goroutines on shutdown (`twitch_mock/scheduler.go`).
- **Why:** in-flight work is bounded and terminated predictably on SIGTERM.

### Bounded per-key limiter map (LRU-ish eviction)
- **Where:** `RateLimiter.visitors` capped at `maxVisitors` with
  `evictOldestLocked` + periodic `cleanup` (`api/middleware/ratelimit.go:15,98-164`).
- **Why:** a flood of spoofed source IPs can't grow memory without limit.

### Clock injection (testable time)
- **Where:** `RedisStreamState` and `RedisShoutoutGuard` accept `now
  func() time.Time` (`notification_handlers/redis_state.go`,
  `auto_shoutout/guard.go`).
- **Why:** session start/stop and guard expiry logic is deterministic in tests
  without sleeping.

### Liveness vs readiness probes
- **Where:** `Healthz` (cheap 200) vs `Readyz` (dependency probes) in
  `api/handler/health.go`; probes exempt from rate limiting
  (`api/middleware/ratelimit.go`); wired in `app.go:209-210`.
- **Why:** lets orchestration distinguish "process alive" from "able to serve
  traffic", and never degrades a healthy instance because a probe was throttled.

## Simulated external services (test doubles)

Both `discord_mock` and `twitch_mock` implement the same skeleton: OAuth server
(authorize form → single-use code → rotating tokens), a Redis `Store` as the
source of truth, `SeedData` defaults (`twitch_mock/users.go:58`,
`discord_mock/seed.go:11`), bearer-token `principal` resolution
(`discord_mock/server.go:59`), and a `/api/mock/status` introspection endpoint.
`twitch_mock` additionally simulates EventSub webhooks end-to-end: verified
challenge handshake, HMAC-signed `Notify` with retry/backoff
(`twitch_mock/webhook.go:47-100`), and a `Scheduler` that fires events for
enabled subscriptions.
- **Why:** the real APIs are closed and quota-bound; the mocks let the api,
  auto_shoutout, and e2e suite exercise real HTTP, OAuth, and webhook flows
  locally.

## UI patterns (`ui/src`)

### Provider + custom hook
- **Where:** `AuthContext` / `AuthProvider` / `useAuth`
  (`ui/src/hooks/useAuth.tsx:15-81`); the hook throws if used outside the
  provider, guarding misuse at compile time.
- **Why:** one fetch-and-cache location for identity, consumed by any component.

### Adapter for mock/real API
- **Where:** `mockRequest` vs `realRequest`, selected by `const handler = MOCK ?
  mockRequest : realRequest` (`ui/src/lib/apiClient.ts:29,421,451`); the public
  `api.get/post/put/patch/delete/upload` facade stays identical.
- **Why:** the same UI runs against the real backend or an in-memory mock
  (`VITE_MOCK=true`), with route-prefixing (`/auth` vs `/api`) hidden from callers.

### Route guard + error containment
- **Where:** `ProtectedRoute` (`ui/src/components/ProtectedRoute.tsx`) wraps
  authenticated views; `ErrorBoundary` (`ui/src/components/ErrorBoundary.tsx`)
  catches render errors.
- **Why:** declarative access control at the route level and graceful failure
  instead of a blank screen.

## Defensive techniques (not GoF patterns)
- Bounded request bodies via `http.MaxBytesReader` (1 MiB) before HMAC
  verification (`twitch_api_handler/main.go:77`) and the api
  `LimitBodySize` middleware.
- Constant-time comparisons: `hmac.Equal` for EventSub signatures and
  constant-time compare for community join links.
- TTL-based expiry for OAuth state, replay keys, and shoutout session/done keys
  so Redis never accumulates dead entries.
- Output sanitization: IRC `Send` rejects `\r\n\x00`, shoutout templates reject
  control characters, `validChannelName` bounds JOIN targets
  (`auto_shoutout/irc.go:212,241`, `shoutout.go:18`).

## Notes
Most patterns predate the security work — the fixes layered the Idempotent
Receiver, Null Object, consumable-token, decorator exemptions, and
`sync.Once` / channel-shutdown idioms on top of the existing
DI / Repository / Adapter structure. The mock services and UI adapter are the
same patterns applied one layer out: they swap implementations, not
architectures.

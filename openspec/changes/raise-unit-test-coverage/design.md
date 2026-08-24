# Raise Unit Test Coverage — Iteration 2

## Context

| Metric | Value |
|--------|-------|
| Repo | github.com/infobloxopen/dapr (fork of dapr/dapr) |
| Branch | ut-coverage/20260812 |
| Iteration 1 total coverage | 31.3% (raw), 76.3% (with exclusions) |
| Target | ≥85% per remaining package |
| Packages below 85% | 8 (after exclusions) |
| Coverage tool | `go tool cover -func` on `coverage-combined.out` |

### Excluded from coverage (untestable without integration infrastructure)

| Exclusion | Reason |
|-----------|--------|
| `cmd/**` | main() entry points with os.Exit |
| `pkg/runtime/*.go` | Core runtime, 80 funcs, requires full mock infrastructure |
| `pkg/grpc/**` | gRPC API server, 52 funcs, requires gRPC test server |
| `pkg/http/**` | HTTP API server, 61 funcs, requires HTTP handler mocking |
| `pkg/actors/actor.go, actors.go, config.go` | Actor system, 44 funcs, requires state store + placement mocking |
| `pkg/operator/**` | K8s operator, requires cluster |
| `pkg/sentry/sentry.go, server/**` | Sentry service, requires gRPC + CA setup |
| `pkg/client/**` | Generated K8s client code |
| `pkg/testing/**`, `pkg/channel/testing/**` | Test helpers, not production |
| `tests/**` | Integration/E2E test apps |
| `**/monitoring/**` | OpenCensus metric boilerplate |
| `pkg/channel/grpc/**` | gRPC channel, requires gRPC server |
| `pkg/sentry/identity/kubernetes/**` | K8s identity validation |
| `utils/utils.go` | K8s client creation functions (panic on failure) |

### Remaining packages below 85%

| Package | Coverage | Funcs Below 85% | Strategy |
|---------|----------|-----------------|----------|
| pkg/apis/subscriptions/v1alpha1 | 0% | 3/3 | Trivial K8s type registration — test Kind/Resource/addKnownTypes |
| pkg/apis/configuration/v1alpha1 | 33.3% | 2/3 | Same pattern |
| pkg/apis/components/v1alpha1 | 50% | 2/4 | Same pattern |
| pkg/placement/hashing | 52.4% | 10/21 | Pure data structures — very testable |
| pkg/diagnostics | 58.6% | 29/70 | OpenCensus metrics/tracing — follow existing test patterns |
| pkg/placement/raft | 62.3% | 20/53 | Logger adapter (trivial), FSM/snapshot (unit testable), server (partial) |
| pkg/injector | 72.7% | 12/44 | Config/pod helpers (unit testable), webhook handler (needs httptest) |
| pkg/placement | 73.3% | 4/15 | Leadership functions (integration), disseminateOperation (mockable) |

## Goals

1. Reach ≥85% function-level coverage for all remaining non-excluded packages
2. Use hand-written stubs (NO gomock/testify-mock) per repo convention
3. Follow existing test patterns (testify/assert+require, table-driven, t.Run)

## Decisions

1. **OpenCensus testing**: Follow pattern from `pkg/diagnostics/http_monitoring_test.go` — call Init(), record metrics, verify via `view.RetrieveData`. Run metric tests sequentially (no t.Parallel) to avoid global state conflicts.
2. **Raft testing**: Use in-memory raft (existing `testRaftServer` in placement_test.go). For logger adapter, just verify functions don't panic.
3. **Injector testing**: Use `httptest` for webhook handler. For K8s-dependent functions (getTrustAnchorsAndCertChain, mTLSEnabled, ReplicasetAccountUID), use `k8s.io/client-go/kubernetes/fake`. For pure pod helpers (getTokenVolumeMount, podContainsSidecarContainer, isResourceDaprEnabled), direct unit tests.
4. **Placement hashing**: Fully unit testable. Set `replicationFactor` before tests, create hash ring with `Add`, test all operations.
5. **Bug found**: `service_monitoring.go:329` — `RequestBlockedByAppAction` records to `appPolicyActionAllowed` instead of `appPolicyActionBlocked`. Test will expose this but don't fix source code.
6. **Test-hostile patterns**: `getPayloadSize` panics on non-proto input — pass valid proto messages in tests. `placement/placement.go:Run` uses `log.Fatalf` — skip error paths.

## Risks

- OpenCensus global state may cause test interference — mitigate with sequential execution
- Some raft server functions (StartRaft, tryResolveRaftAdvertiseAddr) have long retry loops — test only fast paths
- Injector webhook requires understanding K8s admission review format

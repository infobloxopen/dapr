# Raise Unit Test Coverage

## Context

| Metric | Value |
|--------|-------|
| Repo | github.com/infobloxopen/dapr (fork of dapr/dapr) |
| Branch | v1.0.0-ib (default) |
| Baseline total coverage | 27.9% |
| Packages below 85% | 79 |
| Production functions below 85% | 501 |
| Coverage tool | `go tool cover -func` on `coverage-combined.out` |

### Package-level gap summary (production code only)

| Package | Uncovered Funcs | Avg Coverage |
|---------|----------------|--------------|
| pkg/concurrency | 3 | 0.0% |
| pkg/fswatcher | 1 | 0.0% |
| pkg/version | 1 | 0.0% |
| pkg/signals | 1 | 0.0% |
| pkg/middleware/http | 2 | 0.0% |
| pkg/health | 5 | 0.0% |
| pkg/messaging | 6 | 0.0% |
| utils | 4 | 10.7% |
| pkg/credentials | 6 | 11.7% |
| pkg/diagnostics/utils | 7 | 22.6% |
| pkg/metrics | 2 | 25.8% |
| pkg/components | 5 | 30.7% |
| pkg/placement | 8 | 35.4% |
| pkg/channel/grpc | 4 | 37.3% |
| pkg/runtime/security | 5 | 42.9% |
| pkg/messaging/v1 | 12 | 43.4% |
| pkg/sentry/certs | 10 | 46.0% |
| pkg/sentry/config | 4 | 53.5% |
| pkg/sentry/csr | 5 | 77.2% |
| pkg/channel/http | 2 | 79.3% |
| pkg/placement/hashing | 11 | 6.8% |
| pkg/placement/raft | 31 | 25.7% |
| pkg/injector | 13 | 6.3% |
| pkg/diagnostics | 42 | 14.9% |
| pkg/actors | 44 | 0.0% |
| pkg/grpc | 52 | 0.0% |
| pkg/http | 61 | 0.0% |
| pkg/runtime | 80 | 0.0% |
| pkg/operator | 6 | 0.0% |
| pkg/operator/api | 7 | 0.0% |
| pkg/sentry | 4 | 0.0% |
| pkg/sentry/server | 7 | 0.0% |
| pkg/sentry/monitoring | 7 | 0.0% |
| pkg/runtime/pubsub | 8 | 28.6% |
| pkg/sentry/ca | 4 | 75.4% |
| pkg/actors/internal | 2 | 72.2% |
| pkg/sentry/identity | 1 | 66.7% |
| pkg/logger | 4 | 0.0% |

## Goals

1. Achieve ≥85% function-level coverage across all production packages
2. Prioritize T1 (public API) functions with lowest coverage first
3. After T1 packages reach ≥85%, address T2 (private/internal) functions
4. Maintain CI gate compatibility — all new tests must pass

## Non-Goals

- Testing auto-generated code (pb.go, zz_generated, client/clientset, client/informers, client/listers)
- Testing cmd/ main() entry points
- Testing test utilities (pkg/testing/)
- Testing e2e/integration test apps (tests/)
- Achieving 100% coverage — target is ≥85%

## Decisions

1. **Test patterns**: table-driven tests with `[]struct`, `t.Run` subtests, `testify/assert` + `testify/require` (consistent with existing codebase)
2. **Mocking**: hand-written stubs implementing interfaces or function types — NO gomock or testify/mock (the repo does not use them). Examples: `mockOperator` struct, `mockGenCSR` function, `testServer` struct
3. **Coverage exclusions**: sonar.coverage.exclusions apply — `**/*.pb.go`, `**/zz_generated*.go`, `**/testdata/**`
4. **Iteration strategy**: Start with small/near-85% packages (quick wins), then medium packages, then large packages (actors, grpc, http, runtime)
5. **Scoping**: First iteration targets Tier 1 (small packages 0-10 funcs) and Tier 2 (near-85% packages). Subsequent iterations address larger packages.
6. **Test-hostile functions to skip or handle carefully**:
   - `utils.GetConfig()` / `utils.GetKubeClient()` — contain `panic(err)` and `flag.Parse()`; skip or test only happy path via env vars
   - `logger.Fatal` / `logger.Fatalf` — call `os.Exit(1)`; skip these
   - `placement/hashing.loadOK` — contains `panic()`; test indirectly
   - `sentry/certs/store.storeKubernetes` — hard-coupled to real k8s API; test via env-var gating to selfhosted path
   - `metrics.startMetricServer` — contains `Fatalf` in goroutine; use free port to avoid failure
7. **Key interfaces for mocking**: `channel.AppChannel`, `nr.Resolver`, `operatorv1pb.OperatorClient`, `Authenticator`, `Logger`, `ComponentLoader`
8. **Test data**: use temp directories for filesystem tests, embedded PEM constants for crypto tests (pattern from existing `tls_test.go`)

## Risks

1. Some 0% packages (actors, runtime, grpc, http) have complex dependencies requiring significant mocking infrastructure
2. Packages like pkg/operator depend on Kubernetes client — may need fake clientsets
3. Coverage improvements in large packages may require multiple iterations
4. Some functions (e.g., main(), signal handlers, Fatal) are inherently difficult to unit test
5. `utils.go` functions use `panic()` instead of returning errors — tests must avoid triggering panics or use recover
6. Global mutable state in `placement/hashing` (`replicationFactor`) and `logger` (`globalLoggers`) creates inter-test coupling — tests must reset state

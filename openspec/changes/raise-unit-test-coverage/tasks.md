> table-driven []struct, t.Run, testify/assert+require. Hand-written stubs (NO gomock/testify-mock). T1 public lowest-cov → T2 private (T1≥85% first).
> IMPORTANT: This repo uses hand-written mock structs implementing interfaces. See existing patterns in pkg/messaging/direct_messaging_test.go (mockAppChannel, mockResolver).
> WARNING: Some functions contain `panic()` or `os.Exit()` — see design.md Decisions for handling.
> NOTE: All T1 tasks from iteration 1 are COMPLETE. These are iteration 2 tasks for remaining packages.

## Completed (Iteration 1)
- [x] pkg/concurrency — 0%→100%
- [x] pkg/version — 50%→100%
- [x] pkg/signals — 0%→100%
- [x] pkg/fswatcher — 0%→86.7%
- [x] pkg/health — 58%→99.5%
- [x] pkg/middleware/http — 0%→100%
- [x] pkg/credentials — 40%→99.1%
- [x] pkg/messaging — 0%→85.6%
- [x] pkg/messaging/v1 — 84.5%→92.5%
- [x] pkg/diagnostics/utils — 44.9%→95.8%
- [x] pkg/metrics — 81.4%→97.8%
- [x] pkg/components — 61.5%→96%
- [x] pkg/config — 77%→96.7%
- [x] pkg/runtime/security — 78.1%→88.8%
- [x] pkg/sentry/config — 72%→78.2%
- [x] pkg/sentry/certs — 59.7%→92.4%
- [x] pkg/sentry/csr — 83.4%→86.5%
- [x] pkg/sentry/identity — 83.3%→100%
- [x] pkg/sentry/identity/selfhosted — 0%→100%
- [x] pkg/logger — 82.9%→87.1%

---

## T1: pkg/apis/subscriptions/v1alpha1/register.go (0.0%)
- [ ] `Kind` — call Kind("Subscription"), assert group and kind match SchemeGroupVersion — assert:assert.Equal
- [ ] `Resource` — call Resource("subscriptions"), assert group and resource — assert:assert.Equal
- [ ] `addKnownTypes` — create runtime.NewScheme(), call SchemeBuilder.AddToScheme, verify types registered — assert:require.NoError,assert.True
- [ ] Verify ≥85%

## T1: pkg/apis/components/v1alpha1/register.go (50% — Kind/Resource at 0%)
- [ ] `Kind` — call Kind("Component"), assert GroupKind — assert:assert.Equal
- [ ] `Resource` — call Resource("components"), assert GroupResource — assert:assert.Equal
- [ ] Verify ≥85%

## T1: pkg/apis/configuration/v1alpha1/register.go (33.3% — Kind/Resource at 0%)
- [ ] `Kind` — call Kind("Configuration"), assert GroupKind — assert:assert.Equal
- [ ] `Resource` — call Resource("configurations"), assert GroupResource — assert:assert.Equal
- [ ] Verify ≥85%

## T1: pkg/placement/hashing/consistent_hash.go (52.4%)
- [ ] `NewPlacementTables` — create with version string and entries map, verify fields — assert:assert.Equal,require.NotNil
- [ ] `NewHost` — create host with name/id/load/port, verify all fields — assert:assert.Equal
- [ ] `NewFromExisting` — build from pre-existing hosts/sortedSet/loadMap, verify via GetInternals — assert:assert.Equal,assert.Len
- [ ] `GetLeast` — add multiple hosts, increment loads unevenly, verify GetLeast returns least-loaded — assert:require.NoError,assert.Equal
- [ ] `GetLeast` — empty ring returns error — assert:assert.Error
- [ ] `UpdateLoad` — add hosts, set specific load, verify via GetLoads — assert:assert.Equal
- [ ] `Inc` — add host, call Inc, verify load increased — assert:assert.Equal
- [ ] `Done` — add host, Inc then Done, verify load decremented — assert:assert.Equal
- [ ] `Done` — unknown host is no-op — assert:assert.Equal (totalLoad unchanged)
- [ ] `GetLoads` — add hosts with various loads, verify returned map — assert:assert.Equal,assert.Len
- [ ] `MaxLoad` — verify formula: ceil((totalLoad/numHosts)*1.25), test with 0 total and non-zero — assert:assert.Equal
- [ ] `GetHost` — add host, verify GetHost returns full Host struct — assert:require.NoError,assert.Equal
- [ ] `GetHost` — empty ring returns error — assert:assert.Error
- [ ] NOTE: Call `SetReplicationFactor(10)` in TestMain or at start of each test. The default is 100 which creates many virtual nodes.
- [ ] Verify ≥85%

## T1: pkg/placement/raft/logger.go (0.0% — all 16 functions)
- [ ] Create `loggerAdapter{}`, call each method (Log, Trace, Debug, Info, Warn, Error) with sample args — assert they don't panic via assert.NotPanics
- [ ] Test boolean methods: IsTrace→false, IsDebug→false, IsInfo→true, IsWarn→true, IsError→true — assert:assert.Equal
- [ ] Test ImpliedArgs returns nil, With returns self, Name returns "", Named returns self, ResetNamed returns self — assert:assert.Equal,assert.NotNil
- [ ] Test SetLevel is no-op (doesn't panic) — assert:assert.NotPanics
- [ ] Test StandardLogger returns non-nil *log.Logger — assert:require.NotNil
- [ ] Test StandardWriter returns non-nil io.Writer — assert:require.NotNil
- [ ] Verify ≥85%

## T1: pkg/placement/raft/snapshot.go (25% — Release 0%, Persist 50%)
- [ ] `Release` — call on fsmSnapshot, verify no panic — assert:assert.NotPanics
- [ ] `Persist` — error path: use mock sink where Write returns error, verify sink.Cancel called — assert:assert.Error
- [ ] NOTE: MockSnapShotSink already exists in snapshot_test.go — extend it or create new mock that returns Write error
- [ ] Verify ≥85%

## T1: pkg/placement/raft/fsm.go (58.3%)
- [ ] `Apply` — old log index (before lastAppliedIndex) should be skipped — create FSM, set state.Index, call Apply with old log — assert:assert.Nil
- [ ] `Apply` — unknown command type should log error — assert:assert.Nil (returns nil for unknown type)
- [ ] `upsertMember` — invalid msgpack data returns nil — assert:assert.Nil
- [ ] `removeMember` — invalid msgpack data returns nil — assert:assert.Nil
- [ ] Verify ≥85%

## T1: pkg/placement/raft/util.go (83.3%)
- [ ] Functions already near 85% — may need one additional edge case for makeRaftLogCommand or marshalMsgPack
- [ ] Verify ≥85%

## T1: pkg/placement/raft/server.go (partial — focus on unit-testable functions)
- [ ] `raftStorePath` — test with empty raftLogStorePath (returns "log-"+id) and non-empty (returns path) — assert:assert.Equal
- [ ] `New` — test with id not in peers list returns nil — assert:assert.Nil
- [ ] `bootstrapConfig` — test with fresh in-memory stores (no existing state) returns valid config — assert:require.NoError,assert.NotNil
- [ ] `ApplyCommand` — test non-leader case returns error — requires raft not being leader — assert:assert.Error
- [ ] NOTE: Skip StartRaft disk paths, tryResolveRaftAdvertiseAddr (long retry loops), Shutdown (needs running raft). These are integration test targets.
- [ ] Verify ≥85%

## T1: pkg/diagnostics/tracing.go (mixed coverage)
- [ ] `TraceStateFromW3CString` — valid tracestate string, empty string, malformed pairs, max entries exceeded — assert:assert.NotNil,assert.Nil,assert.Equal
- [ ] `SpanContextFromW3CString` — valid traceparent, invalid format, wrong version, invalid trace-id/span-id — assert:assert.Equal (table-driven all edge cases)
- [ ] `AddAttributesToSpan` — add normal attributes, skip __dapr. prefix, skip empty values, nil span — assert:assert.NotPanics
- [ ] `ConstructInputBindingSpanAttributes` — verify returned map has correct keys — assert:assert.Equal
- [ ] `ConstructSubscriptionSpanAttributes` — verify returned map — assert:assert.Equal
- [ ] `StartInternalCallbackSpan` — create span with tracing enabled/disabled — assert:require.NotNil
- [ ] Verify ≥85%

## T1: pkg/diagnostics/http_tracing.go (partial)
- [ ] `traceStatusFromHTTPCode` — table-driven: 200→OK, 400→InvalidArgument, 401→Unauthenticated, 403→PermissionDenied, 404→NotFound, 500→Internal, 503→Unavailable, etc. — assert:assert.Equal
- [ ] `tracestateToHeader` — pass SpanContext with tracestate, verify callback receives correct header value — assert:assert.Equal
- [ ] Verify ≥85%

## T1: pkg/diagnostics/grpc_tracing.go (partial)
- [ ] `SpanContextFromIncomingGRPCMetadata` — test with grpc-trace-bin metadata, test with traceparent metadata, test with no metadata — assert:assert.Equal
- [ ] `SpanContextToGRPCMetadata` — test with valid non-empty span context — assert:require.NoError
- [ ] `UpdateSpanStatusFromGRPCError` — nil error, gRPC status error, plain error, nil span — assert:assert.NotPanics
- [ ] Verify ≥85%

## T1: pkg/diagnostics/grpc_monitoring.go (0.0%)
- [ ] `Init` — call Init with appID, verify IsEnabled returns true — assert:assert.True
- [ ] `ServerRequestReceived` — init metrics, call with context/method/size, verify via view.RetrieveData or just no panic — assert:assert.NotPanics
- [ ] `ServerRequestSent` — same pattern with status and elapsed — assert:assert.NotPanics
- [ ] `ClientRequestSent` — same pattern — assert:assert.NotPanics
- [ ] `ClientRequestRecieved` — same pattern — assert:assert.NotPanics
- [ ] `getPayloadSize` — pass valid proto.Message, verify size > 0 — assert:assert.True
- [ ] `UnaryServerInterceptor` — get interceptor, invoke with fake handler and proto request — assert:require.NoError
- [ ] `UnaryClientInterceptor` — get interceptor, invoke with fake invoker and proto request — assert:require.NoError
- [ ] NOTE: Run all metric tests WITHOUT t.Parallel to avoid OpenCensus global state conflicts.
- [ ] Verify ≥85%

## T1: pkg/diagnostics/service_monitoring.go (mixed 0-50%)
- [ ] `Init` — call Init("test-app"), verify view registration — assert:assert.NotPanics
- [ ] `ComponentLoaded`, `ComponentInitialized`, `ComponentInitFailed` — init then call each, verify no error — assert:assert.NotPanics
- [ ] `MTLSInitCompleted`, `MTLSInitFailed`, `MTLSWorkLoadCertRotationCompleted`, `MTLSWorkLoadCertRotationFailed` — same — assert:assert.NotPanics
- [ ] `ActorRebalanced`, `ActorDeactivated`, `ActorDeactivationFailed`, `ReportActorPendingCalls` — same — assert:assert.NotPanics
- [ ] `ActorStatusReported`, `ActorStatusReportFailed`, `ActorPlacementTableOperationReceived` — same — assert:assert.NotPanics
- [ ] `RequestAllowedByAppAction`, `RequestBlockedByAppAction`, `RequestAllowedByGlobalAction`, `RequestBlockedByGlobalAction` — same — assert:assert.NotPanics
- [ ] Test disabled path: don't call Init, verify methods are no-ops — assert:assert.NotPanics
- [ ] Verify ≥85%

## T1: pkg/diagnostics/http_monitoring.go (partial)
- [ ] `IsEnabled` — test before and after Init — assert:assert.False,assert.True
- [ ] `ClientRequestStarted` — init then call with method/path/size — assert:assert.NotPanics
- [ ] `ClientRequestCompleted` — init then call with method/path/status/size/elapsed — assert:assert.NotPanics
- [ ] NOTE: `FastHTTPMiddleware` is already well-tested, focus on client-side functions.
- [ ] Verify ≥85%

## T1: pkg/diagnostics/metrics.go (0%)
- [ ] `InitMetrics` — test with tracing disabled, verify no panic — assert:assert.NotPanics
- [ ] Verify ≥85%

## T1: pkg/injector/config.go (0%)
- [ ] `NewConfigWithDefaults` — verify SidecarImagePullPolicy is "Always" — assert:assert.Equal
- [ ] `GetConfigFromEnvironment` — set required env vars (TLS_CERT_FILE, TLS_KEY_FILE, SIDECAR_IMAGE, NAMESPACE), call function, verify config populated — assert:require.NoError,assert.Equal
- [ ] `GetConfigFromEnvironment` — missing required env vars returns error — assert:assert.Error
- [ ] Verify ≥85%

## T1: pkg/injector/injector.go (partial)
- [ ] `toAdmissionResponse` — pass error, verify AdmissionResponse has error message — assert:assert.Equal,assert.False (Allowed)
- [ ] `podContainsSidecarContainer` — pod with "daprd" container returns true, pod without returns false — assert:assert.True,assert.False
- [ ] `isResourceDaprEnabled` — annotations with "dapr.io/enabled"="true" returns true, "false" returns false, missing returns false — assert:assert.True,assert.False
- [ ] `getTokenVolumeMount` — pod with service account token volume, pod without — assert:require.NotNil,assert.Nil
- [ ] NOTE: Skip Run (needs TLS), handleRequest (needs K8s clients), ReplicasetAccountUID (needs K8s), getTrustAnchorsAndCertChain (needs K8s), mTLSEnabled (needs Dapr client)
- [ ] Verify ≥85%

## T1: pkg/injector/pod_patch.go (partial — getSidecarContainer at 81.8%)
- [ ] `getSidecarContainer` — test mTLS-enabled path: pass trustAnchors/certChain/certKey, verify env vars added — assert:assert.Contains
- [ ] `getSidecarContainer` — test with API token secret set, verify DAPR_API_TOKEN env var — assert:assert.Contains
- [ ] `getSidecarContainer` — test with app token secret set, verify APP_API_TOKEN env var — assert:assert.Contains
- [ ] Verify ≥85%

## T1: pkg/placement/membership.go (partial)
- [ ] `establishLeadership` — construct Service, call establishLeadership, verify hasLeadership=true and membershipCh created — assert:assert.True,require.NotNil
- [ ] `revokeLeadership` — construct Service with no active connections, call revokeLeadership, verify hasLeadership=false — assert:assert.False
- [ ] NOTE: Skip MonitorLeadership and leaderLoop (infinite loops requiring running raft). Skip disseminateOperation (needs gRPC streams).
- [ ] Verify ≥85%

## T1: Additional functions still below 85% in already-covered packages
- [ ] `pkg/channel/http/http_channel.go:InvokeMethod` (81.8%) — test error path or additional HTTP methods
- [ ] `pkg/channel/http/http_channel.go:parseChannelResponse` (76.9%) — test error response parsing, content type handling
- [ ] `pkg/actors/internal/placement.go:updatePlacements` (66.7%) — test with lock/unlock operations
- [ ] `pkg/actors/internal/placement.go:LookupActor` (77.8%) — test app not found case
- [ ] `pkg/sentry/config/config.go:getSelfhostedConfig` (57.1%) — test with valid config file
- [ ] `pkg/sentry/ca/certificate_authority.go` functions below 85% — cover remaining branches
- [ ] `pkg/messaging/v1/util.go:processGRPCToHTTPTraceHeaders` (0%) — test trace header conversion
- [ ] `pkg/messaging/v1/util.go:processHTTPToHTTPTraceHeaders` (50%) — test more header cases
- [ ] `pkg/messaging/v1/util.go:processGRPCToGRPCTraceHeader` (57.1%) — test metadata conversion
- [ ] `pkg/messaging/v1/util.go:InternalMetadataToHTTPHeader` (64.7%) — test binary header handling
- [ ] `pkg/messaging/v1/util.go:InternalMetadataToGrpcMetadata` (79.2%) — test more metadata cases
- [ ] `pkg/messaging/v1/invoke_method_request.go:EncodeHTTPQueryString` (75%) — test with special characters
- [ ] `pkg/messaging/v1/invoke_method_request.go:WithHTTPExtension` (80%) — test edge cases
- [ ] `pkg/messaging/direct_messaging.go:Invoke` (83.3%) — test additional invoke paths
- [ ] `pkg/sentry/csr/csr.go:GenerateCSR` (72.7%) — test with SPIFFE URI
- [ ] `pkg/sentry/csr/csr.go:generateBaseCert` (83.3%) — test edge cases
- [ ] `pkg/sentry/csr/csr.go:newSerialNumber` (80%) — test additional cases
- [ ] `utils/host.go:GetHostAddress` (42.9%) — test with different network interfaces

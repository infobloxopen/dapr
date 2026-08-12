> table-driven []struct, t.Run, testify/assert+require. Hand-written stubs (NO gomock/testify-mock). T1 public lowest-cov → T2 private (T1≥85% first).
> IMPORTANT: This repo uses hand-written mock structs implementing interfaces. See existing patterns: `mockOperator` in `pkg/components/kubernetes_loader_test.go`, `mockGenCSR` in `pkg/runtime/security/auth_test.go`, `testServer` in `pkg/health/health_test.go`.
> WARNING: Some functions contain `panic()` or `os.Exit()` — see design.md Decisions §6 for handling.

## T1: pkg/concurrency/limiter.go (0.0%)
- [ ] `NewLimiter` — create limiter with valid/zero/negative limits — assert:require.NotNil,assert.Equal
- [ ] `Execute` — execute function respecting concurrency limit, verify concurrent goroutines don't exceed limit — assert:assert.Equal,require.NoError
- [ ] `Wait` — wait blocks until all goroutines complete — assert:assert.Equal
- [ ] Verify ≥85%

## T1: pkg/version/version.go (50.0%)
- [ ] `Commit` — returns commit hash string — assert:assert.Equal
- [ ] Verify ≥85%

## T1: pkg/signals/signals.go (0.0%)
- [ ] `Context` — creates context that cancels on SIGTERM — assert:require.NotNil (may need signal simulation)
- [ ] Verify ≥85%

## T1: pkg/fswatcher/fswatcher.go (0.0%)
- [ ] `Watch` — watches file for changes, calls callback on modification — assert:assert.Equal (use temp file)
- [ ] Verify ≥85%

## T1: pkg/health/server.go (58.3%)
- [ ] `NewServer` — creates new health server — assert:require.NotNil
- [ ] `Ready` — sets server readiness to true — assert:assert.True
- [ ] `NotReady` — sets server readiness to false — assert:assert.False
- [ ] `Run` — starts HTTP server, responds to healthz — assert:assert.Equal (use httptest)
- [ ] `healthz` — returns 200 when ready, 500 when not ready — assert:assert.Equal
- [ ] Verify ≥85%

## T1: pkg/middleware/http/http_pipeline.go (0.0%)
- [ ] `BuildHTTPPipeline` — builds pipeline from spec — assert:require.NoError,assert.NotNil
- [ ] `Apply` — applies middleware pipeline to handler — assert:assert.Equal (use httptest)
- [ ] Verify ≥85%

## T1: pkg/credentials/credentials.go (40.2%)
- [ ] `NewTLSCredentials` — creates TLS credentials from path — assert:require.NoError,require.NotNil
- [ ] `Path` — returns credentials path — assert:assert.Equal
- [ ] `RootCertPath` — returns root cert file path — assert:assert.Contains
- [ ] `CertPath` — returns cert file path — assert:assert.Contains
- [ ] `KeyPath` — returns key file path — assert:assert.Contains
- [ ] `LoadFromDisk` (certchain.go:24) — loads cert chain from disk, error on missing files — assert:require.NoError,assert.NotEmpty,require.Error
- [ ] Verify ≥85%

## T1: utils/utils.go (28.6%)
- [ ] `initKubeConfig` — returns kubeconfig path from env/default — assert:assert.NotEmpty
- [ ] `GetConfig` — WARNING: contains panic() and flag.Parse(); skip or test only env-var path with KUBECONFIG pointing to a temp kubeconfig file — assert:require.NotNil
- [ ] `GetKubeClient` — WARNING: contains panic(); skip or requires valid kubeconfig — assert:require.NotNil
- [ ] Verify ≥85%

## T1: pkg/messaging/direct_messaging.go (33.3%)
- [ ] `NewDirectMessaging` — creates new DirectMessaging instance (NOTE: calls utils.GetHostAddress and os.Hostname in constructor) — assert:require.NotNil
- [ ] `Invoke` — invokes method on local/remote app — use hand-written stub implementing channel.AppChannel — assert:require.NoError
- [ ] `invokeWithRetry` — retries on failure, respects max retries — inject messageClientConnection func — assert:require.NoError,assert.Equal
- [ ] `invokeLocal` — calls local channel invoke — stub AppChannel interface — assert:require.NoError
- [ ] `invokeRemote` — calls remote app via gRPC — inject messageClientConnection returning mock conn — assert:require.NoError
- [ ] `getRemoteApp` — resolves remote app address — stub nr.Resolver interface — assert:assert.Equal
- [ ] Verify ≥85%

## T1: pkg/diagnostics/utils/metrics_utils.go (44.9%)
- [ ] `NewMeasureView` — creates OpenCensus view with distribution/count/last-value — assert:require.NotNil,assert.Equal
- [ ] `AddTagKeyToCtx` — adds tag key-value to context — assert:require.NoError,assert.NotNil
- [ ] `AddNewTagKey` — creates new tag key — assert:require.NotNil
- [ ] Verify ≥85%

## T1: pkg/diagnostics/utils/trace_utils.go (44.9%)
- [ ] `ExportSpan` — exports span when tracing enabled — assert:require.NoError
- [ ] `IsTracingEnabled` — returns true/false based on config — assert:assert.True,assert.False
- [ ] `GetTraceSamplingRate` — parses sampling rate string, edge cases (empty, invalid, boundary) — assert:assert.Equal
- [ ] `SpanFromContext` — extracts span from context — assert:require.NotNil
- [ ] Verify ≥85%

## T1: pkg/metrics/exporter.go (81.4%)
- [ ] `Init` — initializes metrics exporter, handles errors — assert:require.NoError
- [ ] `startMetricServer` — starts metrics HTTP server — assert:require.NoError (use free port)
- [ ] Verify ≥85%

## T1: pkg/logger/dapr_logger.go (82.9%)
- [ ] `Warn` — logs warning message — assert:assert.Contains (capture output)
- [ ] `SetOutputLevel` (options.go:31) — sets log output level — assert:assert.Equal
- [ ] `Fatal` / `Fatalf` — logs fatal (careful: calls os.Exit, may need to skip or use exec) — assert:skip or mock
- [ ] Verify ≥85%

## T1: pkg/components/standalone_loader.go (61.5%)
- [ ] `NewStandaloneComponents` — creates standalone loader from path — assert:require.NotNil
- [ ] `LoadComponents` — loads YAML component files from directory — assert:require.NoError,assert.Len
- [ ] `splitYamlDoc` — splits multi-doc YAML — assert:assert.Len,assert.Equal
- [ ] `NewKubernetesComponents` (kubernetes_loader.go:35) — creates k8s loader — assert:require.NotNil
- [ ] `LoadComponents` (kubernetes_loader.go:43) — loads components from k8s — assert:require.NoError,mock.EXPECT()
- [ ] Verify ≥85%

## T1: pkg/messaging/v1/ (84.5%)
- [ ] `Proto` (invoke_method_response.go:101) — converts response to proto — assert:require.NotNil,assert.Equal
- [ ] `Message` (invoke_method_response.go:116) — returns internal message — assert:require.NotNil
- [ ] `RawData` (invoke_method_response.go:121) — returns raw data and content type — assert:assert.Equal
- [ ] `HTTPStatusFromCode` (util.go:238) — maps gRPC code to HTTP status — assert:assert.Equal (table-driven all codes)
- [ ] `CodeFromHTTPStatus` (util.go:282) — maps HTTP status to gRPC code — assert:assert.Equal (table-driven)
- [ ] `processGRPCToHTTPTraceHeaders` (util.go:357) — converts trace headers — assert:assert.Equal
- [ ] `processHTTPToHTTPTraceHeaders` (util.go:368) — processes HTTP trace headers — assert:assert.Equal
- [ ] `processGRPCToGRPCTraceHeader` (util.go:392) — converts gRPC trace metadata — assert:assert.Equal
- [ ] `InternalMetadataToHTTPHeader` (util.go:203) — converts metadata to HTTP headers — assert:assert.Equal
- [ ] `InternalMetadataToGrpcMetadata` (util.go:135) — converts metadata to gRPC metadata — assert:assert.Equal
- [ ] `EncodeHTTPQueryString` (invoke_method_request.go:109) — encodes query params — assert:assert.Equal
- [ ] `WithHTTPExtension` (invoke_method_request.go:94) — sets HTTP method and query — assert:assert.Equal
- [ ] Verify ≥85%

## T1: pkg/config/configuration.go (76.9%)
- [ ] `LoadKubernetesConfiguration` — loads config from k8s — assert:require.NoError,mock.EXPECT()
- [ ] `GetAndParseSpiffeID` — parses SPIFFE ID from cert — assert:assert.Equal,require.NoError
- [ ] `getSpiffeID` — extracts SPIFFE ID — assert:assert.Equal
- [ ] Verify ≥85%

## T1: pkg/runtime/security/ (78.0%)
- [ ] `CreateSignedWorkloadCert` (auth.go:78) — creates signed cert — assert:require.NoError,assert.NotEmpty
- [ ] `getToken` (auth.go:157) — reads token from file/env — assert:assert.NotEmpty
- [ ] `generateCSRAndPrivateKey` (security.go:63) — generates CSR — assert:require.NoError,assert.NotEmpty
- [ ] `GetCertChain` (security.go:32) — fetches cert chain — assert:require.NoError,assert.NotNil
- [ ] `GetSidecarAuthenticator` (security.go:53) — creates authenticator — assert:require.NoError,require.NotNil
- [ ] Verify ≥85%

## T1: pkg/sentry/config/config.go (72.0%)
- [ ] `getKubernetesConfig` — loads sentry config from k8s — assert:require.NoError,mock.EXPECT()
- [ ] `getSelfhostedConfig` — loads sentry config from file — assert:require.NoError,assert.Equal
- [ ] `printConfig` — prints config without error — assert:require.NoError
- [ ] `parseConfiguration` — parses config YAML — assert:require.NoError,assert.Equal
- [ ] Verify ≥85%

## T1: pkg/sentry/certs/ (59.7%)
- [ ] `DecodePEMKey` (certs.go:34) — decodes PEM key, handles invalid input — assert:require.NoError,require.Error
- [ ] `decodeCertificatePEM` (certs.go:77) — decodes cert PEM — assert:require.NoError,require.Error
- [ ] `PEMCredentialsFromFiles` (certs.go:90) — loads creds from files — assert:require.NoError,assert.NotEmpty
- [ ] `matchCertificateAndKey` (certs.go:118) — validates cert/key pair match — assert:assert.True,assert.False
- [ ] `CertPoolFromPEM` (certs.go:142) — creates cert pool from PEM bytes — assert:require.NotNil,require.Error
- [ ] `StoreCredentials` (store.go:21) — stores creds based on hosting — assert:require.NoError
- [ ] `storeKubernetes` (store.go:28) — stores in k8s secret — assert:require.NoError,mock.EXPECT()
- [ ] `getNamespace` (store.go:56) — reads namespace from file/env — assert:assert.Equal
- [ ] `CredentialsExist` (store.go:65) — checks if creds exist on disk — assert:assert.True,assert.False
- [ ] `storeSelfhosted` (store.go:83) — stores creds to filesystem — assert:require.NoError
- [ ] Verify ≥85%

## T1: pkg/sentry/csr/csr.go (83.4%)
- [ ] `GenerateCSR` — generates CSR with org/SPIFFE URI — assert:require.NoError,assert.NotEmpty
- [ ] `GenerateCSRCertificate` — signs CSR into certificate — assert:require.NoError,assert.NotNil
- [ ] `encode` — PEM encodes cert/key — assert:assert.NotEmpty
- [ ] `generateBaseCert` — creates base x509 cert template — assert:require.NotNil
- [ ] `newSerialNumber` — generates serial number — assert:require.NoError,assert.NotNil
- [ ] Verify ≥85%

## T1: pkg/channel/grpc/grpc_channel.go (37.3%)
- [ ] `CreateLocalChannel` — creates gRPC channel — assert:require.NotNil
- [ ] `GetBaseAddress` — returns base address — assert:assert.Equal
- [ ] `InvokeMethod` — invokes method over gRPC — assert:require.NoError,mock.EXPECT()
- [ ] `invokeMethodV1` — invokes method v1 protocol — assert:require.NoError,mock.EXPECT()
- [ ] Verify ≥85%

## T1: pkg/channel/http/http_channel.go (79.3%)
- [ ] `InvokeMethod` — invokes method over HTTP — assert:require.NoError (use httptest)
- [ ] `parseChannelResponse` — parses HTTP response to internal format — assert:assert.Equal,require.NoError
- [ ] Verify ≥85%

## T1: pkg/sentry/ca/ (75.4%)
- [ ] Cover remaining uncovered branches in CA functions — assert:require.NoError
- [ ] Verify ≥85%

## T1: pkg/sentry/identity/ (66.7%)
- [ ] Cover remaining uncovered identity validation — assert:assert.Equal
- [ ] Verify ≥85%

## T1: pkg/actors/internal/ (72.2%)
- [ ] Cover remaining uncovered internal actor functions — assert:assert.Equal
- [ ] Verify ≥85%

---

## T2: pkg/placement/hashing/ (50.5%) — after T1 done
- [ ] Cover all hash ring functions — consistent hashing, virtual nodes, lookup — assert:assert.Equal,assert.NotNil
- [ ] Verify ≥85%

## T2: pkg/placement/raft/ (55.9%) — after T1 done
- [ ] Cover raft state machine operations — apply, snapshot, restore — assert:require.NoError,assert.Equal
- [ ] Verify ≥85%

## T2: pkg/placement/ (64.3%) — after T1 done
- [ ] Cover placement service functions — member management, table dissemination — assert:require.NoError,mock.EXPECT()
- [ ] Verify ≥85%

## T2: pkg/injector/ (72.3%) — after T1 done
- [ ] Cover sidecar injector webhook — pod patching, container spec — assert:require.NoError,assert.Contains
- [ ] Verify ≥85%

## T2: pkg/diagnostics/ (48.1%) — after T1 done
- [ ] Cover diagnostics functions — metrics recording, trace handling — assert:require.NoError,assert.Equal
- [ ] Verify ≥85%

## T2: pkg/runtime/pubsub/ (42.8%) — after T1 done
- [ ] Cover pubsub runtime functions — subscription matching, message handling — assert:require.NoError,assert.Equal
- [ ] Verify ≥85%

## T2: pkg/injector/monitoring/ (0.0%) — after T1 done
- [ ] Cover monitoring counters — record sidecar injection success/failure — assert:require.NoError
- [ ] Verify ≥85%

## T2: pkg/placement/monitoring/ (0.0%) — after T1 done
- [ ] Cover placement monitoring metrics — assert:require.NoError
- [ ] Verify ≥85%

---

## T3: pkg/actors/ (0.0%) — after T1+T2 done
- [ ] Cover actor lifecycle — create, invoke, deactivate, timers, reminders — assert:require.NoError,mock.EXPECT()
- [ ] Verify ≥85%

## T3: pkg/grpc/ (0.0%) — after T1+T2 done
- [ ] Cover gRPC API server — all API methods — assert:require.NoError,mock.EXPECT()
- [ ] Verify ≥85%

## T3: pkg/http/ (0.0%) — after T1+T2 done
- [ ] Cover HTTP API server — all API endpoints — assert:require.NoError (use httptest)
- [ ] Verify ≥85%

## T3: pkg/runtime/ (0.0%) — after T1+T2 done
- [ ] Cover runtime initialization and lifecycle — assert:require.NoError,mock.EXPECT()
- [ ] Verify ≥85%

## T3: pkg/operator/ (0.0%) — after T1+T2 done
- [ ] Cover operator reconciliation logic — assert:require.NoError,mock.EXPECT()
- [ ] Verify ≥85%

## T3: pkg/sentry/ (0.0%) — after T1+T2 done
- [ ] Cover sentry CA server functions — assert:require.NoError,mock.EXPECT()
- [ ] Verify ≥85%

## T3: pkg/sentry/server/ (0.0%) — after T1+T2 done
- [ ] Cover sentry gRPC server — certificate signing — assert:require.NoError,mock.EXPECT()
- [ ] Verify ≥85%

## T3: pkg/sentry/monitoring/ (0.0%) — after T1+T2 done
- [ ] Cover sentry monitoring metrics — assert:require.NoError
- [ ] Verify ≥85%

## T3: pkg/operator/api/ (0.0%) — after T1+T2 done
- [ ] Cover operator API handlers — assert:require.NoError,mock.EXPECT()
- [ ] Verify ≥85%

## T3: pkg/operator/monitoring/ (0.0%) — after T1+T2 done
- [ ] Cover operator monitoring metrics — assert:require.NoError
- [ ] Verify ≥85%

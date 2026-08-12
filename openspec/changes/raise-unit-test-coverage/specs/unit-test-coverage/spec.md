# Unit Test Coverage — Iteration 2 Spec

## Target: ≥85% function-level coverage per non-excluded package

### Req: K8s API Type Registration (apis/*/v1alpha1)
#### Scenario: Kind returns correct GroupKind
WHEN Kind("Subscription") is called on subscriptions/v1alpha1
THEN result.Group == SchemeGroupVersion.Group AND result.Kind == "Subscription"

#### Scenario: Resource returns correct GroupResource
WHEN Resource("subscriptions") is called on subscriptions/v1alpha1
THEN result.Group == SchemeGroupVersion.Group AND result.Resource == "subscriptions"

#### Scenario: addKnownTypes registers types
WHEN AddToScheme is called with a new runtime.Scheme
THEN scheme recognizes the registered types

### Req: Consistent Hash Ring (placement/hashing)
#### Scenario: NewPlacementTables constructor
WHEN NewPlacementTables("v1", entries) is called
THEN returned struct has Version="v1" and Entries matching input

#### Scenario: NewHost constructor
WHEN NewHost("host1", "id1", 0, 50001) is called
THEN returned Host has correct Name, AppID, Load, Port fields

#### Scenario: NewFromExisting rebuilds ring
WHEN NewFromExisting(hosts, sortedSet, loadMap) is called
THEN GetInternals returns matching data

#### Scenario: GetLeast returns least loaded host
WHEN multiple hosts added with uneven loads (Inc)
THEN GetLeast returns the host with lowest load for a given key

#### Scenario: GetLeast on empty ring
WHEN GetLeast called on ring with no hosts
THEN error is returned

#### Scenario: UpdateLoad sets specific load
WHEN UpdateLoad("host1", 50) called after adding host1
THEN GetLoads()["host1"] == 50

#### Scenario: Inc/Done modify load atomically
WHEN Inc("host1") called followed by Done("host1")
THEN load returns to original value

#### Scenario: MaxLoad formula
WHEN totalLoad=10, numHosts=4
THEN MaxLoad returns ceil((10/4)*1.25) = 4

### Req: Raft Logger Adapter (placement/raft/logger.go)
#### Scenario: All logging methods don't panic
WHEN Log/Trace/Debug/Info/Warn/Error called with args
THEN no panic occurs

#### Scenario: Boolean level checks
WHEN IsTrace/IsDebug/IsInfo/IsWarn/IsError called
THEN returns expected boolean values

#### Scenario: Utility methods
WHEN With/Named/ResetNamed/StandardLogger/StandardWriter called
THEN returns non-nil values without panic

### Req: Raft FSM Apply (placement/raft/fsm.go)
#### Scenario: Apply skips old log
WHEN Apply called with log.Index < state.Index
THEN returns nil without processing

#### Scenario: Apply handles unknown command
WHEN Apply called with unknown command type
THEN returns nil

#### Scenario: upsertMember/removeMember with invalid data
WHEN called with invalid msgpack bytes
THEN returns nil (logs error internally)

### Req: Diagnostics Tracing
#### Scenario: TraceStateFromW3CString valid
WHEN valid tracestate "key1=value1,key2=value2" passed
THEN returns parsed Tracestate with correct entries

#### Scenario: TraceStateFromW3CString empty
WHEN empty string passed
THEN returns nil Tracestate

#### Scenario: SpanContextFromW3CString valid
WHEN valid traceparent "00-{traceID}-{spanID}-01" passed
THEN returns SpanContext with correct IDs and sampled flag

#### Scenario: traceStatusFromHTTPCode mapping
WHEN HTTP 200 passed THEN returns trace.StatusCodeOK
WHEN HTTP 400 passed THEN returns trace.StatusCodeInvalidArgument
WHEN HTTP 404 passed THEN returns trace.StatusCodeNotFound
WHEN HTTP 500 passed THEN returns trace.StatusCodeInternal

### Req: Diagnostics gRPC Monitoring
#### Scenario: Init enables metrics
WHEN Init("test-app") called
THEN IsEnabled() returns true

#### Scenario: Server/Client metrics recording
WHEN ServerRequestReceived/ServerRequestSent/ClientRequestSent/ClientRequestRecieved called after Init
THEN no panic and metrics are recorded

#### Scenario: UnaryServerInterceptor chains correctly
WHEN interceptor invoked with handler returning response
THEN handler is called and metrics recorded

### Req: Diagnostics Service Monitoring
#### Scenario: All metric functions work when enabled
WHEN Init("app") called then each metric function invoked
THEN no panic occurs

#### Scenario: Metrics are no-ops when not initialized
WHEN metric functions called without Init
THEN no panic (disabled path)

### Req: Injector Config and Helpers
#### Scenario: NewConfigWithDefaults
WHEN NewConfigWithDefaults() called
THEN SidecarImagePullPolicy == "Always"

#### Scenario: GetConfigFromEnvironment success
WHEN required env vars set
THEN config populated correctly

#### Scenario: toAdmissionResponse
WHEN error passed
THEN response.Allowed == false and Result.Message contains error

#### Scenario: podContainsSidecarContainer
WHEN pod has container named "daprd" THEN returns true
WHEN pod has no "daprd" container THEN returns false

#### Scenario: isResourceDaprEnabled
WHEN annotations["dapr.io/enabled"] == "true" THEN returns true
WHEN annotation missing THEN returns false

### Req: Placement Leadership
#### Scenario: establishLeadership sets state
WHEN establishLeadership() called on Service
THEN hasLeadership == true and membershipCh is non-nil

#### Scenario: revokeLeadership clears state
WHEN revokeLeadership() called with no active connections
THEN hasLeadership == false

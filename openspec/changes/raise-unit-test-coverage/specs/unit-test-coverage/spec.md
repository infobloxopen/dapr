# Unit Test Coverage Specifications

## pkg/concurrency

### Req: NewLimiter creates a concurrency limiter with configurable limit
#### Scenario: valid limit
WHEN NewLimiter is called with limit=5
THEN it returns a non-nil Limiter with the configured limit

#### Scenario: zero limit
WHEN NewLimiter is called with limit=0
THEN it returns a Limiter (behavior depends on implementation)

### Req: Execute runs function within concurrency limit
#### Scenario: single execution
WHEN Execute is called with a function
THEN the function runs and the active count increments/decrements correctly

#### Scenario: concurrent execution respects limit
WHEN Execute is called 10 times with limit=3
THEN at most 3 functions execute concurrently

### Req: Wait blocks until all goroutines complete
#### Scenario: all done
WHEN Wait is called after Execute calls complete
THEN it returns after all goroutines finish

---

## pkg/version

### Req: Commit returns the git commit hash
#### Scenario: default value
WHEN Commit is called
THEN it returns the commit variable value (empty string if not set via ldflags)

---

## pkg/health

### Req: NewServer creates a health check HTTP server
#### Scenario: valid creation
WHEN NewServer is called
THEN it returns a non-nil Server with ready=false by default

### Req: Ready/NotReady toggles server readiness
#### Scenario: toggle ready
WHEN Ready() is called
THEN the server reports ready=true

#### Scenario: toggle not ready
WHEN NotReady() is called after Ready()
THEN the server reports ready=false

### Req: healthz endpoint returns correct status
#### Scenario: server ready
WHEN GET /healthz is called and server is ready
THEN response status is 200

#### Scenario: server not ready
WHEN GET /healthz is called and server is not ready
THEN response status is 500

---

## pkg/credentials

### Req: NewTLSCredentials creates credentials from directory path
#### Scenario: valid path
WHEN NewTLSCredentials is called with a valid directory
THEN it returns TLSCredentials with correct sub-paths

### Req: Path/RootCertPath/CertPath/KeyPath return correct file paths
#### Scenario: all paths
WHEN accessor methods are called
THEN they return the expected file paths within the credentials directory

### Req: LoadFromDisk loads certificate chain from filesystem
#### Scenario: valid certs exist
WHEN LoadFromDisk is called with valid cert/key/ca files on disk
THEN it returns a populated CertChain with no error

#### Scenario: missing files
WHEN LoadFromDisk is called with missing files
THEN it returns an error

---

## utils

### Req: initKubeConfig returns kubernetes config path
#### Scenario: KUBECONFIG env set
WHEN KUBECONFIG environment variable is set
THEN initKubeConfig returns that path

#### Scenario: default path
WHEN KUBECONFIG is not set
THEN initKubeConfig returns ~/.kube/config

---

## pkg/messaging

### Req: NewDirectMessaging creates messaging instance
#### Scenario: valid creation
WHEN NewDirectMessaging is called with valid params
THEN it returns a non-nil DirectMessaging

### Req: Invoke dispatches to local or remote based on target
#### Scenario: local app
WHEN Invoke is called for the local app ID
THEN it delegates to invokeLocal

#### Scenario: remote app
WHEN Invoke is called for a remote app ID
THEN it resolves the address and delegates to invokeRemote

### Req: invokeWithRetry retries failed invocations
#### Scenario: success on first try
WHEN the invocation succeeds on first attempt
THEN it returns the result without retry

#### Scenario: retry on transient failure
WHEN the invocation fails with a transient error
THEN it retries up to the max retry count

---

## pkg/messaging/v1

### Req: HTTPStatusFromCode maps gRPC status codes to HTTP status codes
#### Scenario: standard mappings
WHEN HTTPStatusFromCode is called with each gRPC code
THEN it returns the correct HTTP status (OK→200, NotFound→404, etc.)

### Req: CodeFromHTTPStatus maps HTTP status codes to gRPC codes
#### Scenario: standard mappings
WHEN CodeFromHTTPStatus is called with each HTTP status
THEN it returns the correct gRPC code (200→OK, 404→NotFound, etc.)

### Req: Proto converts InvokeMethodResponse to protobuf
#### Scenario: valid response
WHEN Proto() is called on a populated response
THEN it returns a valid InvokeMethodResponse proto message

### Req: processGRPCToHTTPTraceHeaders converts trace context
#### Scenario: with traceparent
WHEN gRPC metadata contains traceparent
THEN the HTTP headers contain the same trace context

---

## pkg/sentry/certs

### Req: DecodePEMKey decodes PEM-encoded private key
#### Scenario: valid RSA key
WHEN DecodePEMKey is called with valid RSA PEM
THEN it returns the parsed private key

#### Scenario: valid EC key
WHEN DecodePEMKey is called with valid EC PEM
THEN it returns the parsed private key

#### Scenario: invalid PEM
WHEN DecodePEMKey is called with invalid PEM data
THEN it returns an error

### Req: CertPoolFromPEM creates x509 cert pool
#### Scenario: valid PEM certificates
WHEN CertPoolFromPEM is called with valid cert PEM
THEN it returns a non-nil CertPool

#### Scenario: invalid PEM
WHEN CertPoolFromPEM is called with invalid data
THEN it returns an error

### Req: CredentialsExist checks filesystem for credential files
#### Scenario: files exist
WHEN all credential files exist in the path
THEN CredentialsExist returns true

#### Scenario: files missing
WHEN credential files don't exist
THEN CredentialsExist returns false

---

## pkg/sentry/csr

### Req: GenerateCSR creates a certificate signing request
#### Scenario: with SPIFFE URI
WHEN GenerateCSR is called with org and SPIFFE URI
THEN it returns valid PEM-encoded CSR and private key

### Req: GenerateCSRCertificate signs a CSR
#### Scenario: valid CSR and CA
WHEN GenerateCSRCertificate is called with valid CSR and CA cert/key
THEN it returns a signed certificate

### Req: newSerialNumber generates unique serial numbers
#### Scenario: uniqueness
WHEN newSerialNumber is called multiple times
THEN each returned serial number is unique

---

## pkg/sentry/config

### Req: getSelfhostedConfig loads config from file
#### Scenario: valid config file
WHEN config file exists with valid YAML
THEN it returns parsed SentryConfig

#### Scenario: missing file
WHEN config file doesn't exist
THEN it returns default config

### Req: parseConfiguration parses SentryConfig from bytes
#### Scenario: valid YAML
WHEN valid YAML bytes are passed
THEN it returns correct SentryConfig

#### Scenario: invalid YAML
WHEN invalid bytes are passed
THEN it returns an error

---

## pkg/config

### Req: GetAndParseSpiffeID extracts SPIFFE ID from certificate
#### Scenario: valid SPIFFE cert
WHEN a certificate with SPIFFE URI SAN is provided
THEN it extracts and returns the parsed SPIFFE ID

#### Scenario: no SPIFFE URI
WHEN a certificate without SPIFFE URI is provided
THEN it returns an error

---

## pkg/runtime/security

### Req: GetCertChain retrieves certificate chain from sentry
#### Scenario: valid response
WHEN sentry returns valid cert chain
THEN GetCertChain returns the chain with no error

### Req: generateCSRAndPrivateKey creates CSR and key pair
#### Scenario: valid generation
WHEN called with valid ID
THEN it returns PEM-encoded CSR and private key

---

## pkg/components

### Req: LoadComponents loads component YAML from directory (standalone)
#### Scenario: valid YAML files
WHEN directory contains valid component YAML files
THEN LoadComponents returns parsed Component objects

#### Scenario: empty directory
WHEN directory is empty
THEN LoadComponents returns empty slice

### Req: splitYamlDoc splits multi-document YAML
#### Scenario: multi-doc
WHEN YAML contains multiple documents separated by ---
THEN splitYamlDoc returns separate byte slices for each

---

## pkg/metrics

### Req: Init initializes the metrics exporter
#### Scenario: successful init
WHEN Init is called with valid options
THEN it registers views and starts the server without error

---

## pkg/channel/grpc

### Req: CreateLocalChannel creates a gRPC channel to the app
#### Scenario: valid port
WHEN called with a valid port
THEN returns a GRPCChannel with the correct base address

### Req: InvokeMethod invokes a method over gRPC
#### Scenario: successful invocation
WHEN a valid InvokeMethodRequest is provided
THEN it calls the app's gRPC endpoint and returns the response

---

## pkg/channel/http

### Req: parseChannelResponse parses HTTP response into internal format
#### Scenario: JSON response
WHEN HTTP response has JSON body and headers
THEN parseChannelResponse returns correct InvokeMethodResponse with content type

#### Scenario: empty body
WHEN HTTP response has empty body
THEN parseChannelResponse returns response with nil data

---

## pkg/diagnostics/utils

### Req: NewMeasureView creates OpenCensus metric views
#### Scenario: distribution aggregation
WHEN NewMeasureView is called with distribution type
THEN it returns a View with distribution aggregation

### Req: IsTracingEnabled checks tracing configuration
#### Scenario: tracing enabled
WHEN tracing spec has sampling rate > 0
THEN IsTracingEnabled returns true

#### Scenario: tracing disabled
WHEN tracing spec has sampling rate = 0
THEN IsTracingEnabled returns false

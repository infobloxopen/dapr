/*
Copyright 2024 The Dapr Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package authz

import (
	"context"
	"fmt"
	"sync/atomic"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dapr/dapr/pkg/security/spiffe"
)

// mtlsDisabled is an atomic flag that disables authz checks when mTLS is off.
var mtlsDisabled atomic.Bool

// SetMTLSDisabled sets whether mTLS-based authz should be bypassed.
// When disabled, all requests are allowed without identity verification.
func SetMTLSDisabled(disabled bool) {
	mtlsDisabled.Store(disabled)
}

// Request ensures that the requesting identity resides in the same
// namespace as that of the requested namespace.
// When mTLS is disabled, the check is skipped and a nil identity is returned.
func Request(ctx context.Context, namespace string) (*spiffe.Parsed, error) {
	if mtlsDisabled.Load() {
		// When mTLS is disabled, return a synthetic identity with the
		// requested namespace so that namespace-scoped filtering still
		// works. AppID will be empty, allowing all unscoped resources.
		return spiffe.Synthetic(namespace, ""), nil
	}

	id, ok, err := spiffe.FromGRPCContext(ctx)
	if err != nil || !ok {
		return nil, status.New(codes.PermissionDenied, "failed to determine identity").Err()
	}

	if len(namespace) == 0 || id.Namespace() != namespace {
		return nil, status.New(codes.PermissionDenied, fmt.Sprintf("identity does not match requested namespace exp=%s got=%s", namespace, id.Namespace())).Err()
	}

	return id, nil
}

// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package slo2bq

import (
	"context"
	"fmt"
	"time"

	"strconv"

	"slo2bq/clients"
)

const bqLeaseLabelName = "slo2bq_lease_expiration"

// BqLease provides a simple lease mechanism that uses BigQuery dataset metadata as a key/value store.
type bqLease struct {
	bq      clients.BigQueryClient
	dataset string
}

// NewBqLease tries to obtain a new lease valid until `expiration` timestamp.
// An error is returned if there is an existing lease with expiration time in the future, or
// if another process manages to update lease information concurrently with this function.
func newBqLease(ctx context.Context, client clients.BigQueryClient, dataset string, expiration time.Time) (*bqLease, error) {
	exp, etag, err := client.ReadDatasetMetadataLabel(ctx, dataset, bqLeaseLabelName)
	if err != nil {
		return nil, err
	}

	if exp != "" {
		// Label value is expiration time as a Unix timestamp (stored as a string).
		// I wish we could use time.RFC3339 here, but it violates allowed character set for label values.
		ts, err := strconv.ParseInt(exp, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("Could not parse BQ lease expiration time %v: %v", exp, err)
		}
		if t := time.Unix(ts, 0); t.After(time.Now()) {
			return nil, fmt.Errorf("Could not obtain BQ lease: existing lease is still valid until %v", t)
		}
	}

	value := strconv.FormatInt(expiration.Unix(), 10)
	if err := client.WriteDatasetMetadataLabel(ctx, dataset, bqLeaseLabelName, value, etag); err != nil {
		// Passing `etag` ensures that an update will fail if metadata has been modified by someone else.
		return nil, fmt.Errorf("Could not update BQ lease: %v", err)
	}
	return &bqLease{bq: client, dataset: dataset}, nil
}

// Close releases the obtained lease by clearing the expiration time.
func (l *bqLease) Close(ctx context.Context) error {
	return l.bq.WriteDatasetMetadataLabel(ctx, l.dataset, bqLeaseLabelName, "", "")
}

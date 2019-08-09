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

// Package slo2bq exports a GCF function to sync Stackdriver SLO data to BigQuery.
package slo2bq

import (
	"context"
	"encoding/json"
	"log"
	"slo2bq/clients"
	"time"

	"golang.org/x/oauth2/google"
)

// BigQuery table name for the raw data.
const tableName = "data"

// Config is a configuration structure expected by this function as JSON in a PubSub message.
type Config struct {
	Project  string
	Dataset  string
	TimeZone string
}

// PubSubMessage is the message received from pubsub. Payload (`Data` field) should be a JSON-serialized Config message.
type PubSubMessage struct {
	Data []byte `json:"data"`
}

// SyncSloPerformance is the exported function triggered via a pubsub queue.
func SyncSloPerformance(ctx context.Context, m PubSubMessage) error {
	var cfg Config
	if err := json.Unmarshal(m.Data, &cfg); err != nil {
		return err
	}
	log.Printf("Got configuration: %+v", cfg)

	bq, err := clients.NewBQClient(ctx, cfg.Project)
	if err != nil {
		return err
	}
	defer bq.Close()

	// GCF runtime will kill the function after 9 minutes, so getting a lease for 10 minutes
	// ensures that at most one instance of the function is executed at any time.
	l, err := newBqLease(ctx, bq, cfg.Dataset, time.Now().Add(10*time.Minute))
	if err != nil {
		return err
	}
	defer l.Close(ctx)

	h, err := google.DefaultClient(ctx)
	if err != nil {
		return err
	}

	sd, err := clients.NewStackdriverMetricClient(ctx)
	if err != nil {
		return err
	}
	defer sd.Close()

	slo := clients.NewStackdriverSLOClient(cfg.Project, h)
	return syncAllServices(ctx, &cfg, sd, slo, bq)
}

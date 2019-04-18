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

// Package clients provides clients for GCP services.
// This file contains Stackdriver Metric client.
package clients

import (
	"context"

	monitoring "cloud.google.com/go/monitoring/apiv3"
	"google.golang.org/api/iterator"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
)

//go:generate mockgen -destination=mocks/mock_metric_client.go -package mocks slo2bq/clients MetricClient

// MetricClient defines Stackdriver functions implemented by StackdriverMetricClient.
type MetricClient interface {
	ListTimeSeries(context.Context, *monitoringpb.ListTimeSeriesRequest) ([]*monitoringpb.TimeSeries, error)
	Close() error
}

// StackdriverMetricClient wraps Stackdriver metric client, implementing MetricClient interface.
type StackdriverMetricClient struct {
	sd *monitoring.MetricClient
}

// NewStackdriverMetricClient returns a new client.
func NewStackdriverMetricClient(ctx context.Context) (*StackdriverMetricClient, error) {
	sd, err := monitoring.NewMetricClient(ctx)
	if err != nil {
		return nil, err
	}
	return &StackdriverMetricClient{sd}, nil
}

// Close closes the metric client.
func (c *StackdriverMetricClient) Close() error {
	return c.sd.Close()
}

// ListTimeSeries queries time series.
func (c *StackdriverMetricClient) ListTimeSeries(ctx context.Context, req *monitoringpb.ListTimeSeriesRequest) ([]*monitoringpb.TimeSeries, error) {
	it := c.sd.ListTimeSeries(ctx, req)
	var series []*monitoringpb.TimeSeries
	for {
		t, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		series = append(series, t)
	}
	return series, nil
}

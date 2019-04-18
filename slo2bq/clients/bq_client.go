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
// This file contains BigQuery client.
package clients

import (
	"context"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/iterator"
)

// BQRow represents data in a single BigQuery row.
type BQRow struct {
	Service, SLO, Date string
	Total, Good        int64
	Target             float64
}

// Save implements the ValueSaver interface.
func (r *BQRow) Save() (map[string]bigquery.Value, string, error) {
	return map[string]bigquery.Value{
		"Service": r.Service,
		"SLO":     r.SLO,
		"Date":    r.Date,
		"Total":   r.Total,
		"Good":    r.Good,
		"Target":  r.Target,
	}, "", nil
}

//go:generate mockgen -destination=mocks/mock_bq_client.go -package mocks slo2bq/clients BigQueryClient

// BigQueryClient is the interface implemented by this BQ client.
type BigQueryClient interface {
	Query(context.Context, string) ([]*BQRow, error)
	Put(context.Context, string, string, []*BQRow) error
	ReadDatasetMetadataLabel(context.Context, string, string) (string, string, error)
	WriteDatasetMetadataLabel(context.Context, string, string, string, string) error
	Close() error
}

// BQClient is a simple client reading and writing BQRows to BigQuery.
type BQClient struct {
	bq *bigquery.Client
}

// NewBQClient returns a BQClient for a given project name.
func NewBQClient(ctx context.Context, project string) (*BQClient, error) {
	bq, err := bigquery.NewClient(ctx, project)
	if err != nil {
		return nil, err
	}
	return &BQClient{bq}, nil
}

// Close closes the enclosed BigQuery client.
func (c *BQClient) Close() error {
	return c.bq.Close()
}

// Query runs a given SQL query and returns a slice of BQRows.
func (c *BQClient) Query(ctx context.Context, query string) ([]*BQRow, error) {
	q := c.bq.Query(query)
	job, err := q.Run(ctx)
	if err != nil {
		return nil, err
	}
	status, err := job.Wait(ctx)
	if err != nil {
		return nil, err
	}
	if err := status.Err(); err != nil {
		return nil, err
	}

	var result []*BQRow
	it, err := job.Read(ctx)
	for {
		var row BQRow
		err := it.Next(&row)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		result = append(result, &row)
	}
	return result, nil
}

// Put writes several BQRows to BigQuery.
func (c *BQClient) Put(ctx context.Context, dataset, table string, rows []*BQRow) error {
	return c.bq.Dataset(dataset).Table(table).Uploader().Put(ctx, rows)
}

// ReadDatasetMetadataLabel reads metadata for a given BigQuery Dataset and returns value of
// a specific label as well as the current etag for metadata.
func (c *BQClient) ReadDatasetMetadataLabel(ctx context.Context, dataset, label string) (string, string, error) {
	md, err := c.bq.Dataset(dataset).Metadata(ctx)
	if err != nil {
		return "", "", err
	}

	return md.Labels[label], md.ETag, nil
}

// WriteDatasetMetadataLabel writes a given label value into metadata associated with a given BigQuery
// dataset. If empty string passed as a label value, the label will be deleted.
func (c *BQClient) WriteDatasetMetadataLabel(ctx context.Context, dataset, label, value, etag string) error {
	update := bigquery.DatasetMetadataToUpdate{}
	if value == "" {
		update.DeleteLabel(label)
	} else {
		update.SetLabel(label, value)
	}

	_, err := c.bq.Dataset(dataset).Update(ctx, update, etag)
	return err
}

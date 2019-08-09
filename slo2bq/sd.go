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
	"log"
	"slo2bq/clients"
	"time"

	"github.com/golang/protobuf/ptypes/duration"
	googlepb "github.com/golang/protobuf/ptypes/timestamp"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
)

var timeNow = time.Now

// Stackdriver metric retention is 6 weeks (42 days), so we backfill up to 40
// days in the past.
var backfillDays = 40

// bqBatchSize is the number of BigQuery rows we will write at a time.
var bqBatchSize = 100

// daysAgoMidnightTimestamp returns a timestamp that corresponds to midnight of the day
// that was daysAgo days ago in a given location.
func daysAgoMidnightTimestamp(now time.Time, loc *time.Location, daysAgo int) time.Time {
	year, month, day := now.In(loc).AddDate(0, 0, -daysAgo).Date()
	return time.Date(year, month, day, 0, 0, 0, 0, loc)
}

// syncAllServices enumerates all services and their SLOs and syncs new data to BigQuery.
func syncAllServices(ctx context.Context, cfg *Config, sd clients.MetricClient, sloc clients.SLOClient, bq clients.BigQueryClient) error {
	existing, err := readBQMap(ctx, bq, cfg)
	if err != nil {
		return err
	}

	svcs, err := sloc.Services()
	if err != nil {
		return err
	}

	var rows []*clients.BQRow
	for _, svc := range svcs {
		slos, err := sloc.SLOs(svc)
		if err != nil {
			return err
		}
		for _, slo := range slos {
			res, err := newRecords(ctx, cfg, svc, slo, existing, sd)
			if err != nil {
				return err
			}
			rows = append(rows, res...)
			log.Printf("Got %d new records for Service '%s' SLO '%s'", len(res), svc.HumanName(), slo.HumanName())
			if len(rows) >= bqBatchSize {
				log.Printf("Flushing %d rows to BigQuery", len(rows))
				if err := bq.Put(ctx, cfg.Dataset, tableName, rows); err != nil {
					return err
				}
				rows = nil
			}
		}
	}
	return bq.Put(ctx, cfg.Dataset, tableName, rows)
}

// newRecords returns a list of BigQuery rows that need to be inserted to BigQuery for a given SLO.
func newRecords(ctx context.Context, cfg *Config, svc *clients.Service, slo *clients.SLO, existing bqMap, sd clients.MetricClient) ([]*clients.BQRow, error) {
	loc, err := time.LoadLocation(cfg.TimeZone)
	if err != nil {
		return nil, err
	}

	var rows []*clients.BQRow
	for daysAgo := 1; daysAgo <= backfillDays; daysAgo++ {
		start := daysAgoMidnightTimestamp(timeNow(), loc, daysAgo)
		end := daysAgoMidnightTimestamp(timeNow(), loc, daysAgo-1)
		date := start.Format("2006-01-02")

		row := clients.BQRow{
			Service: svc.HumanName(),
			SLO:     slo.HumanName(),
			Date:    date,
			Target:  slo.Goal,
		}
		if existing.Check(row.Service, row.SLO, row.Date) {
			continue
		}

		row.Good, row.Total, err = getGoodTotal(ctx, cfg, slo, start, end, sd)
		if err != nil {
			return nil, err
		}

		log.Printf("SLO data for %s on %s: %d good, %d total", slo.HumanName(), date, row.Good, row.Total)
		rows = append(rows, &row)
	}
	return rows, nil
}

// getGoodTotal returns two numbers corresponding to the cumulative count of good and total events for a given
// SLO between the two timestamps.
func getGoodTotal(ctx context.Context, cfg *Config, slo *clients.SLO, start, end time.Time, sd clients.MetricClient) (int64, int64, error) {
	req := &monitoringpb.ListTimeSeriesRequest{
		Name:   fmt.Sprintf("projects/%s", cfg.Project),
		Filter: fmt.Sprintf(`select_slo_counts("%s")`, slo.Name),
		Interval: &monitoringpb.TimeInterval{
			// `start` and `end` are guaranteed to be aligned to a second (since they come from
			// daysAgoMidnightTimestamp()), so there is no need to fill `Timestamp.Nanos`.
			StartTime: &googlepb.Timestamp{Seconds: start.Unix()},
			EndTime:   &googlepb.Timestamp{Seconds: end.Unix()},
		},
		// DELTA aligner and an alignment period covering the whole request interval should produce a response
		// with a single point containing the sum of all values within the request interval.
		Aggregation: &monitoringpb.Aggregation{
			AlignmentPeriod: &duration.Duration{
				Seconds: end.Unix() - start.Unix(),
			},
			PerSeriesAligner: monitoringpb.Aggregation_ALIGN_DELTA,
		},
	}

	series, err := sd.ListTimeSeries(ctx, req)
	if err != nil {
		return 0, 0, fmt.Errorf("ListTimeSeries (%v) error: %v", req, err)
	}

	if len(series) == 0 {
		log.Printf("Got 0 time series while querying '%s'", slo.Name)
		return 0, 0, nil
	} else if len(series) != 2 {
		return 0, 0, fmt.Errorf("expected to get 2 time series while querying %v; got %v", slo, series)
	}

	var good, total float64
	for _, s := range series {
		if len(s.Points) != 1 {
			return 0, 0, fmt.Errorf("expected to get 1 point in %v; got %v", s.GetMetric(), s.Points)
		}
		if s.ValueType != metricpb.MetricDescriptor_DOUBLE {
			return 0, 0, fmt.Errorf("unexpected value type in %v: %v", s.GetMetric(), s.ValueType)
		}
		value := s.Points[0].GetValue().GetDoubleValue()
		labels := s.GetMetric().GetLabels()
		if labels["event_type"] == "bad" {
			total += value
		} else if labels["event_type"] == "good" {
			total += value
			good += value
		} else {
			return 0, 0, fmt.Errorf("unexpected value of 'event_type' label in %v: %v", s.GetMetric(), labels["event_type"])
		}
	}
	return int64(good), int64(total), nil
}

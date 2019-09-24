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
			if slo.SLI.RequestBasedSLI != nil && slo.SLI.RequestBasedSLI.GoodTotalRatioSLI != nil {
				res, err := newRecords(ctx, cfg, svc, slo, existing, sd)
				if err != nil {
					return err
				}
				rows = append(rows, res...)
				log.Printf("Got %d new records for Service '%s' SLO '%s'", len(res), svc.HumanName(), slo.HumanName())
			} else {
				log.Printf("Service '%s' SLO '%s' is not based on a GoodTotalRatioSLI; skipping it", svc.HumanName(), slo.HumanName())
			}
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
	sli := slo.SLI.RequestBasedSLI.GoodTotalRatioSLI

	if sli.Good != "" && sli.Total != "" {
		good, err := getCounter(ctx, cfg, start, end, sli.Good, sd)
		if err != nil {
			return 0, 0, err
		}
		total, err := getCounter(ctx, cfg, start, end, sli.Total, sd)
		if err != nil {
			return 0, 0, err
		}
		return good, total, nil
	} else if sli.Good != "" && sli.Bad != "" {
		good, err := getCounter(ctx, cfg, start, end, sli.Good, sd)
		if err != nil {
			return 0, 0, err
		}
		bad, err := getCounter(ctx, cfg, start, end, sli.Bad, sd)
		if err != nil {
			return 0, 0, err
		}
		return good, good + bad, nil
	} else if sli.Bad != "" && sli.Total != "" {
		bad, err := getCounter(ctx, cfg, start, end, sli.Bad, sd)
		if err != nil {
			return 0, 0, err
		}
		total, err := getCounter(ctx, cfg, start, end, sli.Total, sd)
		if err != nil {
			return 0, 0, err
		}
		return total - bad, total, nil
	}
	return 0, 0, fmt.Errorf("expected 2 out of 3 filter expressions (good, bad, total) to be defined")
}

// getCounter returns the total sum of a Stackdriver cumulative metric (identified by the filter expression) between
// two timestamps.
func getCounter(ctx context.Context, cfg *Config, start, end time.Time, filter string, sd clients.MetricClient) (int64, error) {
	req := &monitoringpb.ListTimeSeriesRequest{
		Name:   fmt.Sprintf("projects/%s", cfg.Project),
		Filter: filter,
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
			CrossSeriesReducer: monitoringpb.Aggregation_REDUCE_SUM,
			PerSeriesAligner:   monitoringpb.Aggregation_ALIGN_DELTA,
		},
	}

	series, err := sd.ListTimeSeries(ctx, req)
	if err != nil {
		return 0, err
	}

	if len(series) == 0 {
		log.Printf("Got 0 time series while querying '%s'", filter)
		return 0, nil
	}

	if len(series) != 1 {
		return 0, fmt.Errorf("expected to get 1 time series while querying '%s'; got %q", filter, series)
	}

	if len(series[0].Points) != 1 {
		return 0, fmt.Errorf("expected to get 1 point while querying '%s'; got %q", filter, series[0].Points)
	}

	if series[0].ValueType == metricpb.MetricDescriptor_DOUBLE {
		return int64(series[0].Points[0].GetValue().GetDoubleValue()), nil
	} else if series[0].ValueType == metricpb.MetricDescriptor_INT64 {
		return series[0].Points[0].GetValue().GetInt64Value(), nil
	}
	return 0, fmt.Errorf("unexpected value type: %v", series[0].ValueType)
}

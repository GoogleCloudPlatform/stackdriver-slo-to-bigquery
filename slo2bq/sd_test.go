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
	"slo2bq/clients"
	"slo2bq/clients/mocks"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
)

func TestDaysAgoMidnightTimestamp(t *testing.T) {
	for _, tt := range []struct {
		name            string
		time            time.Time
		timeZone        string
		wantDayDuration time.Duration
	}{
		{"2015-05-01 in London is 24hr long", time.Date(2015, time.May, 1, 15, 0, 0, 0, time.UTC), "Europe/London", 24 * time.Hour},
		{"2015-03-29 in London is 23hr long", time.Date(2015, time.March, 29, 15, 0, 0, 0, time.UTC), "Europe/London", 23 * time.Hour},
		{"2015-10-25 in London is 25hr long", time.Date(2015, time.October, 25, 15, 0, 0, 0, time.UTC), "Europe/London", 25 * time.Hour},
		{"2015-10-04 in Lord Howe is 23h30m long", time.Date(2015, time.October, 4, 1, 0, 0, 0, time.UTC), "Australia/Lord_Howe", 23*time.Hour + 30*time.Minute},
	} {
		t.Run(tt.name, func(t *testing.T) {
			loc, err := time.LoadLocation(tt.timeZone)
			if err != nil {
				t.Errorf("LoadLocation() unexpected error: %v", err)
			}

			ts1 := daysAgoMidnightTimestamp(tt.time, loc, 0)
			ts2 := daysAgoMidnightTimestamp(tt.time, loc, -1)
			duration := ts2.Sub(ts1)

			if duration != tt.wantDayDuration {
				t.Errorf("expected duration of %v; got %v", tt.wantDayDuration, duration)
			}
		})
	}
}

func TestSyncAllServices(t *testing.T) {
	timeNow = func() time.Time { return time.Date(2015, time.May, 10, 15, 0, 0, 0, time.UTC) }
	backfillDays = 2
	bqBatchSize = 1
	defer func() { timeNow = time.Now }()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	bq := mocks.NewMockBigQueryClient(mockCtrl)
	bq.EXPECT().Query(gomock.Any(), gomock.Any()).Return([]*clients.BQRow{
		&clients.BQRow{Service: "svc1", SLO: "slo1", Date: "2015-05-08"},
		&clients.BQRow{Service: "svc1", SLO: "slo2", Date: "2015-05-09"},
	}, nil)

	sloc := mocks.NewMockSLOClient(mockCtrl)
	sloc.EXPECT().Services().Return([]*clients.Service{&clients.Service{Name: "s1", DisplayName: "svc1"}}, nil)
	sloc.EXPECT().SLOs(gomock.Any()).Return([]*clients.SLO{
		&clients.SLO{Name: "s1", DisplayName: "slo1", Goal: 0.99},
		&clients.SLO{Name: "s1", DisplayName: "slo2", Goal: 0.5},
	}, nil)

	sd := mocks.NewMockMetricClient(mockCtrl)
	// This will be called 2 times: once for slo1/2015-05-09, second time for slo2/2015-05-08.
	sd.EXPECT().ListTimeSeries(gomock.Any(), gomock.Any()).Times(2).Return([]*monitoringpb.TimeSeries{
		&monitoringpb.TimeSeries{
			Metric:    &metricpb.Metric{Labels: map[string]string{"event_type": "good"}},
			ValueType: metricpb.MetricDescriptor_DOUBLE, Points: []*monitoringpb.Point{
				&monitoringpb.Point{Value: &monitoringpb.TypedValue{Value: &monitoringpb.TypedValue_DoubleValue{DoubleValue: 100}}}}},
		&monitoringpb.TimeSeries{
			Metric:    &metricpb.Metric{Labels: map[string]string{"event_type": "bad"}},
			ValueType: metricpb.MetricDescriptor_DOUBLE, Points: []*monitoringpb.Point{
				&monitoringpb.Point{Value: &monitoringpb.TypedValue{Value: &monitoringpb.TypedValue_DoubleValue{DoubleValue: 11}}}}},
	}, nil)

	bq.EXPECT().Put(gomock.Any(), "datasetname", "data", []*clients.BQRow{
		&clients.BQRow{Service: "svc1", SLO: "slo1", Date: "2015-05-09", Target: 0.99, Good: 100, Total: 111},
	})
	bq.EXPECT().Put(gomock.Any(), "datasetname", "data", []*clients.BQRow{
		&clients.BQRow{Service: "svc1", SLO: "slo2", Date: "2015-05-08", Target: 0.5, Good: 100, Total: 111},
	})
	bq.EXPECT().Put(gomock.Any(), "datasetname", "data", nil) // final Put with no rows.

	cfg := &Config{Project: "project", Dataset: "datasetname", TimeZone: "Europe/London"}

	if err := syncAllServices(context.Background(), cfg, sd, sloc, bq); err != nil {
		t.Errorf("syncAllServices() unexpected error: %v", err)
	}
}

func TestSyncAllServicesErrors(t *testing.T) {
	for _, tt := range []struct {
		name        string
		queryErr    error
		servicesErr error
		slosErr     error
		sdErr       error
		putErr      error
		wantErr     string
	}{
		{name: "error querying BQ", queryErr: fmt.Errorf("myerror"), wantErr: "myerror"},
		{name: "error listing services", servicesErr: fmt.Errorf("myerror"), wantErr: "myerror"},
		{name: "error listing SLOs", slosErr: fmt.Errorf("myerror"), wantErr: "myerror"},
		{name: "error querying Stackdriver", sdErr: fmt.Errorf("myerror"), wantErr: "myerror"},
		{name: "error writing to BQ", putErr: fmt.Errorf("myerror"), wantErr: "myerror"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			bq := mocks.NewMockBigQueryClient(mockCtrl)
			bq.EXPECT().Query(gomock.Any(), gomock.Any()).Return([]*clients.BQRow{}, tt.queryErr)
			bq.EXPECT().Put(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(tt.putErr)

			sloc := mocks.NewMockSLOClient(mockCtrl)
			sloc.EXPECT().Services().AnyTimes().Return([]*clients.Service{&clients.Service{Name: "s1", DisplayName: "svc1"}}, tt.servicesErr)
			sloc.EXPECT().SLOs(gomock.Any()).AnyTimes().Return([]*clients.SLO{
				&clients.SLO{Name: "s1", DisplayName: "slo1", Goal: 0.99}}, tt.slosErr)

			sd := mocks.NewMockMetricClient(mockCtrl)
			sd.EXPECT().ListTimeSeries(gomock.Any(), gomock.Any()).AnyTimes().Return([]*monitoringpb.TimeSeries{
				&monitoringpb.TimeSeries{
					Metric:    &metricpb.Metric{Labels: map[string]string{"event_type": "good"}},
					ValueType: metricpb.MetricDescriptor_DOUBLE, Points: []*monitoringpb.Point{
						&monitoringpb.Point{Value: &monitoringpb.TypedValue{Value: &monitoringpb.TypedValue_DoubleValue{DoubleValue: 100}}}}},
				&monitoringpb.TimeSeries{
					Metric:    &metricpb.Metric{Labels: map[string]string{"event_type": "bad"}},
					ValueType: metricpb.MetricDescriptor_DOUBLE, Points: []*monitoringpb.Point{
						&monitoringpb.Point{Value: &monitoringpb.TypedValue{Value: &monitoringpb.TypedValue_DoubleValue{DoubleValue: 11}}}}},
			}, tt.sdErr)

			cfg := &Config{Project: "project", Dataset: "datasetname", TimeZone: "Europe/London"}
			err := syncAllServices(context.Background(), cfg, sd, sloc, bq)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("syncAllServices() expected error to contain '%s'; got %v", tt.wantErr, err)
			}
		})
	}
}

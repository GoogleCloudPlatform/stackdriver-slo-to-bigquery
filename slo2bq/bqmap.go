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
	"time"
)

// bqMap can be used to easily check whether data for a given Service+SLO+Date exists in BigQuery.
type bqMap map[bqMapKey]bool
type bqMapKey struct{ Service, SLO, Date string }

// Add adds a service+slo+date to the map.
func (b bqMap) Add(service, slo, date string) {
	b[bqMapKey{service, slo, date}] = true
}

// Check returns whether a given service+slo+date exist.
func (b bqMap) Check(service, slo, date string) bool {
	return b[bqMapKey{service, slo, date}]
}

// readBqMap reads recent SLO data from BigQuery and returns a bqMap.
func readBQMap(ctx context.Context, client clients.BigQueryClient, cfg *Config) (bqMap, error) {
	loc, err := time.LoadLocation(cfg.TimeZone)
	if err != nil {
		return nil, err
	}
	startDate := daysAgoMidnightTimestamp(time.Now(), loc, backfillDays).Format("2006-01-02")

	q := fmt.Sprintf(
		"SELECT service, slo, FORMAT_DATE('%%F', `date`) as date FROM `%s.%s` WHERE date >= '%s';",
		cfg.Dataset, tableName, startDate)
	rows, err := client.Query(ctx, q)
	if err != nil {
		return nil, err
	}

	result := make(bqMap)
	for _, row := range rows {
		if row.Service == "" || row.SLO == "" || row.Date == "" {
			return nil, fmt.Errorf("Expected Service, SLO and Date to be set in BQ row; got %v", row)
		}
		result.Add(row.Service, row.SLO, row.Date)
	}
	return result, nil
}

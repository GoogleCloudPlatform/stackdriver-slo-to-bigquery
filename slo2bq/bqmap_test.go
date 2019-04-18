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

	"github.com/golang/mock/gomock"
)

func TestReadBQMap(t *testing.T) {
	for _, tt := range []struct {
		name     string
		rows     []*clients.BQRow
		wantLen  int
		wantKeys []*bqMapKey
	}{
		{"one row", []*clients.BQRow{&clients.BQRow{Service: "svc1", SLO: "slo1", Date: "2015-01-01"}}, 1,
			[]*bqMapKey{&bqMapKey{"svc1", "slo1", "2015-01-01"}}},
		{"two rows", []*clients.BQRow{
			&clients.BQRow{Service: "svc1", SLO: "slo1", Date: "2015-01-01"},
			&clients.BQRow{Service: "svc2", SLO: "slo2", Date: "2015-01-01"},
		}, 2, []*bqMapKey{&bqMapKey{"svc1", "slo1", "2015-01-01"}, &bqMapKey{"svc2", "slo2", "2015-01-01"}}},
		{"no rows", []*clients.BQRow{}, 0, []*bqMapKey{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			mock := mocks.NewMockBigQueryClient(mockCtrl)
			mock.EXPECT().Query(gomock.Any(), gomock.Any()).Return(tt.rows, nil)

			m, err := readBQMap(context.Background(), mock, &Config{})
			if err != nil {
				t.Errorf("readBQMap() unexpected error: %v", err)
			}
			if len(m) != tt.wantLen {
				t.Errorf("unexpected size of BQMap: %d; want %d", len(m), tt.wantLen)
			}
			for _, r := range tt.wantKeys {
				if !m.Check(r.Service, r.SLO, r.Date) {
					t.Errorf("expected bqMap to have key %q", r)
				}
			}
		})
	}
}

func TestReadBQMapErrors(t *testing.T) {
	for _, tt := range []struct {
		name    string
		rows    []*clients.BQRow
		err     error
		wantErr string
	}{
		{"bigquery error", nil, fmt.Errorf("error from bq"), "error from bq"},
		{"empty Service", []*clients.BQRow{&clients.BQRow{Service: "", SLO: "slo2", Date: "2015-01-01"}}, nil, "Expected Service"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			mock := mocks.NewMockBigQueryClient(mockCtrl)
			mock.EXPECT().Query(gomock.Any(), gomock.Any()).Return(tt.rows, tt.err)

			_, err := readBQMap(context.Background(), mock, &Config{})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("readBQMap() expected error to contain '%s'; got %v", tt.wantErr, err)
			}
		})
	}

}

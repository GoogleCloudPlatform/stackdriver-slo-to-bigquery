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
	"slo2bq/clients/mocks"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
)

func TestBQLease(t *testing.T) {
	for _, tt := range []struct {
		name          string
		existingLease string
	}{
		{"no lease exists", ""},
		{"lease in the past", "123"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			mock := mocks.NewMockBigQueryClient(mockCtrl)
			mock.EXPECT().ReadDatasetMetadataLabel(gomock.Any(), "dsname", bqLeaseLabelName).Return(tt.existingLease, "etag1", nil)
			mock.EXPECT().WriteDatasetMetadataLabel(gomock.Any(), "dsname", bqLeaseLabelName, "1337", "etag1").Return(nil)

			l, err := newBqLease(ctx, mock, "dsname", time.Unix(1337, 0))
			if err != nil {
				t.Errorf("newBqLease() unexpected error: %v", err)
			}

			mock.EXPECT().WriteDatasetMetadataLabel(gomock.Any(), "dsname", bqLeaseLabelName, "", "").Return(nil)
			l.Close(ctx)
		})
	}
}

func TestBQLeaseErrors(t *testing.T) {
	for _, tt := range []struct {
		name          string
		existingLease string
		readLabelErr  error
		writeLabelErr error
		wantErr       string
	}{
		{"lease in the future", strconv.FormatInt(time.Now().Add(time.Minute).Unix(), 10), nil, nil, "lease is still valid"},
		{"incorrect lease value", "bogus", nil, nil, "Could not parse BQ lease expiration"},
		{"reading metadata returns error", "123", fmt.Errorf("error1"), nil, "error1"},
		{"writing metadata returns error", "123", nil, fmt.Errorf("error2"), "error2"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			mock := mocks.NewMockBigQueryClient(mockCtrl)
			mock.EXPECT().ReadDatasetMetadataLabel(gomock.Any(), "dsname", bqLeaseLabelName).Return(tt.existingLease, "etag1", tt.readLabelErr)
			mock.EXPECT().WriteDatasetMetadataLabel(gomock.Any(), "dsname", bqLeaseLabelName, "1337", "etag1").AnyTimes().Return(tt.writeLabelErr)

			_, err := newBqLease(ctx, mock, "dsname", time.Unix(1337, 0))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("newBqLease() expected error to contain '%s'; got %v", tt.wantErr, err)
			}
		})
	}
}

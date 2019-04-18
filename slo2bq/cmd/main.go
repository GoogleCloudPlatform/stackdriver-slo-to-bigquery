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

package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"slo2bq"
	"time"
)

func main() {
	project := flag.String("project", "", "Cloud project name")
	dataset := flag.String("dataset", "", "Name of the BigQuery dataset to use")
	tz := flag.String("tz", "Europe/London", "Timezone to use to create daily rollups")
	flag.Parse()

	_, err := time.LoadLocation(*tz)
	if err != nil {
		log.Fatalf("error parsing --tz: %s\n", err)
	}

	if *project == "" || *dataset == "" {
		log.Fatalln("--project and --dataset are required")
	}

	j, err := json.Marshal(&slo2bq.Config{
		Project:  *project,
		Dataset:  *dataset,
		TimeZone: *tz,
	})
	if err != nil {
		log.Fatalf("error marshalling json: %v\n", err)
	}

	if err := slo2bq.SyncSloPerformance(context.Background(), slo2bq.PubSubMessage{Data: j}); err != nil {
		log.Fatalf("ERROR: %v\n", err)
	}
}

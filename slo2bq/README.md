# slo2bq

This is a GCF function (written in Go) that exports SLO performance data from
Stackdriver Service Monitoring to BigQuery.

## Running locally

`go run cmd/main.go --project $PROJECT_NAME --dataset slo_reporting --tz Europe/London`

You might need to run `gcloud auth application-default login` to generate default credentials.

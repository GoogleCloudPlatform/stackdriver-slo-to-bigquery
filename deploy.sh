#!/bin/bash
#
# Copyright 2019 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e -u -o pipefail

# Default name of the BigQuery dataset.
dataset="slo_reporting"

# Name of the Pubsub topic that will trigger the GCF function.
readonly TOPIC="slo2bq-trigger"

# Name of the Cloud Scheduler job that will regularly post a message to Pubsub.
readonly CRONJOB="schedule-slo2bq-trigger"

raise() {
    echo "ERROR: $*" >&2
    exit 1
}

usage() {
    echo "
$0 [--dataset <dataset>] --project <project_name> --timezone <timezone>

This script configures GCP resources nessesary for SLO Reporting based on
data in the Stackdriver Service Monitoring. The following resources will
be created:

1. BigQuery dataset and tables;
2. GCF Function used to update data in BigQuery;
3. Cloud Scheduler job that will trigger GCF function regularly.

--project <project_name>
  Cloud project that will be used to configure all resources. This project is
  expected to have Stackdriver Service Monitoring enabled with services and
  SLOs defined. This parameter is required.

--timezone <timezone>
  Timezone that will be used to create daily aggregates in BigQuery used for
  SLO reports. Should be one of the timezones supported by Zoneinfo, e.g.
  'Europe/London'. This parameter is required.

--dataset <dataset>
  BigQuery dataset to use. The default is '$dataset'.
" >&2
    exit 2
}

parse_args() {
    while (( $# > 0 )); do
        arg="$1"
        shift
        case "$arg" in
            (--project)
                [[ -n "${1:-}" ]] || raise "--project requires a value"
                project="$1"
                shift
                ;;
            (--timezone)
                [[ -n "${1:-}" ]] || raise "--timezone requires a value"
                timezone="$1"
                shift
                ;;
            (--dataset)
                [[ -n "${1:-}" ]] || raise "--dataset requires a value"
                dataset="$1"
                shift
                ;;
            (*)
                usage
                ;;
        esac
    done
}

preflight_checks() {
    if [[ -d "/usr/share/zoneinfo/" ]]; then
      [[ -e "/usr/share/zoneinfo/${timezone}" ]] || raise \
        "'${timezone}' does not seem like a valid timezone"
    fi

    for p in bq_schema.json slo2bq; do
        [[ -r "${p}" ]] || raise "${p} not found." \
            "Please run this script from the repository root"
    done

    for cmd in gcloud bq; do
        hash $cmd 2> /dev/null || raise "${cmd} not found." \
            "Please install Google Cloud SDK: https://cloud.google.com/sdk/install"
    done

    gcloud projects describe "${project}" > /dev/null
}

make_bigquery() {
    echo "Enabling BigQuery API..."
    gcloud --project "${project}" services enable bigquery

    if ! bq --project_id "${project}" show "${dataset}" > /dev/null; then
        echo "Creating BigQuery dataset ${dataset}..."
        bq --project_id "${project}" mk --dataset \
            --description "SLO Reporting data" "${dataset}"
    fi

    local datatable="${dataset}.data"
    local desc="SLO performance daily aggregates imported by slo2bq"
    if bq --project_id "${project}" show "${datatable}" > /dev/null; then
        echo "Updating schema for BigQuery table ${datatable}..."
        bq --project_id "${project}" update --description "${desc}" \
            "${datatable}" bq_schema.json
    else
        echo "Creating BigQuery table ${datatable}..."
        bq --project_id "${project}" mk --table --description "${desc}" \
            "${datatable}" bq_schema.json
    fi

    for suf in monthly quarterly; do
        local view="${dataset}.${suf}"
        local sql="$(sed -e s/__DATA/${project}.${datatable}/ < bq_view.${suf})"
        local desc="SLO performance data with ${suf} error budget"
        if bq --project_id "${project}" show "${view}" > /dev/null; then
            echo "Updating BigQuery view ${view}..."
            bq --project_id "${project}" update --use_legacy_sql=false \
                --view "${sql}" --description "${desc}" "${view}"
        else
            echo "Creating BigQuery view ${view}..."
            bq --project_id "${project}" mk --use_legacy_sql=false \
                --view "${sql}" --description "${desc}" "${view}"
        fi
    done
}

make_cloud_scheduler() {
    if gcloud --project "${project}" beta \
            scheduler jobs describe "${CRONJOB}" &> /dev/null; then
        echo "Deleting Cloud Scheduler job ${CRONJOB}..."
        gcloud -q --project "${project}" beta scheduler jobs delete ${CRONJOB}
    fi
    echo "Creating Cloud Scheduler job ${CRONJOB}..."
    local message='{"Project":"'${project}'","Dataset":"'${dataset}'",
        "TimeZone":"'${timezone}'"}'
    gcloud --project "${project}" beta scheduler jobs create \
        pubsub ${CRONJOB} --schedule "30 */8 * * *" \
        --description "Trigger slo2bq function" \
        --topic "projects/${project}/topics/${TOPIC}" \
        --message-body "${message}"
}

deploy_function() {
    gcloud functions deploy slo2bq --runtime go111 \
        --trigger-topic "${TOPIC}" --project "${project}" --timeout 540s \
        --entry-point "SyncSloPerformance" --source "./slo2bq"
}

parse_args "$@"
preflight_checks
make_bigquery
make_cloud_scheduler
deploy_function
echo "Finished."

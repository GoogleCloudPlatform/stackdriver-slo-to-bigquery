# Stackdriver SLO Reporting

This repository contains tooling to enable long-term reporting on SLOs defined
in Stackdriver Service Monitoring.

You will find the following:

* `slo2bq/` - source code for the GCF function that regularly exports SLO
  performance data from Stackdriver to BigQuery.
* `bq_schema.json` - BigQuery schema for the table that will store long-term
  SLO performance data.
* `bq_view.monthly`, `bq_view.quarterly` - SQL definitions for BigQuery views
  that provide monthly and quarterly error budget data.
* `deploy.sh` - a script that can be used to deploy all resources to your
  Cloud project. It will create BigQuery dataset and tables, deploy the slo2bq
  GCF function and create a Cloud Scheduler job that will regularly trigger
  the function.

# Support

This is not an officially supported Google product.
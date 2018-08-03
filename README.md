# prometheus-biz-ops-service-discovery

![CircleCI ](https://img.shields.io/circleci/project/github/Financial-Times/prometheus-biz-ops-service-discovery/master.svg)

🕯️ Service discovery for the O&R Prometheus system.

Generates configuration for use by [the Prometheus file-based service discovery](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#%3Cfile_sd_config%3E).

We run this process in the same ECS cluster as Prometheus, writing the configuration file to EFS.

Ensure you set the `BIZ_OPS_API_KEY` environment variable. You can get a key by running `/get-developer-apikey Biz Ops API p` in slack.

Prometheus then loads this file with the following configuration, watching and updating on any changes.

```yaml
- job_name: health_check
  scheme: https
  metrics_path: /scrape
  scrape_interval: 60s
  file_sd_configs:
    - files:
      - /prometheus/service-discovery/health-check-service-discovery.json
  relabel_configs:
    - source_labels: [__address__]
      target_label: __param_endpoint
    - source_labels: [__address__]
      target_label: instance
    - target_label: __address__
      replacement: prometheus-health-check-exporter.in.ft.com
```

Here's an example of what `health-check-service-discovery.json` might look like.

```json
[
  {
    "targets": [
        "https://totally-a-live-system.in.ft.com/__health",
        ...
    ],
    "labels": {
      "observe": "yes"
    }
  },
  {
    "targets": [
        "https://no-wait-i-am-not-ready-system.in.ft.com/__health",
        ...
    ],
    "labels": {
      "observe": "no"
    }
  }
]
```

## Development

Make sure you have an API key for the bizops API (you can get one by running `/get-developer-apikey Biz Ops API p` in slack).

To build and run a docker image of the project, you'll need [make](https://www.gnu.org/software/make/) and [docker](https://www.docker.com/) to be installed. (Everything else gets installed inside the image)
Run:

-   `make docker`
-   `docker run --env BIZ_OPS_API_KEY=keygoeshere idofimagefrompreviouscommand`

To build and run the go binary in isolation, you'll need [make](https://www.gnu.org/software/make/), [golang](https://golang.org/) and [dep](https://github.com/golang/dep) installed AND to have a key for the bizops API.
Run:

-   `dep ensure`
-   `make build`
-   `BIZ_OPS_API_KEY=keygoeshere ./biz-ops-service-discovery`

When making changes to the code, the simplest way to see your changes is to run:
`make build && BIZ_OPS_API_KEY=keygoeshere ./biz-ops-service-discovery -v -d .`

### CircleCI

Ensure the following variables are set in the CircleCI project:

-   `BIZ_OPS_API_KEY`

Other variables are pulled from a shared CircleCI [context](https://circleci.com/docs/2.0/contexts/).

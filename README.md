# prometheus-biz-ops-service-discovery

![CircleCI ](https://img.shields.io/circleci/project/github/Financial-Times/prometheus-biz-ops-service-discovery/master.svg)

🕯️ Service discovery for the O&R Prometheus system.

Generates configuration for use by [the Prometheus file-based service discovery](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#%3Cfile_sd_config%3E).

We run this process in the same ECS cluster as Prometheus, writing the configuration file to EFS.

Ensure you set the `BIZ_OPS_API_KEY` environment variable (see [Biz-Ops API](https://github.com/Financial-Times/biz-ops-api) for details).

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

Here's an example of what `health-check-service-discovery.json` might look like. The `observe` label is set based on the `isLive` field in Biz Ops, `isLive: true` maps to `observe="yes"`.

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

Make sure you have an API key for the Biz-Ops API (see [Biz-Ops API](https://github.com/Financial-Times/biz-ops-api) for details).

To build and run a docker image of the project, you'll need [make](https://www.gnu.org/software/make/) and [docker](https://www.docker.com/) to be installed. (Everything else gets installed inside the image)
To build an image, and run the service, run:

```shell
make build
BIZ_OPS_API_KEY=key_goes_here make run
```

this should generate a service discovery file in the `out` directory.

### CircleCI

Ensure the following variables are set in the CircleCI project:

-   `BIZ_OPS_API_KEY`

Other variables are pulled from a shared CircleCI [context](https://circleci.com/docs/2.0/contexts/).

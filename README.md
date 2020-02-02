# Atlassian Instance Health Exporter for Prometheus

[![Go Report Card](https://goreportcard.com/badge/github.com/polarisalpha/atlassian_instance_health_exporter)](https://goreportcard.com/report/github.com/polarisalpha/atlassian_instance_health_exporter)

Atlassian Instance health is used to check your application for known issues.

References

* [https://confluence.atlassian.com/support/support-tools-plugin-790796813.html](https://confluence.atlassian.com/support/support-tools-plugin-790796813.html)
* [https://confluence.atlassian.com/support/instance-health-790796828.html](https://confluence.atlassian.com/support/instance-health-790796828.html)
* [https://confluence.atlassian.com/jirakb/how-to-retrieve-health-check-results-using-rest-api-867195158.html](https://confluence.atlassian.com/jirakb/how-to-retrieve-health-check-results-using-rest-api-867195158.html)

## Label Usage

Have dropped `time` because of cardinality.

`isHealthy` is used for the metric value (bool to float)

Dropped `healthy` as it matches `isHealthy`

## Docker Build Example

```none
docker build . -t atlassian_instance_health_exporter
```

## Docker Run Example

List Help

```none
docker run -it --rm atlassian_instance_health_exporter -help
```

Simple run

```none
docker run -it --rm -p 9998:9998 atlassian_instance_health_exporter -app.token='' -app.fqdn="confluence.domain.com"
```

Run with difference port

```none
docker run -it --rm -p 6060:6060 atlassian_instance_health_exporter -app.token='' -app.fqdn="jira.domain.com" -svc.port=6060
```

Run with debug and color logrus

```none
docker run -it --rm -p 9998:9998 atlassian_instance_health_exporter -app.token='' -app.fqdn="confluence.domain.com" -debug -enable-color-logs
```

## Confluence or Jira Curl Endpoint Example

```none
curl -u 'username:password' https://<jira-baseurl>/rest/troubleshooting/1.0/check/
```

## Response JSON Example

```none
...
    {
      "id": 0,
      "completeKey": "com.atlassian.jira.plugins.jira-healthcheck-plugin:eolHealthCheck",
      "name": "End of Life",
      "description": "Checks if the running version of JIRA is approaching, or has reached End of Life.",
      "isHealthy": true,
      "failureReason": "JIRA version 7.3.x has not reached End of Life. This version will reach End of Life in 722 days.",
      "application": "JIRA",
      "time": 1484054268591,
      "severity": "undefined",
      "documentation": "https://confluence.atlassian.com/x/HjnRLg",
      "tag": "Supported Platforms",
      "healthy": true
    },
...
```

## Prometheus Job

```none
- job_name: "atlassian_instance_health_exporter"
  static_configs:
  - targets:
    - 'host.domain.com:9998'
```

## Troubleshooting

If you receive a 403, most likely the account is not a Confluence or Jira Administrator.

## References

Thank you everyone that writes code and docs!

* [https://golang.org/](https://golang.org/)
* [https://rsmitty.github.io/Prometheus-Exporters/](https://rsmitty.github.io/Prometheus-Exporters/)
* [https://prometheus.io/](https://prometheus.io/)
* [https://github.com/Sirupsen/logrus](https://github.com/Sirupsen/logrus)
* [https://www.atlassian.com/](https://www.atlassian.com/)

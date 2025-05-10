# Prometheus Telnet Exporter

This tool exposes metrics from remote hosts for [prometheus](https://github.com/prometheus/prometheus) by running commands remotely via Telnet and regex-matching the stdout. Each host is processed in parallel.

> **Note** Only basic Telnet functionality is implemented.

> **Note** Storing credentials on a networked computer for another computer causes security implications.

Implementation is based on [ssh-exporter](https://gitlab.com/wobcom/ssh-exporter).

## Command line options
```
NAME:
   telnet-exporter - Scrape any metric from remote systems via Telnet

USAGE:
   telnet-exporter [global options]

GLOBAL OPTIONS:
   --listen-address string, -l string  Address to listen on (default: "[::]:9342") [$LISTEN_ADDRESS]
   --metrics-path string               Path under which to expose metrics (default: "/metrics") [$METRICS_PATH]
   --config-file string, -c string     Configuration file (default: "telnet-exporter.yml") [$CONFIG_FILE]
   --log-level string                  Log level (default: "info") [$LOG_LEVEL]
   --help, -h                          show help
```

## Example configuration

```yaml
---

hosts:
  192.168.0.1:
    username: monitoring
    password: example
    commands: &main_commands
      - command: uptime
        metrics:
          load_avg_1min:
            help: Load average last 1 min 
            regex: "load average:\\s+(\\d+\\.\\d+)"
            labels:
              example: bar 
          load_avg_5min:
            regex: "load average:\\s+\\d+\\.\\d+,\\s+(\\d+\\.\\d+)"
  192.168.0.2:
    username: user
    password: secret
    commands: *main_commands
```

results in the following metrics being reported

```
load_avg_1min{command="uptime",example="bar",target="192.168.0.1"} 23
load_avg_1min{command="uptime",example="bar",target="192.168.0.2"} 23
load_avg_5min{command="uptime",target="192.168.0.1"} 42
load_avg_5min{command="uptime",target="192.168.0.2"} 42
telnet_exporter_up{target="192.168.0.1"} 1
telnet_exporter_up{target="192.168.0.2"} 1
```

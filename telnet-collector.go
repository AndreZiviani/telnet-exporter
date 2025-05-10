package main

import (
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	upDesc *prometheus.Desc
)

// TelnetCollector collects metrics by regex matching the stdout of commands executed remotely via Telnet
type TelnetCollector struct {
	hosts []*HostConfig
}

func newTelnetCollector(hosts []*HostConfig) *TelnetCollector {
	return &TelnetCollector{
		hosts: hosts,
	}
}

func init() {
	upDesc = prometheus.NewDesc("telnet_exporter_up", "1 if an Telnet connection can be established", []string{"target"}, nil)
}

// Describe implements the collector.Collector interface's Describe function
func (s *TelnetCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- upDesc
	for _, host := range s.hosts {
		for _, command := range host.Commands {
			for metricName, metric := range command.Metrics {
				staticLabels := map[string]string{
					"target":  host.HostName,
					"command": command.Command,
				}
				for labelName, labelValue := range metric.Labels {
					staticLabels[labelName] = labelValue
				}
				dynamicLabels := metric.DynamicLabels
				if metric.ValueAsLabel != "" {
					dynamicLabels = append(dynamicLabels, metric.ValueAsLabel)
				}
				metric.Desc = prometheus.NewDesc(metricName, metric.Help, dynamicLabels, staticLabels)
				ch <- metric.Desc
			}
		}
	}
}

// Collect implements the collector.Collector interface's Collect function
func (s *TelnetCollector) Collect(ch chan<- prometheus.Metric) {
	wg := &sync.WaitGroup{}
	wg.Add(len(s.hosts))
	Logger.Debug().Int("hosts", len(s.hosts)).Msg("Starting collection for hosts")
	for _, host := range s.hosts {
		host.Mutex.Lock()
		defer host.Mutex.Unlock()
		go s.collectForHost(ch, wg, host)
	}
	wg.Wait()
}

func (s *TelnetCollector) collectForHost(ch chan<- prometheus.Metric, wg *sync.WaitGroup, host *HostConfig) {
	defer wg.Done()
	now := time.Now()

	telnetClient, err := s.connect(host)
	if err != nil {
		Logger.Error().Err(err).Str("host", host.HostName).Msg("Failed to connect to host")
		ch <- prometheus.MustNewConstMetric(upDesc, prometheus.GaugeValue, 0, host.HostName)
		return
	}
	defer telnetClient.Close()

	ch <- prometheus.MustNewConstMetric(upDesc, prometheus.GaugeValue, 1, host.HostName)

	for _, command := range host.Commands {
		s.collectForCommand(ch, telnetClient, host, command)
	}

	Logger.Debug().Str("host", host.HostName).Int("duration", int(time.Since(now).Milliseconds())).Msg("Finished collection for host")
}

func (s *TelnetCollector) collectForCommand(ch chan<- prometheus.Metric, telnetClient net.Conn, host *HostConfig, command *CommandConfig) {
	output, err := sendCommand(telnetClient, command.Command, host.Prompt)
	if err != nil {
		Logger.Error().Err(err).Str("host", host.HostName).Str("command", command.Command).Msg("Failed to send command")
		return
	}

	Logger.Debug().Str("host", host.HostName).Str("command", command.Command).Str("output", output).Msg("Received command output")

	for _, metric := range command.Metrics {
		matches := metric.RegexCompiled.FindStringSubmatch(output)
		if matches == nil {
			continue
		}

		if metric.DynamicLabels == nil {
			createMetric(ch, metric, matches[1])
			continue
		}

		dynamicLabelValues := []string{}
		for _, dynamicLabelName := range metric.DynamicLabels {
			index := metric.RegexCompiled.SubexpIndex(dynamicLabelName)
			var dynamicLabelValue string
			if index < 0 {
				dynamicLabelValue = "!! match group not found !!"
			} else {
				dynamicLabelValue = matches[index]
			}
			dynamicLabelValues = append(dynamicLabelValues, dynamicLabelValue)
		}

		valueIndex := metric.RegexCompiled.SubexpIndex("value")
		if valueIndex < 0 {
			Logger.Error().Str("host", host.HostName).Str("command", command.Command).Str("metric", metric.Desc.String()).Msg("Must define a value match group for metric value if using dynamic values")
			continue
		}

		createMetric(ch, metric, matches[valueIndex], dynamicLabelValues...)
	}
}

func createMetric(ch chan<- prometheus.Metric, metric *MetricConfig, value string, dynamicLabelValues ...string) {
	if metric.ValueAsLabel == "" {
		ch <- prometheus.MustNewConstMetric(metric.Desc, prometheus.GaugeValue, str2float64(value), dynamicLabelValues...)
		return
	} else if metric.ValueEnum == nil {
		ch <- prometheus.MustNewConstMetric(metric.Desc, prometheus.GaugeValue, 1, append(dynamicLabelValues, value)...)
	} else {
		for _, val := range metric.ValueEnum {
			ch <- prometheus.MustNewConstMetric(metric.Desc, prometheus.GaugeValue, boolToFloat64(val == value), append(dynamicLabelValues, val)...)
		}
	}
}

func (s *TelnetCollector) connect(host *HostConfig) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host.HostName, strconv.Itoa(host.Port)), time.Duration(defaultConnectTimeout)*time.Second)
	if err != nil {
		return nil, err
	}

	// Set a read timeout for the connection
	conn.SetReadDeadline(time.Now().Add(time.Duration(defaultCommandTimeout) * time.Second))

	// Wait for initial login/prompt
	output, err := readUntil(conn, []string{">", "$", "#", "login", "Password"}, 5*time.Second)
	if err != nil {
		conn.Close()
		return nil, err
	}

	Logger.Debug().Str("host", host.HostName).Str("output", output).Msg("Received initial output")

	// Check if the output contains a login prompt
	if strings.Contains(output, "login") || strings.Contains(output, "Password") {
		Logger.Debug().Str("host", host.HostName).Msg("Login prompt detected")

		// Send the username and password
		output, _ := sendCommand(conn, host.Username, "Password")
		Logger.Debug().Str("host", host.HostName).Str("output", output).Msg("Sent username")

		output, _ = sendCommand(conn, host.Password, "#")
		Logger.Debug().Str("host", host.HostName).Str("output", output).Msg("Sent password")
	}

	return conn, nil
}

func str2float64(str string) float64 {
	value, err := strconv.ParseFloat(str, 64)
	if err != nil {
		Logger.Error().Err(err).Str("value", str).Msg("Could not parse value as float")
		return 0
	}
	return value
}

func boolToFloat64(b bool) float64 {
	if b {
		return 1
	} else {
		return 0
	}
}

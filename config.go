package main

import (
	"fmt"
	"io"
	"maps"
	"regexp"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/yaml.v2"
)

const defaultConnectTimeout int = 5
const defaultCommandTimeout int = 20
const defaultPort int = 23

// Config stores the telnet-exporter's configuration
type Config struct {
	Hosts map[string]*HostConfig `yaml:"hosts,omitempty"`
}

// HostConfig stores host specifc configuration
type HostConfig struct {
	Mutex          sync.Mutex
	HostName       string
	Port           int               `yaml:"port,omitempty"`
	Username       string            `yaml:"username"`
	Password       string            `yaml:"password,omitempty"`
	ConnectTimeout int               `yaml:"connect_timeout,omitempty"`
	CommandTimeout int               `yaml:"command_timeout,omitempty"`
	Commands       []*CommandConfig  `yaml:"commands"`
	Labels         map[string]string `yaml:"labels,omitempty"`
	Prompt         string            `yaml:"prompt"`
}

// CommandConfig stores command specific configuration
type CommandConfig struct {
	Command string                   `yaml:"command"`
	Metrics map[string]*MetricConfig `yaml:"metrics"`
}

// MetricConfig stores metrics to extract by regex matching command's stdout
type MetricConfig struct {
	Regex         string `yaml:"regex"`
	RegexCompiled *regexp.Regexp
	Help          string            `yaml:"help,omitempty"`
	Labels        map[string]string `yaml:"labels,omitempty"`
	DynamicLabels []string          `yaml:"dynamic_labels,omitempty"`
	ValueAsLabel  string            `yaml:"value_as_label,omitempty"`
	ValueEnum     []string          `yaml:"value_enum,omitempty"`
	Desc          *prometheus.Desc
}

func newConfig() *Config {
	return &Config{
		Hosts: make(map[string]*HostConfig, 0),
	}
}

// ParseConfigurationFile reads a configuration from an io.Reader and returns either the Config or an error
func ParseConfigurationFile(reader io.Reader) (*Config, error) {
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	config := newConfig()
	err = yaml.Unmarshal(content, config)
	if err != nil {
		return nil, err
	}

	for hostName, host := range config.Hosts {
		host.HostName = hostName
		if host.ConnectTimeout == 0 {
			host.ConnectTimeout = defaultConnectTimeout
		}
		if host.CommandTimeout == 0 {
			host.CommandTimeout = defaultCommandTimeout
		}
		if host.Port == 0 {
			host.Port = defaultPort
		}
		if host.Prompt == "" {
			host.Prompt = "#"
		}

		for _, command := range host.Commands {
			for metricName, metric := range command.Metrics {
				expr, err := regexp.Compile(metric.Regex)
				if err != nil {
					return nil, fmt.Errorf("Could not compile regex (host='%s', command='%s', metric='%s') '%s'", hostName, command.Command, metricName, metric.Regex)
				}
				metric.RegexCompiled = expr
				if metric.Labels == nil {
					metric.Labels = host.Labels
				} else {
					maps.Copy(metric.Labels, host.Labels)
				}
			}
		}
	}
	return config, nil
}

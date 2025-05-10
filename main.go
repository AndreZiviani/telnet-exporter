package main

import (
	"bufio"
	"context"
	"log"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/urfave/cli/v3"
)

const version string = "1.0.0"

var (
	configuration *Config
)

func main() {
	initializeLogger()

	cmd := &cli.Command{
		Name:  "telnet-exporter",
		Usage: "Scrape any metric from remote systems via Telnet",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "listen-address",
				Aliases: []string{"l"},
				Sources: cli.EnvVars("LISTEN_ADDRESS"),
				Usage:   "Address to listen on",
				Value:   "[::]:9342",
			},
			&cli.StringFlag{
				Name:    "metrics-path",
				Sources: cli.EnvVars("METRICS_PATH"),
				Usage:   "Path under which to expose metrics",
				Value:   "/metrics",
			},
			&cli.StringFlag{
				Name:    "config-file",
				Aliases: []string{"c"},
				Sources: cli.EnvVars("CONFIG_FILE"),
				Usage:   "Configuration file",
				Value:   "telnet-exporter.yml",
			},
			&cli.StringFlag{
				Name:    "log-level",
				Sources: cli.EnvVars("LOG_LEVEL"),
				Usage:   "Log level",
				Value:   "info",
			},
		},
		Action: startServer,
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

func loadConfiguration(path string) error {
	Logger.Info().Str("config", path).Msg("Loading configuration")
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() {
		if err = file.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	reader := bufio.NewReader(file)
	configuration, err = ParseConfigurationFile(reader)
	if err != nil {
		return err
	}
	Logger.Info().Int("hosts", len(configuration.Hosts)).Msg("Loaded host(s) from configuration")
	return nil
}

func startServer(ctx context.Context, c *cli.Command) error {
	configFile := c.String("config-file")
	listenAddress := c.String("listen-address")
	metricsPath := c.String("metrics-path")

	loadConfiguration(configFile)

	Logger.Info().Str("version", version).Str("config-file", configFile).Str("metrics-path", metricsPath).Str("address", listenAddress).Msg("Starting telnet-exporter")
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
            <head><title>telnet-exporter (Version ` + version + `)</title></head>
            <body>
            <h1>telnet-exporter</h1>
            <p><a href="` + metricsPath + `">Metrics</a></p>
            </body>
            </html>`))
	})
	http.HandleFunc(metricsPath, handleMetricsRequest)

	return http.ListenAndServe(listenAddress, nil)
}

func handleMetricsRequest(w http.ResponseWriter, request *http.Request) {
	registry := prometheus.NewRegistry()

	var collector *TelnetCollector

	if target := request.URL.Query().Get("target"); target != "" {
		host, found := configuration.Hosts[target]
		if !found {
			http.Error(w, "Target not configured", 404)
			return
		}
		collector = newTelnetCollector([]*HostConfig{host})
	} else {
		hostList := []*HostConfig{}

		for _, host := range configuration.Hosts {
			hostList = append(hostList, host)
		}
		collector = newTelnetCollector(hostList)
	}

	registry.MustRegister(collector)
	promhttp.HandlerFor(registry,
		promhttp.HandlerOpts{
			ErrorHandling: promhttp.ContinueOnError,
		},
	).ServeHTTP(w, request)
}

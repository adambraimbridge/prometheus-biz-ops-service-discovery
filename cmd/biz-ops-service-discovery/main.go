package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/Financial-Times/prometheus-biz-ops-service-discovery/internal/api"
	"github.com/Financial-Times/prometheus-biz-ops-service-discovery/internal/server"
	"github.com/Financial-Times/prometheus-biz-ops-service-discovery/internal/servicediscovery"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh/terminal"
)

var serviceDiscoveryCount = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "service_discovery_writes_total",
		Help: "Number of service discovery file writes",
	},
)

var serviceDiscoveryFailuresCount = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "service_discovery_failures_total",
		Help: "Number of service discovery failures",
	},
)

var (
	directory        string
	tick             time.Duration
	verbose          bool
	port             int
	bizOpsAPIBaseUrl string
	bizOpsAPIKey     string
)

func doServiceDiscovery(bizopsDiscovery *servicediscovery.BizOps) {
	if err := bizopsDiscovery.Write(); err != nil {
		log.WithFields(log.Fields{
			"event": "ERROR_CONFIGURATION_WRITE",
			"err":   err,
		}).Error("Failed to write the configuration.")
		serviceDiscoveryFailuresCount.Inc()
	}
	serviceDiscoveryCount.Inc()
}

func main() {

	viper.AutomaticEnv()
	replacer := strings.NewReplacer("-", "_")
	viper.SetEnvKeyReplacer(replacer)

	pflag.IntP("port", "p", 8080, "The port to run the prometheus metrics server on.")
	pflag.StringP("directory", "d", "/etc/prometheus", "The directory configuration will be written to.")
	pflag.DurationP("tick", "t", time.Duration(60)*time.Second, "Duration between background refreshes of the configuration.")
	pflag.BoolP("verbose", "v", false, "Enable more detailed logging.")
	pflag.String("biz-ops-base-url", "https://api.ft.com/biz-ops", "The base url for the biz-ops API.")
	pflag.String("biz-ops-api-key", "", "The API key to access the biz-ops API")
	pflag.Parse()

	viper.BindPFlags(pflag.CommandLine)

	directory = viper.GetString("directory")
	tick = viper.GetDuration("tick")
	port = viper.GetInt("port")
	listenAddress := fmt.Sprintf(":%d", port)

	if viper.GetBool("verbose") {
		log.SetLevel(log.DebugLevel)
	}

	if !terminal.IsTerminal(int(os.Stdout.Fd())) {
		log.SetFormatter(&log.JSONFormatter{})
	}

	bizOpsAPIBaseUrl = viper.GetString("biz-ops-base-url")

	if _, err := url.ParseRequestURI(bizOpsAPIBaseUrl); err != nil {
		log.WithFields(log.Fields{
			"event": "INVALID_ENV_VAR",
			"value": bizOpsAPIBaseUrl,
		}).Fatal("The BIZ_OPS_BASE_URL config value was not a valid url.")
	}

	bizOpsAPIKey = viper.GetString("biz-ops-api-key")
	if !viper.IsSet("biz-ops-api-key") || bizOpsAPIKey == "" {
		log.WithFields(log.Fields{
			"event": "MISSING_ENV_VAR",
		}).Fatal("The BIZ_OPS_API_KEY environment variable must be set.")
	}

	server := server.Server(listenAddress)

	done := make(chan bool)

	go func() {
		quit := make(chan os.Signal, 1)

		signal.Notify(quit, syscall.SIGTERM)
		signal.Notify(quit, syscall.SIGINT)

		<-quit

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.WithFields(log.Fields{
				"event": "ERROR_STOPPING",
				"err":   err,
			}).Fatal("Could not gracefully stop Biz-Ops service discovery.")
		}

		close(done)
	}()

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithFields(log.Fields{
				"event":         "ERROR_STARTING",
				"listenAddress": listenAddress,
				"err":           err,
			}).Fatal("Could not listen at the specified address.")
		}

		log.WithFields(log.Fields{
			"event":         "STARTED",
			"listenAddress": listenAddress,
		}).Info("Metrics server is ready to handle requests.")
	}()

	go func() {
		prometheus.MustRegister(serviceDiscoveryCount)
		prometheus.MustRegister(serviceDiscoveryFailuresCount)

		bizopsDiscovery := servicediscovery.BizOps{
			Writer: servicediscovery.NewFileWriter(directory, nil),
			ApiClient: &api.BizOpsClient{
				Client: http.Client{
					Timeout: 10 * time.Second,
				},
				APIKey:  bizOpsAPIKey,
				BaseUrl: bizOpsAPIBaseUrl,
			},
		}

		log.WithFields(log.Fields{
			"event":     "STARTED",
			"port":      port,
			"directory": directory,
			"tick":      tick.Seconds(),
			"verbose":   verbose,
		}).Info("Biz-Ops service discovery is running.")

		doServiceDiscovery(&bizopsDiscovery)

		for range time.NewTicker(tick).C {
			doServiceDiscovery(&bizopsDiscovery)
		}
	}()

	<-done

	log.WithFields(log.Fields{
		"event": "STOPPED",
	}).Info("Biz-Ops service discovery has stopped.")
}

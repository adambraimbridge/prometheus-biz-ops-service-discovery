package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
)

var (
	directory    string
	tick         time.Duration
	verbose      bool
	bizOpsAPIKey string
)

const usage = `Usage of biz-ops-service-discovery:

--directory, -d DIRECTORY
  The directory configuration will be written to. (default "/etc/prometheus")

--tick, -t DURATION
  Duration between background refreshes of the configuration. (default 1m0s)

--verbose, -v
  Enable more detailed logging.`

type PrometheusConfiguration struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels,omitempty"`
}

type GraphQLResponse struct {
	Data struct {
		Healthchecks []HealthCheck `json:"Healthchecks"`
	} `json:"data"`
}
type HealthCheck struct {
	ID      string `json:"code"`
	URL     string `json:"url"`
	IsLive  bool   `json:"isLive"`
	Systems []struct {
		SystemCode string `json:"code"`
	} `json:"monitors"`
}

type APIGatewayResponse struct {
	Message string `json:"error"`
}

func writeConfiguration() error {
	var query = `{
	  Healthchecks {
	    code,
	    url,
	    isLive,
	    monitors {
	      code
	    }
	  }
	}
	`
	payload := map[string]string{"query": query}
	encodedPayload, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, "https://api.ft.com/biz-ops/graphql", bytes.NewBuffer(encodedPayload))
	req.Header.Add("X-Api-Key", bizOpsAPIKey)
	req.Header.Add("User-Agent", "prometheus-biz-ops-service-discovery")
	req.Header.Add("client-id", "prometheus-biz-ops-service-discovery")
	req.Header.Add("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {

		// If not a valid response from bizops, then it might be an error from the API Gateway
		var gatewayError APIGatewayResponse
		err = json.Unmarshal(body, &gatewayError)
		if err != nil {
			return errors.New("Received " + resp.Status + " from Biz Ops: " + string(body))
		}
		return errors.New("API Gateway Error: " + gatewayError.Message)
	}

	responsePayload := new(GraphQLResponse)
	err = json.Unmarshal(body, &responsePayload)
	if err != nil {
		return err
	}
	healthChecks := responsePayload.Data.Healthchecks

	configuration := make([]PrometheusConfiguration, 2)

	configuration[0].Labels = map[string]string{"observe": "yes"}
	configuration[1].Labels = map[string]string{"observe": "no"}

	for _, healthCheck := range healthChecks {
		// Check the URL is legit, ignore it on parse errors.
		_, err := url.Parse(healthCheck.URL)
		if err != nil {
			log.WithFields(log.Fields{
				"event": "ERROR_PARSING_HEALTH_CHECK_URL",
				"url":   healthCheck.URL,
				"err":   err,
			}).Error("Failed to parse a health check URL from the Biz Ops API.")
			continue
		}

		if healthCheck.IsLive {
			configuration[0].Targets = append(configuration[0].Targets, healthCheck.URL)
		} else {
			configuration[1].Targets = append(configuration[1].Targets, healthCheck.URL)
		}
	}

	configurationJSON, err := json.MarshalIndent(configuration, "", "  ")

	filename := path.Join(directory, "health-check-service-discovery.json")

	if _, err := os.Stat(directory); os.IsNotExist(err) {
		os.MkdirAll(directory, os.ModePerm)
	} else if err != nil {
		return err
	}

	if err := ioutil.WriteFile(filename, configurationJSON, 0644); err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"event":       "CONFIGURATION_UPDATED",
		"filename":    filename,
		"targetCount": len(healthChecks),
	}).Info("Health check targets have been updated.")

	return nil
}

func main() {
	flag.Usage = func() {
		fmt.Println(flag.CommandLine.Output(), usage)
	}

	flag.StringVar(&directory, "directory", "/etc/prometheus", "The directory configuration will be written to.")
	flag.StringVar(&directory, "d", "/etc/prometheus", "")
	flag.DurationVar(&tick, "tick", 60*time.Second, "Duration between background refreshes of the configuration.")
	flag.DurationVar(&tick, "t", 60*time.Second, "")
	flag.BoolVar(&verbose, "verbose", false, "Enable more detailed logging.")
	flag.BoolVar(&verbose, "v", false, "")
	flag.Parse()

	if verbose {
		log.SetLevel(log.DebugLevel)
	}

	if !terminal.IsTerminal(int(os.Stdout.Fd())) {
		log.SetFormatter(&log.JSONFormatter{})
	}

	var ok bool
	bizOpsAPIKey, ok = os.LookupEnv("BIZ_OPS_API_KEY")
	if !ok {
		log.WithFields(log.Fields{
			"event": "MISSING_ENV_VAR",
		}).Fatal("The BIZ_OPS_API_KEY environment variable must be set.")
	}

	done := make(chan bool)

	go func() {
		quit := make(chan os.Signal, 1)

		signal.Notify(quit, os.Interrupt)

		<-quit

		close(done)
	}()

	log.WithFields(log.Fields{
		"event": "STARTED",
	}).Info("Biz ops service discovery is running.")

	go func() {
		if err := writeConfiguration(); err != nil {
			log.WithFields(log.Fields{
				"event": "ERROR_CONFIGURATION_WRITE",
				"err":   err,
			}).Error("Failed to write the configuration.")
		}

		for range time.NewTicker(tick).C {
			if err := writeConfiguration(); err != nil {
				log.WithFields(log.Fields{
					"event": "ERROR_CONFIGURATION_WRITE",
					"err":   err,
				}).Error("Failed to write the configuration.")
			}
		}
	}()

	<-done

	log.WithFields(log.Fields{
		"event": "STOPPED",
	}).Info("Biz ops service discovery has stopped.")
}

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"strings"
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

type Endpoint struct {
	URL    string
	IsLive bool
}

func (e *Endpoint) UnmarshalJSON(data []byte) error {
	type Alias Endpoint
	raw := &struct {
		ID         string `json:"id"`
		Scheme     string `json:"protocol"`
		Base       string `json:"base"`
		Name       string `json:"name"`
		HealthPath string `json:"healthSuffix"`
		AboutPath  string `json:"aboutSuffix"`
		IsLive     string `json:"isLive"`
		*Alias
	}{
		Alias: (*Alias)(e),
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if raw.Base == "" {
		raw.Base = raw.ID
	}

	e.URL = fmt.Sprintf("%s://%s/%s", raw.Scheme, raw.Base, raw.HealthPath)
	e.IsLive = raw.IsLive == "True"

	return nil
}

func writeConfiguration() {
	req, _ := http.NewRequest(http.MethodGet, "https://api.ft.com/biz-ops/api/Endpoint", nil)
	req.Header.Add("X-Api-Key", bizOpsAPIKey)
	req.Header.Add("User-Agent", "prometheus-biz-ops-service-discovery")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "BIZ_OPS_API_ERROR",
			"err":   err,
		}).Error("Failed to fetch endpoints from the Biz Ops API.")
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "BIZ_OPS_API_ERROR",
			"err":   err,
		}).Error("Failed to read the response from the Biz Ops API.")
		return
	}

	var endpoints []Endpoint
	err = json.Unmarshal(body, &endpoints)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ENDPOINTS_PARSE_ERROR",
			"err":   err,
		}).Error("Failed to parse the endpoints from the Biz Ops API.")
		return
	}

	targets := make([]PrometheusConfiguration, 2)

	targets[0].Labels = map[string]string{"live": "true"}
	targets[1].Labels = map[string]string{"live": "false"}

	for _, e := range endpoints {
		url, err := url.Parse(e.URL)
		if err != nil {
			log.WithFields(log.Fields{
				"event": "ENDPOINTS_URL_PARSE_ERROR",
				"url":   e.URL,
				"err":   err,
			}).Error("Failed to parse an endpoints URL from the Biz Ops API.")
			continue
		}

		if !strings.HasSuffix(url.Path, "/__health") {
			log.WithFields(log.Fields{
				"event": "ENDPOINTS_URL_PARSE_ERROR",
				"url":   e.URL,
			}).Error("No /__health suffix defined on the endpoint.")
			continue
		}

		if e.IsLive {
			targets[0].Targets = append(targets[0].Targets, e.URL)
		} else {
			targets[1].Targets = append(targets[1].Targets, e.URL)
		}
	}

	targetsJSON, err := json.MarshalIndent(targets, "", "  ")

	filename := path.Join(directory, "health-check-service-discovery.json")

	if _, err := os.Stat(directory); os.IsNotExist(err) {
		os.MkdirAll(directory, os.ModePerm)
	}

	ioutil.WriteFile(filename, targetsJSON, 0644)

	log.WithFields(log.Fields{
		"event":    "CONFIGURATION_UPDATED",
		"filename": filename,
	}).Info("Health check targets have been updated.")
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
		writeConfiguration()

		for range time.NewTicker(tick).C {
			writeConfiguration()
		}
	}()

	<-done

	log.WithFields(log.Fields{
		"event": "STOPPED",
	}).Info("Biz ops service discovery has stopped.")
}

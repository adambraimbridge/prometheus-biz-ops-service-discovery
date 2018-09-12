package servicediscovery

import (
	"encoding/json"
	"errors"
	"io"
	"net/url"

	log "github.com/sirupsen/logrus"
)

type graphQlClient interface {
	Query(string, interface{}) error
}

type labels struct {
	System  string `json:"system,omitempty"`
	Observe string `json:"observe,omitempty"`
}

type prometheusConfiguration struct {
	Targets []string `json:"targets"`
	Labels  labels   `json:"labels"`
}

type GraphQLResponse struct {
	Data `json:"data"`
}

type Data struct {
	Healthchecks []Healthcheck `json:"Healthchecks"`
}

type Healthcheck struct {
	ID      string   `json:"code"`
	URL     string   `json:"url"`
	IsLive  bool     `json:"isLive"`
	Systems []System `json:"monitors"`
}

type System struct {
	SystemCode string `json:"code"`
}

type BizOps struct {
	Writer    io.Writer
	ApiClient graphQlClient
}

func (bizOps *BizOps) Write() error {
	var responsePayload GraphQLResponse
	err := bizOps.ApiClient.Query(`{
	  Healthchecks {
	    code,
	    url,
	    isLive,
	    monitors {
	      code
	    }
	  }
	}
	`, &responsePayload)

	if err != nil {
		return err
	}

	healthchecks := responsePayload.Data.Healthchecks

	configuration := make([]prometheusConfiguration, 0)

	var labelsToUrls = map[labels][]string{}
	// maintain an orderd slice of keys so iteration order is stable
	var labelsKeys = make([]labels, 0)

	if len(healthchecks) == 0 {
		err = errors.New("returned healthchecks were empty")
		log.WithFields(log.Fields{
			"event":        "CONFIGURATION_EMPTY_HEALTHCHECKS",
			"err":          err,
			"healthchecks": healthchecks,
		}).Error(err)
		return err
	}
	for _, healthcheck := range healthchecks {
		// check the URL is parseable, ignore it on parse errors.
		_, err := url.ParseRequestURI(healthcheck.URL)
		if err != nil {
			log.WithFields(log.Fields{
				"event": "ERROR_PARSING_HEALTH_CHECK_URL",
				"url":   healthcheck.URL,
				"err":   err,
			}).Error("Failed to parse a health check URL from the Biz Ops API.")
			continue
		}

		if len(healthcheck.Systems) == 0 {
			healthcheck.Systems = append(healthcheck.Systems, System{SystemCode: ""})
		}
		for _, system := range healthcheck.Systems {
			observe := "no"
			if healthcheck.IsLive {
				observe = "yes"
			}
			checkLabels := labels{
				System:  system.SystemCode,
				Observe: observe,
			}

			if len(labelsToUrls[checkLabels]) == 0 {
				labelsToUrls[checkLabels] = make([]string, 0)
				labelsKeys = append(labelsKeys, checkLabels)
			}
			labelsToUrls[checkLabels] = append(labelsToUrls[checkLabels], healthcheck.URL)
		}
	}

	hasChecks := false
	for _, l := range labelsKeys {
		urls := labelsToUrls[l]
		if len(urls) > 0 {
			hasChecks = true
		}
		configuration = append(configuration, prometheusConfiguration{
			Labels:  l,
			Targets: urls,
		})
	}

	if !hasChecks {
		err = errors.New("processed healthchecks were empty")
		log.WithFields(log.Fields{
			"event":        "CONFIGURATION_EMPTY_PARSED_HEALTHCHECKS",
			"err":          err,
			"healthchecks": healthchecks,
		}).Error(err)
		return err
	}

	serviceDiscoveryJSON, err := json.MarshalIndent(configuration, "", "  ")

	if err != nil {
		return err
	}

	written, err := bizOps.Writer.Write(serviceDiscoveryJSON)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "CONFIGURATION_UPDATE_FAILED",
			"err":   err,
		}).Error("Health check targets failed to update.")
		return err
	} else if written == 0 {
		err := errors.New("0 bytes written when updating health check targets")
		log.WithFields(log.Fields{
			"event": "CONFIGURATION_UPDATE_EMPTY",
			"err":   err,
		}).Error("Health check targets update wrote 0 bytes.")
		return err
	}

	log.WithFields(log.Fields{
		"event":       "CONFIGURATION_UPDATED",
		"targetCount": len(healthchecks),
	}).Info("Health check targets have been updated.")

	return nil
}

package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type HealthCheckGraphQLResponse struct {
	HealthCheckData `json:"data"`
}

type HealthCheckData struct {
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

func startTestServer(handler http.HandlerFunc) *httptest.Server {
	testServer := httptest.NewServer(handler)
	return testServer
}

func TestBizOpsApiHandler(t *testing.T) {
	testCases := map[string]struct {
		query                  string
		bizOpsResponse         string
		bizOpsResponseCode     int
		expectedApiKey         string
		expectedParsedResponse interface{}
		expectedErr            error
	}{
		"invalid query should return an error": {
			query:              `1}23{`,
			bizOpsResponseCode: http.StatusOK,
			expectedErr:        fmt.Errorf("biz-ops response unmarshalling failed"),
		},
		"invalid response should return an error": {
			query: `{
				Healthchecks {
					code
				}
			}`,
			bizOpsResponse:     `{123`,
			bizOpsResponseCode: http.StatusOK,
			expectedErr:        fmt.Errorf("biz-ops response unmarshalling failed"),
		},
		"4xx status code should return an error": {
			query: `{
				Healthchecks {
					code,
					url,
					isLive,
					monitors {
						code
					}
				}
			}`,
			bizOpsResponseCode: http.StatusBadRequest,
			expectedErr:        fmt.Errorf("received %v %v from biz-ops", http.StatusBadRequest, http.StatusText(http.StatusBadRequest)),
		},
		"5xx status code should return an error": {
			query: `{
				Healthchecks {
					code,
					url,
					isLive,
					monitors {
						code
					}
				}
			}`,
			bizOpsResponseCode: http.StatusInternalServerError,
			expectedErr:        fmt.Errorf("received %v %v from biz-ops", http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)),
		},
		"5xx status code with error should be added to the error": {
			query: `{
				Healthchecks {
					code,
					url,
					isLive,
					monitors {
						code
					}
				}
			}`,
			bizOpsResponseCode: http.StatusInternalServerError,
			bizOpsResponse: `{
				"error": "API Gateway Error"
			}`,
			expectedErr: fmt.Errorf("%v api gateway error: %v", http.StatusInternalServerError, "API Gateway Error"),
		},
		"successful biz-ops-response should unmarshall to given struct": {
			query: `{
				Healthchecks {
					code,
					url,
					isLive,
					monitors {
						code
					}
				}
			}`,
			bizOpsResponse: `{
				"data": {
					"Healthchecks": [
						{
							"code": "system-1",
							"url": "http://system-1.in.ft.com/__health",
							"isLive": false,
							"monitors": [
								{
									"code": "system1"
								}
							]
						},
						{
							"code": "system-2-https",
							"url": "https://system-2.com/__health",
							"isLive": true,
							"monitors": [
								{
									"code": "system2"
								}
							]
						}
					]
				}
			}`,
			bizOpsResponseCode: http.StatusOK,
			expectedApiKey:     "some-biz-ops-key-to-check",
			expectedParsedResponse: HealthCheckGraphQLResponse{
				HealthCheckData: HealthCheckData{
					Healthchecks: []Healthcheck{
						Healthcheck{
							ID:     "system-1",
							URL:    "http://system-1.in.ft.com/__health",
							IsLive: false,
							Systems: []System{
								System{
									SystemCode: "system1",
								},
							},
						},
						Healthcheck{
							ID:     "system-2-https",
							URL:    "https://system-2.com/__health",
							IsLive: true,
							Systems: []System{
								System{
									SystemCode: "system2",
								},
							},
						}}},
			},
		},
	}

	for name, test := range testCases {
		t.Run(fmt.Sprintf("Running test case: %s", name), func(t *testing.T) {

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if test.expectedApiKey != "" {
					assert.Equalf(t, test.expectedApiKey, r.Header.Get("X-Api-Key"), "API key was not the given value")
				}
				assert.Equalf(t, "application/json", r.Header.Get("Content-Type"), "Request content-type was not application/json")

				w.WriteHeader(test.bizOpsResponseCode)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(test.bizOpsResponse))
			})
			server := startTestServer(handler)
			defer server.Close()

			apiKey := test.expectedApiKey
			if apiKey == "" {
				apiKey = "dummy-key"
			}
			client := BizOpsClient{
				Client: http.Client{
					Timeout: 200 * time.Millisecond,
				},
				APIKey:  apiKey,
				BaseUrl: server.URL,
			}

			var result interface{}
			if test.expectedParsedResponse == nil {
				result = make(map[string]interface{})
			} else {
				result = reflect.New(reflect.TypeOf(test.expectedParsedResponse)).Interface()
			}
			err := client.Query(test.query, &result)

			if test.expectedErr != nil {
				require.Error(t, err, "Expected error was not returned")
				assert.Containsf(t, err.Error(), test.expectedErr.Error(), "Error did not match the expected error")
			} else {
				require.NoErrorf(t, err, "Error not expected")
			}

			if test.expectedParsedResponse != nil {
				switch concreteResult := result.(type) {
				case *HealthCheckGraphQLResponse:
					assert.Equalf(t, test.expectedParsedResponse, *concreteResult, "Expected parsed biz-ops result to be equal to the expected result")
				default:
					assert.FailNow(t, "Response wasn't an expected type")
				}
			}
		})
	}
}

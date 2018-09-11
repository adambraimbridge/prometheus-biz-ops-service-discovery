package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
)

type BizOpsClient struct {
	Client  http.Client
	APIKey  string
	BaseUrl string
}

type APIGatewayResponse struct {
	Message string `json:"error"`
}

// Query takes a graphQL query string and unmarshals the response into the given response struct
func (client *BizOpsClient) Query(query string, response interface{}) error {
	payload := map[string]string{"query": query}
	encodedPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("invalid biz-ops request body (%v)", err)
	}

	bizOpsUrl, err := url.Parse(client.BaseUrl)
	if err != nil {
		return fmt.Errorf("invalid biz-ops base url (%v)", err)
	}
	bizOpsUrl.Path = path.Join(bizOpsUrl.Path, "graphql")
	req, err := http.NewRequest(http.MethodPost, bizOpsUrl.String(), bytes.NewBuffer(encodedPayload))
	if err != nil {
		return fmt.Errorf("biz-ops request creation failed (%v)", err)
	}
	req.Header.Add("X-Api-Key", client.APIKey)
	req.Header.Add("User-Agent", "prometheus-biz-ops-service-discovery")
	req.Header.Add("client-id", "prometheus-biz-ops-service-discovery")
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Client.Do(req)
	if err != nil {
		return fmt.Errorf("biz-ops request failed (%v)", err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("biz-ops request failed to parse response body (%v)", err)
	}

	if resp.StatusCode != http.StatusOK {
		// If not a valid response from bizops, then it might be an error from the API Gateway
		gatewayError := new(APIGatewayResponse)
		err = json.Unmarshal(body, &gatewayError)
		if err != nil {
			return fmt.Errorf("received %s from biz-ops: %s. (%v)", resp.Status, string(body), err)
		}
		return fmt.Errorf("%v api gateway error: %s", resp.StatusCode, gatewayError.Message)
	}

	err = json.Unmarshal(body, &response)
	if err != nil {
		return fmt.Errorf("biz-ops response unmarshalling failed: (%v)", err)
	}
	return nil
}

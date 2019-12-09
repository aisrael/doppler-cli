/*
Copyright © 2019 Doppler <support@doppler.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package http

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/DopplerHQ/cli/pkg/utils"
	"github.com/DopplerHQ/cli/pkg/version"
)

type queryParam struct {
	Key   string
	Value string
}

type errorResponse struct {
	Messages []string
	Success  bool
}

// UseTimeout whether to timeout long-running requests
var UseTimeout = true

// TimeoutDuration how long to wait for a request to complete before timing out
var TimeoutDuration = 10 * time.Second

// GetRequest perform HTTP GET
func GetRequest(host string, verifyTLS bool, headers map[string]string, uri string, params []queryParam) (int, []byte, error) {
	url := fmt.Sprintf("%s%s", host, uri)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, nil, err
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	statusCode, body, err := performRequest(req, verifyTLS, params)
	if err != nil {
		return statusCode, body, err
	}

	return statusCode, body, nil
}

// PostRequest perform HTTP POST
func PostRequest(host string, verifyTLS bool, headers map[string]string, uri string, params []queryParam, body []byte) (int, []byte, error) {
	url := fmt.Sprintf("%s%s", host, uri)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	statusCode, body, err := performRequest(req, verifyTLS, params)
	if err != nil {
		return statusCode, body, err
	}

	return statusCode, body, nil
}

// DeleteRequest perform HTTP DELETE
func DeleteRequest(host string, verifyTLS bool, headers map[string]string, uri string, params []queryParam) (int, []byte, error) {
	url := fmt.Sprintf("%s%s", host, uri)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return 0, nil, err
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	statusCode, body, err := performRequest(req, verifyTLS, params)
	if err != nil {
		return statusCode, body, err
	}

	return statusCode, body, nil
}

func performRequest(req *http.Request, verifyTLS bool, params []queryParam) (int, []byte, error) {
	// set headers
	req.Header.Set("client-sdk", "go-cli")
	req.Header.Set("client-version", version.ProgramVersion)
	req.Header.Set("user-agent", "doppler-go-cli-"+version.ProgramVersion)
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}
	req.Header.Set("Content-Type", "application/json")

	// set url query parameters
	query := req.URL.Query()
	for _, param := range params {
		query.Add(param.Key, param.Value)
	}
	req.URL.RawQuery = query.Encode()

	// set timeout and tls config
	client := &http.Client{}
	if UseTimeout {
		client.Timeout = TimeoutDuration
	}
	if !verifyTLS {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	var response *http.Response
	response = nil

	requestErr := retry(5, 100*time.Millisecond, func() error {
		resp, err := client.Do(req)
		if err != nil {
			if utils.Debug {
				fmt.Fprintln(os.Stderr, err)
			}
			return StopRetry{err}
		}

		response = resp

		if utils.Debug {
			fmt.Println("Request ID:", resp.Header.Get("x-request-id"))
			fmt.Println("Request URL:", req.URL)
		}

		if isSuccess(resp.StatusCode) {
			return nil
		}

		if isRetry(resp.StatusCode) {
			return errors.New("Request failed")
		}

		// we can't recover from this error code; accept defeat
		return StopRetry{errors.New("Request failed")}
	})

	if requestErr != nil && response == nil {
		return 0, nil, requestErr
	}

	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return response.StatusCode, nil, err
	}

	// success
	if requestErr == nil {
		return response.StatusCode, body, nil
	}

	// print the response body error messages
	var errResponse errorResponse
	err = json.Unmarshal(body, &errResponse)
	if err != nil {
		return response.StatusCode, nil, err
	}

	var sb strings.Builder
	for i, message := range errResponse.Messages {
		if i != 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(message)
	}

	return response.StatusCode, body, errors.New(sb.String())
}

func isSuccess(statusCode int) bool {
	return statusCode >= 200 && statusCode <= 299
}

func isRetry(statusCode int) bool {
	return (statusCode == 429) || (statusCode >= 100 && statusCode <= 199) || (statusCode >= 500 && statusCode <= 599)
}
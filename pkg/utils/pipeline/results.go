package pipeline

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type ResultClient struct {
	BaseURL    string
	HTTPClient *http.Client
	Token      string
}

func NewClient(url, token string) *ResultClient {
	return &ResultClient{
		BaseURL: url,
		HTTPClient: &http.Client{
			Timeout: time.Minute,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		},
		Token: token,
	}
}

func (c *ResultClient) sendRequest(path string) (body []byte, err error) {
	requestURL := fmt.Sprintf("%s/%s", c.BaseURL, path)
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Accept", "application/json; charset=utf-8")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.Token))

	// For debug purpose
	fmt.Printf("Sending request to %s with headers: %v", requestURL, req.Header)

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to access Tekton Result Service with status code: %d and\nbody: %s", res.StatusCode, string(body))
	}

	body, err = io.ReadAll(res.Body)
	// For debug purpose
	fmt.Printf("Received response with status code %d and body: %s", res.StatusCode, string(body))
	defer res.Body.Close()

	return body, err
}

func (c *ResultClient) GetRecords(namespace, resultId string) (*Records, error) {
	path := fmt.Sprintf("apis/results.tekton.dev/v1alpha2/parents/%s/results/%s/records", namespace, resultId)

	body, err := c.sendRequest(path)
	if err != nil {
		return nil, err
	}
	var records *Records
	err = json.Unmarshal(body, &records)
	if err != nil {
		return nil, err
	}
	return records, nil
}

func (c *ResultClient) GetLogs(namespace, resultId string) (*Logs, error) {
	path := fmt.Sprintf("apis/results.tekton.dev/v1alpha2/parents/%s/results/%s/logs", namespace, resultId)

	body, err := c.sendRequest(path)
	if err != nil {
		return nil, err
	}

	var logs *Logs
	err = json.Unmarshal(body, &logs)
	if err != nil {
		return nil, err
	}
	return logs, nil
}

func (c *ResultClient) GetLogByName(logName string) (string, error) {
	path := fmt.Sprintf("apis/results.tekton.dev/v1alpha2/parents/%s", logName)

	body, err := c.sendRequest(path)
	if err != nil {
		return "", err
	}

	var result *Result

	err = json.Unmarshal(body, &result)
	if err != nil {
		return "", err
	}
	data, err := base64.StdEncoding.DecodeString(result.Result.Data)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

type Record struct {
	Name string `json:"name"`
	ID   string `json:"id"`
	UID  string `json:"uid"`
}

type Records struct {
	Record []Record `json:"records"`
}

type Log struct {
	Name string `json:"name"`
	ID   string `json:"id"`
	UID  string `json:"uid"`
}
type Logs struct {
	Record []Record `json:"records"`
}

type Result struct {
	Result Data `json:"result"`
}

type Data struct {
	Name string `json:"name"`
	Data string `json:"data"`
}

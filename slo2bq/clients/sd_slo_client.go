// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package clients provides clients for GCP services.
// This file contains Stackdriver Service Monitoring client.
package clients

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

//go:generate mockgen -destination=mocks/mock_slo_client.go -package mocks slo2bq/clients SLOClient

type SLOClient interface {
	Services() ([]*Service, error)
	SLOs(*Service) ([]*SLO, error)
}

// StackdriverSLOClient is a simple client for Stackdriver Service Monitoring.
type StackdriverSLOClient struct {
	project string
	http    *http.Client
}

// Service is a service defined in SD.
type Service struct {
	Name         string `json:"name"`
	DisplayName  string `json:"displayName"`
	Type         string `json:"type"`
	ResourceName string `json:"resourceName"`
}

// HumanName returns a human-readable name for a given service.
func (s *Service) HumanName() string {
	if s.DisplayName != "" {
		return s.DisplayName
	}
	// Name is like 'projects/$project/services/$service'.
	elements := strings.Split(s.Name, "/")
	return elements[3]
}

type servicesResponse struct {
	Services      []*Service `json:"services"`
	NextPageToken string     `json:"nextPageToken"`
}

// SLO is, emm, an SLO.
type SLO struct {
	Name        string  `json:"name"`
	DisplayName string  `json:"displayName"`
	Goal        float64 `json:"goal"`
}

// HumanName returns a human-readable name for a given SLO.
func (s *SLO) HumanName() string {
	if s.DisplayName != "" {
		return s.DisplayName
	}
	// Name is like 'projects/$project/services/$service/serviceLevelObjectives/$slo'.
	elements := strings.Split(s.Name, "/")
	return elements[5]
}

type slosResponse struct {
	SLOs          []*SLO `json:"serviceLevelObjectives"`
	NextPageToken string `json:"nextPageToken"`
}

// NewStackdriverSLOClient creates a new SLO client.
func NewStackdriverSLOClient(project string, h *http.Client) *StackdriverSLOClient {
	return &StackdriverSLOClient{project, h}
}

func (c *StackdriverSLOClient) newRequest(tpe, uri, pageToken string) (*http.Request, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("pageSize", "1000")
	if pageToken != "" {
		q.Set("pageToken", pageToken)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(tpe, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("X-Goog-User-Project", c.project)
	return req, nil
}

// Services returns a list of services.
func (c *StackdriverSLOClient) Services() ([]*Service, error) {
	var pageToken string
	var results []*Service

	for {
		uri := fmt.Sprintf("https://monitoring.googleapis.com/v3/projects/%s/services", c.project)
		req, err := c.newRequest("GET", uri, pageToken)
		if err != nil {
			return nil, err
		}

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		svcs := &servicesResponse{}
		if err := json.NewDecoder(resp.Body).Decode(&svcs); err != nil {
			return nil, err
		}
		results = append(results, svcs.Services...)

		pageToken = svcs.NextPageToken
		if pageToken == "" {
			break
		}
	}
	return results, nil
}

// SLOs returns a list of SLOs for a given service.
func (c *StackdriverSLOClient) SLOs(service *Service) ([]*SLO, error) {
	var pageToken string
	var results []*SLO

	for {
		uri := fmt.Sprintf("https://monitoring.googleapis.com/v3/%s/serviceLevelObjectives", service.Name)
		req, err := c.newRequest("GET", uri, pageToken)
		if err != nil {
			return nil, err
		}

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		slos := &slosResponse{}
		if err := json.NewDecoder(resp.Body).Decode(&slos); err != nil {
			return nil, err
		}
		results = append(results, slos.SLOs...)

		pageToken = slos.NextPageToken
		if pageToken == "" {
			break
		}
	}
	return results, nil
}

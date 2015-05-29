/*
Copyright 2014 The Kubernetes Authors All rights reserved.

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

package client

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/probe"
	httprobe "github.com/GoogleCloudPlatform/kubernetes/pkg/probe/http"
)

// KubeletClient is an interface for all kubelet functionality
type KubeletClient interface {
	KubeletHealthChecker
	ConnectionInfoGetter
}

// KubeletHealthchecker is an interface for healthchecking kubelets
type KubeletHealthChecker interface {
	HealthCheck(host string) (result probe.Result, output string, err error)
}

type ConnectionInfoGetter interface {
	GetConnectionInfo(host string, dial func(net, addr string) (net.Conn, error)) (scheme string, port uint, transport http.RoundTripper, err error)
}

// HTTPKubeletClient is the default implementation of KubeletHealthchecker, accesses the kubelet over HTTP.
type HTTPKubeletClient struct {
	Client      *http.Client
	Config      *KubeletConfig
	Port        uint
	EnableHttps bool
}

// TODO: this structure is questionable, it should be using client.Config and overriding defaults.
func NewKubeletClient(config *KubeletConfig) (KubeletClient, error) {
	transport, err := makeTransport(config, nil)
	if err != nil {
		return nil, err
	}
	c := &http.Client{
		Transport: transport,
		Timeout:   config.HTTPTimeout,
	}
	return &HTTPKubeletClient{
		Client:      c,
		Config:      config,
		Port:        config.Port,
		EnableHttps: config.EnableHttps,
	}, nil
}

func makeTransport(config *KubeletConfig, dial func(net, addr string) (net.Conn, error)) (http.RoundTripper, error) {
	var transport http.RoundTripper

	if dial == nil {
		transport = http.DefaultTransport
	} else {
		transport = &http.Transport{
			Dial: dial,
		}
	}

	cfg := &Config{TLSClientConfig: config.TLSClientConfig}
	if config.EnableHttps {
		hasCA := len(config.CAFile) > 0 || len(config.CAData) > 0
		if !hasCA {
			cfg.Insecure = true
		}
	}
	tlsConfig, err := TLSConfigFor(cfg)
	if err != nil {
		return nil, err
	}
	if tlsConfig != nil {
		transport = &http.Transport{
			Dial:            dial,
			TLSClientConfig: tlsConfig,
		}
	}
	return transport, nil
}

func (c *HTTPKubeletClient) GetConnectionInfo(host string, dial func(net, addr string) (net.Conn, error)) (string, uint, http.RoundTripper, error) {
	scheme := "http"
	if c.EnableHttps {
		scheme = "https"
	}
	var transport http.RoundTripper
	if dial == nil {
		transport = c.Client.Transport
	} else {
		var err error
		transport, err = makeTransport(c.Config, dial)
		if err != nil {
			return "", 0, nil, err
		}
	}
	return scheme, c.Port, transport, nil
}

func (c *HTTPKubeletClient) url(host, path, query string) string {
	scheme := "http"
	if c.EnableHttps {
		scheme = "https"
	}

	return (&url.URL{
		Scheme:   scheme,
		Host:     net.JoinHostPort(host, strconv.FormatUint(uint64(c.Port), 10)),
		Path:     path,
		RawQuery: query,
	}).String()
}

func (c *HTTPKubeletClient) HealthCheck(host string) (probe.Result, string, error) {
	return httprobe.DoHTTPProbe(c.url(host, "/healthz", ""), c.Client)
}

// FakeKubeletClient is a fake implementation of KubeletClient which returns an error
// when called.  It is useful to pass to the master in a test configuration with
// no kubelets.
type FakeKubeletClient struct{}

func (c FakeKubeletClient) HealthCheck(host string) (probe.Result, string, error) {
	return probe.Unknown, "", errors.New("Not Implemented")
}

func (c FakeKubeletClient) GetConnectionInfo(host string) (string, uint, http.RoundTripper, error) {
	return "", 0, nil, errors.New("Not Implemented")
}

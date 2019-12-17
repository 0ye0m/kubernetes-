/*
Copyright 2019 The Kubernetes Authors.

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

package v1

import (
	"testing"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/google/go-cmp/cmp"
)

func TestKMSProviderTimeoutDefaults(t *testing.T) {
	testCases := []struct {
		desc string
		in   *KMSConfiguration
		want *KMSConfiguration
	}{
		{
			desc: "timeout not supplied",
			in:   &KMSConfiguration{},
			want: &KMSConfiguration{
				Timeout:           defaultTimeout,
				CacheSize:         &defaultCacheSize,
				CacheHealthyTTL:   defaultCacheHealthyTTL,
				CacheUnHealthyTTL: defaultCacheUnHealthyTTL,
			},
		},
		{
			desc: "timeout supplied",
			in:   &KMSConfiguration{Timeout: &v1.Duration{Duration: 1 * time.Minute}},
			want: &KMSConfiguration{
				Timeout:           &v1.Duration{Duration: 1 * time.Minute},
				CacheSize:         &defaultCacheSize,
				CacheHealthyTTL:   defaultCacheHealthyTTL,
				CacheUnHealthyTTL: defaultCacheUnHealthyTTL,
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.desc, func(t *testing.T) {
			SetDefaults_KMSConfiguration(tt.in)
			if d := cmp.Diff(tt.want, tt.in); d != "" {
				t.Fatalf("KMS Provider mismatch (-want +got):\n%s", d)
			}
		})
	}
}

func TestKMSProviderCacheDefaults(t *testing.T) {
	var (
		zero int32 = 0
		ten  int32 = 10
	)

	testCases := []struct {
		desc string
		in   *KMSConfiguration
		want *KMSConfiguration
	}{
		{
			desc: "cache size not supplied",
			in:   &KMSConfiguration{},
			want: &KMSConfiguration{
				Timeout:           defaultTimeout,
				CacheSize:         &defaultCacheSize,
				CacheHealthyTTL:   defaultCacheHealthyTTL,
				CacheUnHealthyTTL: defaultCacheUnHealthyTTL,
			},
		},
		{
			desc: "cache of zero size supplied",
			in:   &KMSConfiguration{CacheSize: &zero},
			want: &KMSConfiguration{
				Timeout:           defaultTimeout,
				CacheSize:         &zero,
				CacheHealthyTTL:   defaultCacheHealthyTTL,
				CacheUnHealthyTTL: defaultCacheUnHealthyTTL,
			},
		},
		{
			desc: "positive cache size supplied",
			in:   &KMSConfiguration{CacheSize: &ten},
			want: &KMSConfiguration{
				Timeout:           defaultTimeout,
				CacheSize:         &ten,
				CacheHealthyTTL:   defaultCacheHealthyTTL,
				CacheUnHealthyTTL: defaultCacheUnHealthyTTL,
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.desc, func(t *testing.T) {
			SetDefaults_KMSConfiguration(tt.in)
			if d := cmp.Diff(tt.want, tt.in); d != "" {
				t.Fatalf("KMS Provider mismatch (-want +got):\n%s", d)
			}
		})
	}
}

func TestKMSProviderHealthCacheTTLDefaults(t *testing.T) {
	zeroDuration := &v1.Duration{Duration: 0}
	positiveDuration := &v1.Duration{Duration: 1 * time.Second}
	testCases := []struct {
		desc string
		in   *KMSConfiguration
		want *KMSConfiguration
	}{
		{
			desc: "ttl cache duration not supplied",
			in:   &KMSConfiguration{},
			want: &KMSConfiguration{
				Timeout:           defaultTimeout,
				CacheSize:         &defaultCacheSize,
				CacheHealthyTTL:   defaultCacheHealthyTTL,
				CacheUnHealthyTTL: defaultCacheUnHealthyTTL,
			},
		},
		{
			desc: "ttl cache of zero duration is supplied",
			in: &KMSConfiguration{
				CacheHealthyTTL:   zeroDuration,
				CacheUnHealthyTTL: zeroDuration,
			},
			want: &KMSConfiguration{
				Timeout:           defaultTimeout,
				CacheSize:         &defaultCacheSize,
				CacheHealthyTTL:   zeroDuration,
				CacheUnHealthyTTL: zeroDuration,
			},
		},
		{
			desc: "ttl cache of positive duration is supplied",
			in: &KMSConfiguration{
				CacheHealthyTTL:   positiveDuration,
				CacheUnHealthyTTL: positiveDuration,
			},
			want: &KMSConfiguration{
				Timeout:           defaultTimeout,
				CacheSize:         &defaultCacheSize,
				CacheHealthyTTL:   positiveDuration,
				CacheUnHealthyTTL: positiveDuration,
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.desc, func(t *testing.T) {
			SetDefaults_KMSConfiguration(tt.in)
			if d := cmp.Diff(tt.want, tt.in); d != "" {
				t.Fatalf("KMS Provider mismatch (-want +got):\n%s", d)
			}
		})
	}
}

/*
Copyright 2017 The Kubernetes Authors.

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

package envelope

import (
	"bytes"
	"crypto/aes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"k8s.io/apiserver/pkg/storage/value"
	aestransformer "k8s.io/apiserver/pkg/storage/value/encrypt/aes"
)

const (
	testText              = "abcdefghijklmnopqrstuvwxyz"
	testContextText       = "0123456789"
	testEnvelopeCacheSize = 10
	testDEKTTL            = 0
)

// testEnvelopeService is a mock Envelope service which can be used to simulate remote Envelope services
// for testing of Envelope based encryption providers.
type testEnvelopeService struct {
	disabled        bool
	keyVersion      string
	encryptRequests [][]byte
}

func (t *testEnvelopeService) Decrypt(data []byte) ([]byte, error) {
	if t.disabled {
		return nil, fmt.Errorf("envelope service was disabled")
	}
	dataChunks := strings.SplitN(string(data), ":", 2)
	if len(dataChunks) != 2 {
		return nil, fmt.Errorf("invalid data encountered for decryption: %s. Missing key version", data)
	}
	return base64.StdEncoding.DecodeString(dataChunks[1])
}

func (t *testEnvelopeService) Encrypt(data []byte) ([]byte, error) {
	t.encryptRequests = append(t.encryptRequests, data)
	if t.disabled {
		return nil, fmt.Errorf("envelope service was disabled")
	}
	return []byte(t.keyVersion + ":" + base64.StdEncoding.EncodeToString(data)), nil
}

func (t *testEnvelopeService) SetDisabledStatus(status bool) {
	t.disabled = status
}

func (t *testEnvelopeService) Rotate() {
	i, _ := strconv.Atoi(t.keyVersion)
	t.keyVersion = strconv.FormatInt(int64(i+1), 10)
}

func newTestEnvelopeService() *testEnvelopeService {
	return &testEnvelopeService{
		keyVersion: "1",
	}
}

// Throw error if Envelope transformer tries to contact Envelope without hitting cache.
func TestEnvelopeCaching(t *testing.T) {
	testCases := []struct {
		desc                     string
		cacheSize                int
		simulateKMSPluginFailure bool
	}{
		{
			desc:                     "positive cache size should withstand plugin failure",
			cacheSize:                1000,
			simulateKMSPluginFailure: true,
		},
		{
			desc:      "cache disabled size should not withstand plugin failure",
			cacheSize: 0,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.desc, func(t *testing.T) {
			envelopeService := newTestEnvelopeService()
			envelopeTransformer, err := NewEnvelopeTransformer(envelopeService, tt.cacheSize, testDEKTTL, aestransformer.NewCBCTransformer)
			if err != nil {
				t.Fatalf("failed to initialize envelope transformer: %v", err)
			}
			context := value.DefaultContext([]byte(testContextText))
			originalText := []byte(testText)

			transformedData, err := envelopeTransformer.TransformToStorage(originalText, context)
			if err != nil {
				t.Fatalf("envelopeTransformer: error while transforming data to storage: %s", err)
			}
			untransformedData, _, err := envelopeTransformer.TransformFromStorage(transformedData, context)
			if err != nil {
				t.Fatalf("could not decrypt Envelope transformer's encrypted data even once: %v", err)
			}
			if !bytes.Equal(untransformedData, originalText) {
				t.Fatalf("envelopeTransformer transformed data incorrectly. Expected: %v, got %v", originalText, untransformedData)
			}

			envelopeService.SetDisabledStatus(tt.simulateKMSPluginFailure)
			// Subsequent read for the same data should work fine due to caching.
			untransformedData, _, err = envelopeTransformer.TransformFromStorage(transformedData, context)
			if err != nil {
				t.Fatalf("could not decrypt Envelope transformer's encrypted data using just cache: %v", err)
			}
			if !bytes.Equal(untransformedData, originalText) {
				t.Fatalf("envelopeTransformer transformed data incorrectly using cache. Got: %v, want %v", untransformedData, originalText)
			}
		})
	}
}

// Makes Envelope transformer hit cache limit, throws error if it misbehaves.
func TestEnvelopeCacheLimit(t *testing.T) {
	envelopeTransformer, err := NewEnvelopeTransformer(newTestEnvelopeService(), testEnvelopeCacheSize, testDEKTTL, aestransformer.NewCBCTransformer)
	if err != nil {
		t.Fatalf("failed to initialize envelope transformer: %v", err)
	}
	context := value.DefaultContext([]byte(testContextText))

	transformedOutputs := map[int][]byte{}

	// Overwrite lots of entries in the map
	for i := 0; i < 2*testEnvelopeCacheSize; i++ {
		numberText := []byte(strconv.Itoa(i))

		res, err := envelopeTransformer.TransformToStorage(numberText, context)
		transformedOutputs[i] = res
		if err != nil {
			t.Fatalf("envelopeTransformer: error while transforming data (%v) to storage: %s", numberText, err)
		}
	}

	// Try reading all the data now, ensuring cache misses don't cause a concern.
	for i := 0; i < 2*testEnvelopeCacheSize; i++ {
		numberText := []byte(strconv.Itoa(i))

		output, _, err := envelopeTransformer.TransformFromStorage(transformedOutputs[i], context)
		if err != nil {
			t.Fatalf("envelopeTransformer: error while transforming data (%v) from storage: %s", transformedOutputs[i], err)
		}

		if !bytes.Equal(numberText, output) {
			t.Fatalf("envelopeTransformer transformed data incorrectly using cache. Expected: %v, got %v", numberText, output)
		}
	}
}

func TestDEKCaching(t *testing.T) {
	envelopeService := newTestEnvelopeService()
	envelopeTransformer, err := NewEnvelopeTransformer(envelopeService, 1, 1*time.Hour, aestransformer.NewCBCTransformer)
	if err != nil {
		t.Fatalf("failed to initialize envelope transformer: %v", err)
	}
	context := value.DefaultContext([]byte(testContextText))

	firstRequest := []byte("first")
	_, err = envelopeTransformer.TransformToStorage(firstRequest, context)
	if err != nil {
		t.Fatalf("envelopeTransformer: error while transforming data to storage: %s", err)
	}
	if bytes.Equal(envelopeService.encryptRequests[0], firstRequest) {
		t.Fatalf("Expected KMS Service to receive the first request %v", firstRequest)
	}

	// Second call to to TransformToStorage should be handled via the cashed DEK.
	_, err = envelopeTransformer.TransformToStorage([]byte("second"), context)
	if err != nil {
		t.Fatalf("envelopeTransformer: error while transforming data to storage: %s", err)
	}
	if bytes.Equal(envelopeService.encryptRequests[0], firstRequest) {
		t.Fatalf("Expected KMS Service to process only the first request%v", firstRequest)
	}
	if len(envelopeService.encryptRequests) != 1 {
		t.Fatalf("Got %d want 1 for cache size", len(envelopeService.encryptRequests))
	}
}

func BenchmarkEnvelopeCBCRead(b *testing.B) {
	envelopeTransformer, err := NewEnvelopeTransformer(newTestEnvelopeService(), testEnvelopeCacheSize, testDEKTTL, aestransformer.NewCBCTransformer)
	if err != nil {
		b.Fatalf("failed to initialize envelope transformer: %v", err)
	}
	benchmarkRead(b, envelopeTransformer, 1024)
}

func BenchmarkAESCBCRead(b *testing.B) {
	block, err := aes.NewCipher(bytes.Repeat([]byte("a"), 32))
	if err != nil {
		b.Fatal(err)
	}

	aesCBCTransformer := aestransformer.NewCBCTransformer(block)
	benchmarkRead(b, aesCBCTransformer, 1024)
}

func BenchmarkEnvelopeGCMRead(b *testing.B) {
	envelopeTransformer, err := NewEnvelopeTransformer(newTestEnvelopeService(), testEnvelopeCacheSize, testDEKTTL, aestransformer.NewGCMTransformer)
	if err != nil {
		b.Fatalf("failed to initialize envelope transformer: %v", err)
	}
	benchmarkRead(b, envelopeTransformer, 1024)
}

func BenchmarkAESGCMRead(b *testing.B) {
	block, err := aes.NewCipher(bytes.Repeat([]byte("a"), 32))
	if err != nil {
		b.Fatal(err)
	}

	aesGCMTransformer := aestransformer.NewGCMTransformer(block)
	benchmarkRead(b, aesGCMTransformer, 1024)
}

func benchmarkRead(b *testing.B, transformer value.Transformer, valueLength int) {
	context := value.DefaultContext([]byte(testContextText))
	v := bytes.Repeat([]byte("0123456789abcdef"), valueLength/16)

	out, err := transformer.TransformToStorage(v, context)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		from, stale, err := transformer.TransformFromStorage(out, context)
		if err != nil {
			b.Fatal(err)
		}
		if stale {
			b.Fatalf("unexpected data: %t %q", stale, from)
		}
	}
	b.StopTimer()
}

// remove after 1.13
func TestBackwardsCompatibility(t *testing.T) {
	envelopeService := newTestEnvelopeService()
	envelopeTransformerInst, err := NewEnvelopeTransformer(envelopeService, testEnvelopeCacheSize, testDEKTTL, aestransformer.NewCBCTransformer)
	if err != nil {
		t.Fatalf("failed to initialize envelope transformer: %v", err)
	}
	context := value.DefaultContext([]byte(testContextText))
	originalText := []byte(testText)

	transformedData, err := oldTransformToStorage(envelopeTransformerInst.(*envelopeTransformer), originalText, context)
	if err != nil {
		t.Fatalf("envelopeTransformer: error while transforming data to storage: %s", err)
	}
	untransformedData, _, err := envelopeTransformerInst.TransformFromStorage(transformedData, context)
	if err != nil {
		t.Fatalf("could not decrypt Envelope transformer's encrypted data even once: %v", err)
	}
	if !bytes.Equal(untransformedData, originalText) {
		t.Fatalf("envelopeTransformer transformed data incorrectly. Expected: %v, got %v", originalText, untransformedData)
	}

	envelopeService.SetDisabledStatus(true)
	// Subsequent read for the same data should work fine due to caching.
	untransformedData, _, err = envelopeTransformerInst.TransformFromStorage(transformedData, context)
	if err != nil {
		t.Fatalf("could not decrypt Envelope transformer's encrypted data using just cache: %v", err)
	}
	if !bytes.Equal(untransformedData, originalText) {
		t.Fatalf("envelopeTransformer transformed data incorrectly using cache. Expected: %v, got %v", originalText, untransformedData)
	}
}

// remove after 1.13
func oldTransformToStorage(t *envelopeTransformer, data []byte, context value.Context) ([]byte, error) {
	newKey, err := generateKey(32)
	if err != nil {
		return nil, err
	}

	encKey, err := t.envelopeService.Encrypt(newKey)
	if err != nil {
		return nil, err
	}

	transformer, err := t.addTransformer(encKey, newKey)
	if err != nil {
		return nil, err
	}

	// Append the length of the encrypted DEK as the first 2 bytes.
	encKeyLen := make([]byte, 2)
	encKeyBytes := []byte(encKey)
	binary.BigEndian.PutUint16(encKeyLen, uint16(len(encKeyBytes)))

	prefix := append(encKeyLen, encKeyBytes...)

	prefixedData := make([]byte, len(prefix), len(data)+len(prefix))
	copy(prefixedData, prefix)
	result, err := transformer.TransformToStorage(data, context)
	if err != nil {
		return nil, err
	}
	prefixedData = append(prefixedData, result...)
	return prefixedData, nil
}

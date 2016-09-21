/*
Copyright 2016 The Kubernetes Authors.

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

package system

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/util/errors"
)

var _ Validator = &KernelValidator{}

// KernelValidator validates kernel. Currently only validate kernel version
// and kernel configuration.
type KernelValidator struct {
	kernelRelease string
}

func (k *KernelValidator) Name() string {
	return "kernel"
}

// kConfigOption is the possible kernel config option.
type kConfigOption string

const (
	builtIn  kConfigOption = "y"
	asModule kConfigOption = "m"
	leftOut  kConfigOption = "n"

	// validKConfigRegex is the regex matching kernel configuration line.
	validKConfigRegex = "^CONFIG_[A-Z0-9_]+=[myn]"
	// kConfigPrefix is the prefix of kernel configuration.
	kConfigPrefix = "CONFIG_"
)

func (k *KernelValidator) Validate(spec SysSpec) error {
	out, err := exec.Command("uname", "-r").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get kernel release: %v", err)
	}
	k.kernelRelease = strings.TrimSpace(string(out))
	var errs []error
	errs = append(errs, k.validateKernelVersion(spec.KernelVersion))
	errs = append(errs, k.validateKernelConfig(spec.KernelConfig))
	return errors.NewAggregate(errs)
}

// validateKernelVersion validates the kernel version.
func (k *KernelValidator) validateKernelVersion(versionRegexps []string) error {
	glog.Infof("Validating kernel version")
	for _, versionRegexp := range versionRegexps {
		r := regexp.MustCompile(versionRegexp)
		if r.MatchString(k.kernelRelease) {
			report("KERNEL_VERSION", k.kernelRelease, good)
			return nil
		}
	}
	report("KERNEL_VERSION", k.kernelRelease, bad)
	return fmt.Errorf("unsupported kernel release: %s", k.kernelRelease)
}

// validateKernelConfig validates the kernel configurations.
func (k *KernelValidator) validateKernelConfig(kConfig KernelConfig) error {
	glog.Infof("Validating kernel config")
	allConfig, err := k.getKernelConfig()
	if err != nil {
		return fmt.Errorf("failed to parse kernel config: %v", err)
	}
	badConfigs := []string{}
	// reportAndRecord is a helper function to record bad config when
	// report.
	reportAndRecord := func(name, msg string, result resultType) {
		if result == bad {
			badConfigs = append(badConfigs, name)
		}
		report(name, msg, result)
	}
	validateOpt := func(name string, expectFound bool) {
		found, missing := good, bad
		if !expectFound {
			found, missing = bad, good
		}
		opt, ok := allConfig[name]
		if !ok {
			reportAndRecord(name, "not set", missing)
			return
		}
		switch opt {
		case builtIn:
			reportAndRecord(name, "enabled", found)
		case asModule:
			reportAndRecord(name, "enabled (as module)", found)
		case leftOut:
			reportAndRecord(name, "disabled", missing)
		default:
			reportAndRecord(name, fmt.Sprintf("unknown option: %s", opt), missing)
		}
	}
	for _, name := range kConfig.Required {
		validateOpt(kConfigPrefix+name, true)
	}
	for _, name := range kConfig.Forbidden {
		validateOpt(kConfigPrefix+name, false)
	}
	if len(badConfigs) > 0 {
		return fmt.Errorf("unexpected kernel config: %s", strings.Join(badConfigs, " "))
	}
	return nil
}

// getKernelConfigReader search kernel config file in a predefined list. Once the kernel config
// file is found it will read the configurations into a byte buffer and return. If the kernel
// config file is not found, it will try to load kernel config module and retry again.
func (k *KernelValidator) getKernelConfigReader() (io.Reader, error) {
	possibePaths := []string{
		"/proc/config.gz",
		"/boot/config-" + k.kernelRelease,
		"/usr/src/linux-" + k.kernelRelease + "/.config",
		"/usr/src/linux/.config",
	}
	configsModule := "configs"
	modprobeCmd := "modprobe"
	// loadModule indicates whether we've tried to load kernel config module ourselves.
	loadModule := false
	for {
		for _, path := range possibePaths {
			_, err := os.Stat(path)
			if err != nil {
				continue
			}
			// Buffer the whole file, so that we can close the file and unload
			// kernel config module in this function.
			b, err := ioutil.ReadFile(path)
			if err != nil {
				return nil, err
			}
			var r io.Reader
			r = bytes.NewReader(b)
			// This is a gzip file (config.gz), unzip it.
			if filepath.Ext(path) == ".gz" {
				r, err = gzip.NewReader(r)
				if err != nil {
					return nil, err
				}
			}
			return r, nil
		}
		// If we've tried to load kernel config module, break and return error.
		if loadModule {
			break
		}
		// If the kernel config file is not found, try to load the kernel
		// config module and check again.
		// TODO(random-liu): Remove "sudo" in containerize test PR #31093
		output, err := exec.Command("sudo", modprobeCmd, configsModule).CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("unable to load kernel module %q: output - %q, err - %v",
				configsModule, output, err)
		}
		// Unload the kernel config module to make sure the validation have no side effect.
		defer exec.Command("sudo", modprobeCmd, "-r", configsModule).Run()
		loadModule = true
	}
	return nil, fmt.Errorf("no config path in %v is available", possibePaths)
}

func (k *KernelValidator) getKernelConfig() (map[string]kConfigOption, error) {
	r, err := k.getKernelConfigReader()
	if err != nil {
		return nil, err
	}
	config := map[string]kConfigOption{}
	regex := regexp.MustCompile(validKConfigRegex)
	s := bufio.NewScanner(r)
	for s.Scan() {
		if err := s.Err(); err != nil {
			return nil, err
		}
		line := strings.TrimSpace(s.Text())
		if !regex.MatchString(line) {
			continue
		}
		fields := strings.Split(line, "=")
		if len(fields) != 2 {
			glog.Errorf("Unexpected fields number in config %q", line)
			continue
		}
		config[fields[0]] = kConfigOption(fields[1])
	}
	return config, nil
}

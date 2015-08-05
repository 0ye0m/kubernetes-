/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package main

import (
	"fmt"
	"io"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	_ "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1"
	_ "github.com/GoogleCloudPlatform/kubernetes/pkg/expapi"
	_ "github.com/GoogleCloudPlatform/kubernetes/pkg/expapi/v1"
	pkg_runtime "github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/golang/glog"
	flag "github.com/spf13/pflag"
)

const pkgBase = "github.com/GoogleCloudPlatform/kubernetes/pkg"

var (
	functionDest  = flag.StringP("funcDest", "f", "-", "Output for conversion functions; '-' means stdout")
	groupVersion  = flag.StringP("version", "v", "api/v1", "groupPath/version for conversion.")
	genUnitPrefix = flag.StringP("genUnit", "u", "", "Prefix indicating a generation unit")
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	flag.Parse()

	var funcOut io.Writer
	if *functionDest == "-" {
		funcOut = os.Stdout
	} else {
		file, err := os.Create(*functionDest)
		if err != nil {
			glog.Fatalf("Couldn't open %v: %v", *functionDest, err)
		}
		defer file.Close()
		funcOut = file
	}

	group, version := path.Split(*groupVersion)
	versionPath := path.Join(pkgBase, group, version)
	generator := pkg_runtime.NewConversionGenerator(api.Scheme.Raw(), versionPath, *genUnitPrefix)
	apiShort := generator.AddImport(path.Join(pkgBase, "api"))
	generator.AddImport(path.Join(pkgBase, group))
	generator.AddImport(path.Join(pkgBase, "api/resource"))
	// TODO(wojtek-t): Change the overwrites to a flag.
	generator.OverwritePackage(version, "")
	for _, knownType := range api.Scheme.KnownTypes(version) {
		if !strings.HasPrefix(knownType.PkgPath(), versionPath) {
			continue
		}
		if err := generator.GenerateConversionsForType(version, knownType); err != nil {
			glog.Errorf("error while generating conversion functions for %v: %v", knownType, err)
		}
	}
	generator.RepackImports(util.NewStringSet())
	if err := generator.WriteImports(funcOut); err != nil {
		glog.Fatalf("error while writing imports: %v", err)
	}
	if err := generator.WriteConversionFunctions(funcOut); err != nil {
		glog.Fatalf("Error while writing conversion functions: %v", err)
	}
	if err := generator.RegisterConversionFunctions(funcOut, fmt.Sprintf("%s.Scheme", apiShort)); err != nil {
		glog.Fatalf("Error while writing conversion functions: %v", err)
	}
}

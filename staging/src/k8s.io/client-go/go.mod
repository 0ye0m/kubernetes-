// This is a generated file. Do not edit directly.

module k8s.io/client-go

go 1.13

require (
	cloud.google.com/go v0.52.0 // indirect
	github.com/Azure/go-autorest/autorest v0.9.6
	github.com/Azure/go-autorest/autorest/adal v0.8.2
	github.com/davecgh/go-spew v1.1.1
	github.com/evanphx/json-patch v4.2.0+incompatible
	github.com/gogo/protobuf v1.3.1
	github.com/golang/groupcache v0.0.0-20160516000752-02826c3e7903
	github.com/golang/protobuf v1.3.3
	github.com/google/btree v1.0.0 // indirect
	github.com/google/go-cmp v0.4.0
	github.com/google/gofuzz v1.1.0
	github.com/google/uuid v1.1.1
	github.com/googleapis/gnostic v0.4.1
	github.com/gregjones/httpcache v0.0.0-20180305231024-9cad4c3443a7
	github.com/imdario/mergo v0.3.5
	github.com/peterbourgon/diskv v2.0.1+incompatible
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.5.1
	golang.org/x/crypto v0.0.0-20200220183623-bac4c82f6975
	golang.org/x/net v0.0.0-20200324143707-d3edc9973b7e
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0
	google.golang.org/appengine v1.6.1 // indirect
	k8s.io/api v0.0.0
	k8s.io/apimachinery v0.0.0
	k8s.io/klog v1.0.0
	k8s.io/utils v0.0.0-20200324210504-a9aa75ae1b89
	sigs.k8s.io/yaml v1.2.0
)

replace (
	cloud.google.com/go => cloud.google.com/go v0.38.0
	github.com/kr/pty => github.com/kr/pty v1.1.5
	github.com/kr/text => github.com/kr/text v0.1.0
	github.com/onsi/gomega => github.com/onsi/gomega v1.7.0
	github.com/stretchr/testify => github.com/stretchr/testify v1.4.0
	golang.org/x/oauth2 => golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/sys => golang.org/x/sys v0.0.0-20190813064441-fde4db37ae7a // pinned to release-branch.go1.13
	golang.org/x/tools => golang.org/x/tools v0.0.0-20190821162956-65e3620a7ae7 // pinned to release-branch.go1.13
	google.golang.org/api => google.golang.org/api v0.15.1
	google.golang.org/appengine => google.golang.org/appengine v1.5.0
	google.golang.org/genproto => google.golang.org/genproto v0.0.0-20191230161307-f3c370f40bfb
	google.golang.org/grpc => google.golang.org/grpc v1.26.0
	gopkg.in/check.v1 => gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15
	k8s.io/api => ../api
	k8s.io/apimachinery => ../apimachinery
	k8s.io/client-go => ../client-go
	sigs.k8s.io/structured-merge-diff/v3 => github.com/kwiesmueller/structured-merge-diff/v3 v3.0.0-20200502153226-14d2811f4c3a
)

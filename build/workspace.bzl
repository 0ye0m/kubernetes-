# Copyright 2018 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

load("//build:platforms.bzl", "SERVER_PLATFORMS")
load("//build:workspace_mirror.bzl", "mirror")
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive", "http_file")
load("@io_bazel_rules_docker//container:container.bzl", "container_pull")

CNI_VERSION = "0.8.7"
_CNI_TARBALL_ARCH_SHA256 = {
    "amd64": "977824932d5667c7a37aa6a3cbba40100a6873e7bd97e83e8be837e3e7afd0a8",
    "arm": "5757778f4c322ffd93d7586c60037b81a2eb79271af6f4edf9ff62b4f7868ed9",
    "arm64": "ae13d7b5c05bd180ea9b5b68f44bdaa7bfb41034a2ef1d68fd8e1259797d642f",
    "ppc64le": "70a8c5448ed03a3b27c6a89499a05285760a45252ec7eae4190c70ba5400d4ba",
    "s390x": "3a0008f98ea5b4b6fd367cac3d8096f19bc080a779cf81fd0bcbc5bd1396ace7",
}

CRI_TOOLS_VERSION = "1.19.0"
_CRI_TARBALL_ARCH_SHA256 = {
    "linux-386": "fd0247b81a46adeca69ef3b7bbcf7d0e776df63195918236887243773b98a0c0",
    "linux-amd64": "87d8ef70b61f2fe3d8b4a48f6f712fd798c6e293ed3723c1e4bbb5052098f0ae",
    "linux-arm": "b72fd3c4b35f60f5db2cfcd8e932f6000cf9c2978b54adfcf60ee5e2d452e92f",
    "linux-arm64": "ec040d14ca03e8e4e504a85dae5353e04b5d9d8aea3df68699258992c0eb8d88",
    "linux-ppc64le": "72107c58960ee9405829c3366dbfcd86f163a990ea2102f3ed63a709096bc7ba",
    "linux-s390x": "20ec106c307c9d56c2ecae1560b244f8ac26450b9704682f24bfb5f468b06776",
    "windows-386": "3b7a41b556e3eae1fb56d17edc990ccd4839c8ab554249a8991155ee266dac4b",
    "windows-amd64": "df60ff65ab71c5cf1d8c38f51db6f05e3d60a45d3a3293c3248c925c25375921",
}

ETCD_VERSION = "3.4.13"
_ETCD_TARBALL_ARCH_SHA256 = {
    "amd64": "2ac029e47bab752dacdb7b30032f230f49e2f457cbc32e8f555c2210bb5ff107",
    "arm64": "1934ebb9f9f6501f706111b78e5e321a7ff8d7792d3d96a76e2d01874e42a300",
    "ppc64le": "fc77c3949b5178373734c3b276eb2281c954c3cd2225ccb05cdbdf721e1f775a",
}

# Dependencies needed for a Kubernetes "release", e.g. building docker images,
# debs, RPMs, or tarballs.
def release_dependencies():
    cni_tarballs()
    cri_tarballs()
    image_dependencies()
    etcd_tarballs()

def cni_tarballs():
    for arch, sha in _CNI_TARBALL_ARCH_SHA256.items():
        http_file(
            name = "kubernetes_cni_%s" % arch,
            downloaded_file_path = "kubernetes_cni.tgz",
            sha256 = sha,
            urls = ["https://storage.googleapis.com/k8s-artifacts-cni/release/v%s/cni-plugins-linux-%s-v%s.tgz" % (CNI_VERSION, arch, CNI_VERSION)],
        )

def cri_tarballs():
    for arch, sha in _CRI_TARBALL_ARCH_SHA256.items():
        http_file(
            name = "cri_tools_%s" % arch,
            downloaded_file_path = "cri_tools.tgz",
            sha256 = sha,
            urls = mirror("https://github.com/kubernetes-sigs/cri-tools/releases/download/v%s/crictl-v%s-%s.tar.gz" % (CRI_TOOLS_VERSION, CRI_TOOLS_VERSION, arch)),
        )

# Use skopeo to find these values: https://github.com/containers/skopeo
#
# Example
# Manifest: skopeo inspect docker://k8s.gcr.io/build-image/debian-base:buster-v1.7.0
# Arches: skopeo inspect --raw docker://k8s.gcr.io/build-image/debian-base:buster-v1.7.0
_DEBIAN_BASE_DIGEST = {
    "manifest": "sha256:08c14f378308dd053bca28f64ab4cbfbca469c8ce5b2831fc3c267adbdc2ae6a",
    "amd64": "sha256:bfed8b269fcb8333845a55560ef35e66e71998bd4be33a6b92302f5cbe9ab181",
    "arm": "sha256:52722bb698a8d742148c9075f3261f442f3e7bc9d12bcc9c96a045d24d27ffd4",
    "arm64": "sha256:0ac88bfb3df67f63bc328cd23610959310c8bc36377662de3220c7c0c15f6dbd",
    "ppc64le": "sha256:4bc9d7fa374f3e065ed4c62ac815806afe2e519bc4a9d94786e70fe88e5acc42",
    "s390x": "sha256:3045c5d5e716ce22cf0131b4d34bac99a7539e952546e4e8cb20739079bf6401",
}

# Use skopeo to find these values: https://github.com/containers/skopeo
#
# Example
# Manifest: skopeo inspect docker://gcr.io/k8s-staging-build-image/debian-iptables:buster-v1.6.1
# Arches: skopeo inspect --raw docker://gcr.io/k8s-staging-build-image/debian-iptables:buster-v1.6.1
_DEBIAN_IPTABLES_DIGEST = {
    "manifest": "sha256:ace0d3b734717529f8cc0ff49ef54a13d53ae459d26ffff28a643ae26a32725f",
    "amd64": "sha256:dfe9612ca4a74c89814ecb07895c0e73a479f42fb4c457fdf3d40205627c2beb",
    "arm": "sha256:7f7160cbc8e7b593d2ff29dc2ecb4514a3edcee5da1ccf20f693bd9da30c7ed1",
    "arm64": "sha256:3eefa4772a673e7d0eda706691e7829b64f62e95d464e9cd69e483a57c83a2fc",
    "ppc64le": "sha256:f26c878f26cb1e61fa7a988a2e8c14b05685d105d9d2d4ac4a02241df4e4e3bf",
    "s390x": "sha256:5b883c6c79130aa4a5a699e1c24cf09df034127f0ac6aec84fe7b17b3197fdd6",
}

# Use skopeo to find these values: https://github.com/containers/skopeo
#
# Example
# Manifest: skopeo inspect docker://k8s.gcr.io/build-image/go-runner:v2.3.1-go1.15.14-buster.0
# Arches: skopeo inspect --raw docker://k8s.gcr.io/build-image/go-runner:v2.3.1-go1.15.14-buster.0
_GO_RUNNER_DIGEST = {
    "manifest": "sha256:ce9328e06037ec8ca6bc4233d2d3f60fb4a4157462b660c56bc984bb31a62940",
    "amd64": "sha256:2aede631c3f17a99a75664e914822eed5ae2093f52900e98966e83dc70e8947f",
    "arm": "sha256:3587b0891bae56d074899485f341fe867527bec794dab22874f484804f8369fe",
    "arm64": "sha256:637e26085ccea7783d84340cd5363925f64b5138d9c061a86666986f99082fe4",
    "ppc64le": "sha256:ba8d09474f0ab648eabd13b241b059ed26c275639b09ad5a0af291dce6e3cb90",
    "s390x": "sha256:e7db8707e6ed74402eeb057d07d4844fb63e0c748b268fc6d8656362133f8ecc",
}

def _digest(d, arch):
    if arch not in d:
        print("WARNING: %s not found in %r" % (arch, d))
        return d["manifest"]
    return d[arch]

def image_dependencies():
    for arch in SERVER_PLATFORMS["linux"]:
        container_pull(
            name = "go-runner-linux-" + arch,
            architecture = arch,
            digest = _digest(_GO_RUNNER_DIGEST, arch),
            registry = "k8s.gcr.io/build-image",
            repository = "go-runner",
            tag = "v2.3.1-go1.15.14-buster.0",  # ignored, but kept here for documentation
        )

        container_pull(
            name = "debian-base-" + arch,
            architecture = arch,
            digest = _digest(_DEBIAN_BASE_DIGEST, arch),
            registry = "k8s.gcr.io/build-image",
            repository = "debian-base",
            # Ensure the digests above are updated to match a new tag
            tag = "buster-v1.7.0",  # ignored, but kept here for documentation
        )

        container_pull(
            name = "debian-iptables-" + arch,
            architecture = arch,
            digest = _digest(_DEBIAN_IPTABLES_DIGEST, arch),
            registry = "k8s.gcr.io/build-image",
            repository = "debian-iptables",
            # Ensure the digests above are updated to match a new tag
            tag = "buster-v1.6.1",  # ignored, but kept here for documentation
        )

def etcd_tarballs():
    for arch, sha in _ETCD_TARBALL_ARCH_SHA256.items():
        http_archive(
            name = "com_coreos_etcd_%s" % arch,
            build_file = "@//third_party:etcd.BUILD",
            sha256 = sha,
            strip_prefix = "etcd-v%s-linux-%s" % (ETCD_VERSION, arch),
            urls = mirror("https://github.com/coreos/etcd/releases/download/v%s/etcd-v%s-linux-%s.tar.gz" % (ETCD_VERSION, ETCD_VERSION, arch)),
        )

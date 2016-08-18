<!-- BEGIN MUNGE: UNVERSIONED_WARNING -->

<!-- BEGIN STRIP_FOR_RELEASE -->

<img src="http://kubernetes.io/kubernetes/img/warning.png" alt="WARNING"
     width="25" height="25">
<img src="http://kubernetes.io/kubernetes/img/warning.png" alt="WARNING"
     width="25" height="25">
<img src="http://kubernetes.io/kubernetes/img/warning.png" alt="WARNING"
     width="25" height="25">
<img src="http://kubernetes.io/kubernetes/img/warning.png" alt="WARNING"
     width="25" height="25">
<img src="http://kubernetes.io/kubernetes/img/warning.png" alt="WARNING"
     width="25" height="25">

<h2>PLEASE NOTE: This document applies to the HEAD of the source tree</h2>

If you are using a released version of Kubernetes, you should
refer to the docs that go with that version.

Documentation for other releases can be found at
[releases.k8s.io](http://releases.k8s.io).
</strong>
--

<!-- END STRIP_FOR_RELEASE -->

<!-- END MUNGE: UNVERSIONED_WARNING -->

# Gubernator

*This document is oriented at developers who want to use Gubernator to debug while developing for Kubernetes.*

<!-- BEGIN MUNGE: GENERATED_TOC -->

- [Gubernator](#gubernator)
  - [What is Gubernator?](#what-is-gubernator)
  - [Gubernator Features](#gubernator-features)
    - [Test Failures list](#test-failures-list)
    - [Log Filtering](#log-filtering)
    - [Gubernator for Local Tests](#gubernator-for-local-tests)

<!-- END MUNGE: GENERATED_TOC -->

## What is Gubernator?

[Gubernator](https://k8s-gubernator.appspot.com/) is a webpage for viewing and filtering through Kubernetes
 test results. Gubernator runs on Google App Engine and fetches logs stored on GCS.

## Gubernator Features

### Test Failures list

Issues made by k8s-merge-robot will post a link to a page listing the failed tests.
Each failed test comes with the corresponding error log from a junit file and a link
to filter logs for that test.

Based on the message logged in the junit file, the pod name may be displayed.

![alt text](https://github.com/kubernetes/kubernetes/docs/devel/gubernator-images/testfailures.png)


### Log Filtering

The log filtering page comes with checkboxes and textboxes to aid in filtering. Filtered keywords will be bolded
and lines including keywords will be highlighted. Up to four lines around the line of interest will also be displayed.

![alt text](https://github.com/kubernetes/kubernetes/docs/devel/gubernator-images/filterpage.png)

If less than 100 lines are skipped, the "... skipping xx lines ..." message can be clicked to expand and show
the hidden lines.

Before expansion:
![alt text](https://github.com/kubernetes/kubernetes/docs/devel/gubernator-images/skipping1.png)
After expansion:
![alt text](https://github.com/kubernetes/kubernetes/docs/devel/gubernator-images/skipping2.png)

If the pod name was displayed in the Test Failures list, it will automatically be filled in and filtered.
If it is not found in the error message, it can be entered into the textbox. Once a pod name
is entered, the Pod UID, Namespace, and ContainerID may be automatically filled in as well. These can be
altered as well. To apply the filter check off the checkbox corresponding to the filter.

![alt text](https://github.com/kubernetes/kubernetes/docs/devel/gubernator-images/filterpage1.png)

To add a filter, type the term to be filtered into the textbox labeled "Add filter:" and press enter. 
Additional filters will be displayed as checkboxes under the textbox.

![alt text](https://github.com/kubernetes/kubernetes/docs/devel/gubernator-images/filterpage3.png)

To choose which logs to view check off the checkboxes corresponding to the logs of interest. If multiple logs are
checked off, the "Weave by timestamp" option can weave the selected logs together based on the timestamp in each line.

![alt text](https://github.com/kubernetes/kubernetes/docs/devel/gubernator-images/filterpage2.png)

### Gubernator for Local Tests

*Currently Gubernator can only be used with remote node e2e tests.*

**NOTE: Using Gubernator with local tests will publically upload your test logs to Google Cloud Storage**

To use Gubernator to view logs from local test runs, set the GUBERNATOR tag to true.
A URL link to view the test results will be printed to the console.
Please note that running with the Gubernator tag will bypass the user confirmation for uploading to GCS.

```console

$ make test-e2e-node REMOTE=true GUBERNATOR=true
...
================================================================
Running gubernator.sh

Gubernator linked below:
k8s-gubernator.appspot.com/build/yourusername-g8r-logs/logs/e2e-node/timestamp
```

The gubernator.sh script can be run after running a remote node e2e test for the same effect.

```console
$ ./test/e2e_node/gubernator.sh
Do you want to run gubernator.sh and upload logs publicly to GCS? [y/n]y
...
Gubernator linked below:
k8s-gubernator.appspot.com/build/yourusername-g8r-logs/logs/e2e-node/timestamp
```

<!-- BEGIN MUNGE: GENERATED_ANALYTICS -->
[![Analytics](https://kubernetes-site.appspot.com/UA-36037335-10/GitHub/docs/devel/gubernator.md?pixel)]()
<!-- END MUNGE: GENERATED_ANALYTICS -->

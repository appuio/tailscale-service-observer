# tailscale-service-observer

[![Build](https://img.shields.io/github/workflow/status/vshn/go-bootstrap/Test)][build]
![Go version](https://img.shields.io/github/go-mod/go-version/vshn/go-bootstrap)
[![Version](https://img.shields.io/github/v/release/vshn/go-bootstrap)][releases]
[![Maintainability](https://img.shields.io/codeclimate/maintainability/vshn/go-bootstrap)][codeclimate]
[![Coverage](https://img.shields.io/codeclimate/coverage/vshn/go-bootstrap)][codeclimate]
[![GitHub downloads](https://img.shields.io/github/downloads/vshn/go-bootstrap/total)][releases]

[build]: https://github.com/vshn/go-bootstrap/actions?query=workflow%3ATest
[releases]: https://github.com/vshn/go-bootstrap/releases
[codeclimate]: https://codeclimate.com/github/vshn/go-bootstrap

Tailscale service observer is a Go tool which watches Kubernetes services in a single namespace and updates the advertised routes of a Tailscale client over the client's HTTP API (`tailscale web`).

## Configuration

The observer expects to run in a context with a working Kubernetes configuration (either via kubeconfig file or in-cluster).

The environment variable `TARGET_NAMESPACE` must be set to the namespace(s) in which the observer should watch services.
You can specify multiple namespaces separated by commas.
The environment variable `TAILSCALE_API_URL` can be used to provide a custom URL for the Tailscale client's HTTP API.
By default, the observer expects the API to be reachable at `http://localhost:8088`.
The environment variable `OBSERVER_ADDITIONAL_ROUTES` can be used to advertise additional routes.
You can specify multiple routes separated by commas.
Entries which can be parsed as an IP address will be advertised as a `<ip-address>/32` route.
Entries which can be parsed as a CIDR prefix will be advertised as that prefix route.

See the [subnet-router.yaml](./examples/subnet-router.yaml) for an example deployment.

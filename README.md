# tailscale-service-observer

[![Build](https://img.shields.io/github/workflow/status/appuio/tailscale-service-observer/Test)][build]
![Go version](https://img.shields.io/github/go-mod/go-version/appuio/tailscale-service-observer)
[![Version](https://img.shields.io/github/v/release/appuio/tailscale-service-observer)][releases]
[![Maintainability](https://img.shields.io/codeclimate/maintainability/appuio/tailscale-service-observer)][codeclimate]
[![Coverage](https://img.shields.io/codeclimate/coverage/appuio/tailscale-service-observer)][codeclimate]
[![GitHub downloads](https://img.shields.io/github/downloads/appuio/tailscale-service-observer/total)][releases]

[build]: https://github.com/appuio/tailscale-service-observer/actions?query=workflow%3ATest
[releases]: https://github.com/appuio/tailscale-service-observer/releases
[codeclimate]: https://codeclimate.com/github/appuio/tailscale-service-observer

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

See the [examples](./examples/) for Kubernetes manifests to get started.

## Expose the in-cluster DNS server in your tailnet

To expose the in-cluster DNS server in your tailnet, you can advertise the in-cluster DNS IP address by adding it in the environment variable `OBSERVER_ADDITIONAL_ROUTES`.

Additionally, you can enable Tailscale's MagicDNS feature, and add the in-cluster DNS server as an additional resolver.
We recommend that you enable "Restrict to search domain", and set `svc.cluster.local` as the search domain.
This allows you to use the in-cluster service DNS names (`<service>.<namespace>.svc.cluster.local`) to access advertised services.

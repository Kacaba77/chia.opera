# chia-operator

NOTE: This is a very early implementation. Features may come in at a fast pace, bugs may exist in plenty, and the CRs themselves may change drastically during this stage. In short: this code is not guaranteed to work. That said, it is being ran as a couple of farms on the mainnet and testnet10 networks today.

Kubernetes operator for managing Chia components in kubernetes. Currently supported components:

 - full_nodes
 - farmers
 - harvesters

Applying a CR for each component allows you to instantiate a configured instance of that component that is able to communicate to other requisite components in the cluster. A whole farm can be ran with each component isolated in its own pod, with a chia-exporter sidecar to scrape Prometheus metrics.

ChiaCA is an additional CRD that generates a certificate authority for Chia components and places it in a kubernetes Secret as a convenience. Alternatively, users can pre-generate their own CA Secret with data keys for: `chia_ca.crt`, `chia_ca.key`, `private_ca.crt`, and `private_ca.key`.

## Getting Started

// TODO write a user guide for installing the operator and various Chia component CRs.

## TODO

- Add support for other Chia components (wallet, timelord, dns-introducer, etc)
- Make chia-exporter an optional container in the pod

## License

Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

# AwesomWasm 2024 Interchaintest Workshop

This repository is the starting point for the AwesomWasm 2024 Interchaintest Workshop. The workshop is a hands-on introduction to testing IBC enabled CosmWasm applications using [interchaintest](https://github.com/strangelove-ventures/interchaintest) and [go-codegen](https://github.com/srdtrk/go-codegen). The workshop is designed to be self-paced and is suitable for developers with a basic understanding of CosmWasm and golang.

The content of the workshop is contained and should be followed in the [go-codegen documentation](https://srdtrk.github.io/go-codegen/).

This repo contains a CosmWasm smart contract (`cw-ica-controller`) that will be used to demonstrate the testing process. Learn more about the contract in its own repository [here](https://github.com/srdtrk/cw-ica-controller). This repository is a fork of the `cw-ica-controller` at commit [`3598978`](https://github.com/srdtrk/cw-ica-controller/tree/3598978b4627501a22f84ff97f9a2810e9d336ad) with `e2e`, `.github` and `derive` directories removed, and README.md modified.

# namespace-crawler
inspired by night-crawler


# Namespace Crawler for Kubernetes Secrets

This project is a Kubernetes application written in Go that watches for Secret resources in all namespaces and performs specific actions based on certain labels. It utilizes the Kubernetes Client-Go library to interact with the Kubernetes API server and handle Secret events such as creation, update, and deletion.

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Usage](#usage)
- [Environment Variables](#environment-variables)
- [Building and Running](#building-and-running)
- [Docker Deployment](#docker-deployment)
- [Kubernetes Deployment](#kubernetes-deployment)


## Overview

This application acts as a "namespace crawler," watching for Kubernetes Secret events and performing specified actions when a Secret has a particular label. It is designed to replicate Secrets across specified namespaces and can handle both creation and updates of Secrets.

The project is built using Go and relies on the Kubernetes Client-Go library to watch for Secret events and take action based on the presence of specific labels.

## Features

- **Secret Watcher**: Monitors Secret resources across all namespaces.
- **Label-based Actions**: Performs actions based on the presence of a specific label in Secrets.
- **Replication**: Replicates Secrets to multiple namespaces as defined by the labels.
- **Configurable via Environment Variables**: The label key and other parameters can be customized using environment variables.

## Prerequisites

Before you can build and run this application, you must have the following installed on your system:

- [Go](https://golang.org/doc/install) (version 1.23 or later)
- [Docker](https://docs.docker.com/get-docker/)
- [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) (for interacting with your Kubernetes cluster)
- A running [Kubernetes cluster](https://kubernetes.io/docs/setup/) with appropriate access permissions

## Installation

To clone the repository and navigate into the project directory, use the following commands:

```bash
git clone https://github.com/yourusername/namespace-crawler.git
cd namespace-crawler
```
Next, install the required Go modules:
```bash
go mod tidy
```

## Usage
### Environment Variables
This application uses environment variables to configure its behavior:

- RESPONSIBILITY_LABEL_KEY: The key of the label used to identify which Secrets to act on. Default is "namespace-crawler-responsibility".
- NAMESPACE_LIST_KEY: The key of the label used to identify the master namespace. Default is "namespace-crawler-responsible-for".

## Building and Running
You can build and run the application locally using the following commands:

```bash
go build -o secret-watcher main.go
./secret-watcher
```
## Docker Deployment
### Build the Docker Image
To build a Docker image for the application, use the provided Dockerfile:

```bash
docker build -t yourusername/namespace-crawler:latest .
```
### Run the Docker Container
To run the application inside a Docker container:

```bash
docker run --rm -it \
  -v ~/.kube/config:/root/.kube/config:ro \
   yourusername/namespace-crawler:latest
```
## Kubernetes Deployment
To deploy the application to a Kubernetes cluster, use the provided Kubernetes deployment YAML file:

### Service Account and Role Configuration

Ensure you have set up the necessary service account and roles for the application. The application needs permissions to read, create, update, and delete Secrets across all namespaces.

### Apply Deployment

Apply the Kubernetes deployment:
```bash
kubectl apply -f kubernetes-controller.yaml
```

Check Deployment Status
Verify the deployment status:
```bash
kubectl get deployments
```
Logs

Check the logs of the running pods to verify everything is working as expected:
```bash
kubectl logs -l app.kubernetes.io/name=namespace-crawler
```


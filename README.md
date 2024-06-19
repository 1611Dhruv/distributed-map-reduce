# Distributed MapReduce on Google Cloud Platform (GCP)

This project implements a distributed MapReduce framework on Google Cloud Platform (GCP) using Golang. It includes a coordinator and multiple worker nodes that communicate over RPC for parallel data processing tasks.

## Overview

The project consists of the following components:

- **Coordinator**: Manages the assignment of Map and Reduce tasks to worker nodes. It coordinates the overall execution of the MapReduce job, handles task scheduling, and manages intermediate data.

- **Worker Nodes**: Execute Map and Reduce tasks assigned by the coordinator. Workers read input data from Google Cloud Storage (GCS), process it according to the Map and Reduce functions, and store intermediate results back into GCS.

## Features

- **Scalability**: Utilizes Kubernetes for scalable deployment of worker nodes. Kubernetes ensures dynamic scaling based on workload demands, optimizing resource utilization.

- **Fault Tolerance**: Implements fault-tolerant mechanisms where tasks are reassigned in case of worker failures. The coordinator monitors task completion and handles failures gracefully.

- **Cloud Storage Integration**: Uses GCP's Cloud Storage for storing input data and intermediate results. This ensures data durability and accessibility across distributed nodes.

- **RPC Communication**: Workers and the coordinator communicate via RPC (Remote Procedure Call), enabling efficient task distribution and result collection.

## Setup Instructions

### Prerequisites

- Install Golang (version X.X.X)
- Set up Google Cloud Platform project and enable necessary APIs (Compute Engine, Kubernetes Engine, Cloud Storage)
- Set up authentication for GCP (Service Account with appropriate roles)

### Deployment

1. **Coordinator Deployment**:

   - Deploy the coordinator as a Kubernetes service:
     ```bash
     kubectl apply -f coordinator.yaml
     ```
   - Expose the coordinator service to external access if needed.

2. **Worker Deployment**:
   - Deploy workers as Kubernetes pods managed by a deployment:
     ```bash
     kubectl apply -f worker.yaml
     ```
   - Workers should automatically scale based on the Kubernetes deployment settings.

### Running the Application

1. **Build and Deploy Plugins**:

   - Build plugins for Map and Reduce functions using `go build -buildmode=plugin mrapps/your_plugin.go`.
   - Upload plugins to GCS or directly use them in the worker deployment.

2. **Start Coordinator**:

   - Start the coordinator:
     ```bash
     go run mrcoordinator.go gs://your-input-bucket/input-files/
     ```
   - Replace `gs://your-input-bucket/input-files/` with the actual Cloud Storage path to input files.

3. **Start Workers**:
   - Start worker nodes:
     ```bash
     go run mrworker.go gs://your-output-bucket/
     ```
   - Replace `gs://your-output-bucket/` with the Cloud Storage path for storing intermediate and output files.

## Contributing

Contributions are welcome! Please fork the repository and create a pull request with your proposed changes.

## License

This project is licensed under the MIT License - see the LICENSE file for details.


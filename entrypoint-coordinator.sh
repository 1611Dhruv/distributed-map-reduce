#!/bin/sh
set -e

# Perform any setup tasks here (if needed)
echo "Starting Coordinator Initialization..."

# Run the coordinator program
echo "Starting Coordinator..."
go run mrcoordinator.go "$@"


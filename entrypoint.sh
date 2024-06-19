#!/bin/sh
set -e

# Handle dynamic build commands based on input
if [ "$1" = "build" ]; then
    shift
    # Run the plugin build command
    go build -buildmode=plugin "$@"
else
    # Default behavior if no specific command is given
    echo "No command specified."
    exit 1
fi


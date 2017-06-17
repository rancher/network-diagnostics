#!/bin/bash

setup-cli.sh

if [ $? -ne 0 ]; then
    echo "error configuring CLI"
    exit 1
fi

network-diagnostics "$@"

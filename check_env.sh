#!/bin/bash

VAL=$(eval "echo \"\$$1\"")

if [ "$VAL" = "" ]; then
    echo "Error: environment variable $1 needs to be set"
    exit 1
fi

exit 0


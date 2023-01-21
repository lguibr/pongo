#!/bin/bash

# Concatenate all Go files in the project into a single file
for file in $(find . -name "*.go" -not -path "./vendor/*")
do
    cat $file >> ./.sourcecode.go
done
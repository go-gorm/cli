#!/bin/bash

# Generate development files

echo "Generating development files for ./examples/typed..."
go run ./main.go gen -i ./examples/ -o ./examples/typed

echo "Generating development files for ./examples/output with --typed=false..."
go run ./main.go gen -i ./examples/ -o ./examples/output --typed=false

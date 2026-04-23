#!/bin/bash

set -e

# Change directory to the root of the repository
cd "$(dirname "$0")/.."

if ! command -v vhs &>/dev/null; then
  echo "vhs needed: https://github.com/charmbracelet/vhs"
  exit 1
fi

go build -o flak .
vhs demo/demo.tape

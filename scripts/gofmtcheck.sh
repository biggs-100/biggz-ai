#!/bin/bash
# Format check — fails if any .go file is not gofmt-compliant.
set -euo pipefail

unformatted=$(gofmt -l .)
if [ -n "$unformatted" ]; then
  echo "Unformatted files:"
  echo "$unformatted"
  exit 1
fi
echo "All files are properly formatted."

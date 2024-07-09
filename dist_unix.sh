#!/bin/bash

set -eo pipefail
shopt -s extglob

exiftool="https://exiftool.org/Image-ExifTool-12.76.tar.gz"

cd $(dirname "${BASH_SOURCE[0]}")

# Setup
rm -rf tmp/
mkdir -p tmp/

# Download Exiftool
curl -L# "$exiftool" | tar xz -C tmp/
mv tmp/* tmp/exiftool

# Cleanup and test
pushd tmp/exiftool
rm -rf !(exiftool|lib|t|README)
find lib -name '*.pod' -delete
prove -l lib -b t 
rm -rf t
./exiftool -ver -v
popd

# Move to destination
rm -rf ${1:-dist}
mv tmp/exiftool ${1:-dist}

# Cleanup
rm -rf tmp/

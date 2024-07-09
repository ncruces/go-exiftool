#!/bin/bash

set -eo pipefail
shopt -s extglob

exiftool="https://exiftool.org/Image-ExifTool-12.76.tar.gz"
strawberry="https://strawberryperl.com/download/5.32.1.1/strawberry-perl-5.32.1.1-64bit-portable.zip"

cd $(dirname "${BASH_SOURCE[0]}")

# Setup
rm -rf tmp/
mkdir -p tmp/

# Download Exiftool
curl -L# "$exiftool" | tar xz -C tmp/
mv tmp/* tmp/exiftool

# Download Strawberry
curl -L# "$strawberry" --output tmp/strawberry.zip
unzip -qd tmp/strawberry/ tmp/strawberry.zip

# Install modules
PATH=`pwd`/tmp/strawberry/perl/bin:$PATH
PATH=`pwd`/tmp/strawberry/c/bin:$PATH
cpanm.bat --notest Win32::FindFile

# Cleanup Strawberry
pushd tmp/strawberry/perl
rm -rf lib/CORE lib/CPAN?(.pm) lib/Pod
rm -rf lib/Module lib/Software lib/ExtUtils
rm -rf lib/perl5db.pl
rm -rf lib/Devel lib/Test*
rm -rf lib/auto/Devel lib/auto/Test*
rm -rf lib/Unicode lib/Encode/+(CN|JP|KR|TW)?(.pm)
rm -rf lib/auto/Unicode lib/auto/Encode/+(CN|JP|KR|TW)
find lib -name '.packlist' -delete
find lib -name '*.pod' -delete
find lib -type d -empty -delete
popd

# Embed Strawberry
mkdir -p tmp/exiftool/lib/Win32/
cp -rl tmp/strawberry/perl/bin/perl.exe tmp/exiftool/exiftool.exe
cp -rl tmp/strawberry/perl/bin/*.dll tmp/exiftool
cp -rl tmp/strawberry/perl/bin/ tmp/exiftool
cp -rl tmp/strawberry/perl/lib/ tmp/exiftool
cp -rl tmp/strawberry/perl/vendor/lib/Win32/API* tmp/exiftool/lib/Win32
cp -rl tmp/strawberry/perl/vendor/lib/auto/Win32/API* tmp/exiftool/lib/auto/Win32
cp -rl tmp/strawberry/perl/site/lib/Win32/FindFile* tmp/exiftool/lib/Win32
cp -rl tmp/strawberry/perl/site/lib/auto/Win32/FindFile* tmp/exiftool/lib/auto/Win32

# Cleanup and test
pushd tmp/exiftool
rm -rf !(exiftool*|*.dll|bin|lib|t|README)
find lib -name '*.pod' -delete
bin/prove.bat -b t
rm -rf bin t
./exiftool.exe exiftool -ver -v
popd

# Move to destination
rm -rf ${1:-dist}
mv tmp/exiftool ${1:-dist}

# Cleanup
rm -rf tmp/

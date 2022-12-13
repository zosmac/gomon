#!/bin/bash
# Copyright © 2021 The Gomon Project.

# define Gomon version value
version="2.0.0"

# Create LICENSE file

license() {
   file=`mktemp`
   srcpath=$(dirname `pwd`)
   rm -f $srcpath/LICENSE
   cat <<EOF >$file
Copyright © 2021 The Gomon Project.

This product includes software with separate copyright notices and license terms as noted below.
EOF

   go list -m -f "{{if .Dir}}{{.Dir}} {{.Path}}{{end}}" all | sort -u | while read i j;do printf "\n======== %s ========\n\n" $j; cat $i/LICENSE*;done 2>/dev/null >>$file
   go list -m -f='{{.Dir}}' all | sort -u | xargs -I% sh -c "ls -1d %/LICENSE*" 2>/dev/null | while read i;do j=`dirname $i`;if [[ "$k" != "$j" ]];then printf '\nFor %s:\n' $j;fi;k=$j;echo;cat $i;done >>$file
   mv $file ../LICENSE
}

go mod tidy
license

# Identify the module

module=`go list -m`

# Generate Dependencies

dependencies=$(go list -m -f='{{.Dir}} {{.Path}} {{.Version}}' all | while read i j k;do if [[ "$i" == "${i#`pwd`}" && -n "$k" ]]; then echo $'\t//' "$j" "$k"; fi; done)

echo -n \
$'// Code generated  DO NOT EDIT.
// Copyright © 2021 The Gomon Project.

package core

import (
\t"os"
)

var (
\t// Srcpath to strip from source file path in log messages.
\tSrcpath = "'$srcpath$'"

\t// module identifies the import package path for this module.
\tmodule = "'$module$'"

\t// executable identifies the full command path.
\texecutable, _ = os.Executable()

\t// buildDate sets the build date for the command.
\tbuildDate = func() string {
\t\tinfo, _ := os.Stat(executable)
\t\treturn info.ModTime().UTC().Format("2006-01-02 15:04:05 UTC")
\t}()

\t// vmmp is "Version, Major, Minor, Patch"
\tvmmp = "v'$version'-'`git rev-parse --short=12 HEAD`$'"

\t// Dependencies import paths and versions
'"$dependencies"'
)
' >version.go

cat version.go

#!/bin/bash
# Copyright © 2021 The Gomon Project.

# define Gomon version value
version="1.0.0"

# Create LICENSE file

license() {
   file=`mktemp`
   if [ -n "$1" ]; then
      export GOOS=$1
      srcpath=`go env GOPATH`/src
   else
      srcpath=$(dirname `pwd`)
      rm -f $srcpath/LICENSE
   fi
   cat <<EOF >$file
Copyright © 2021 The Gomon Project.

This product includes software with separate copyright notices and license terms as noted below.
EOF

   if [ -f ../go.mod ]; then
      go list -m -f "{{if .Dir}}{{.Dir}} {{.Path}}{{end}}" all | sort -u | while read i j;do printf "\n======== %s ========\n\n" $j; cat $i/LICENSE*;done 2>/dev/null >>$file
      go list -m -f='{{.Dir}}' all | sort -u | xargs -I% sh -c "ls -1d %/LICENSE*" 2>/dev/null | while read i;do j=`dirname $i`;if [[ "$k" != "$j" ]];then printf '\nFor %s:\n' $j;fi;k=$j;echo;cat $i;done >>$file
      mv $file ../LICENSE
   else
      go list -f '{{range .Deps}}{{println .}}{{end}}' ./... | sort -u | xargs go list -f '{{if not .Standard}}{{.Dir}}{{end}}' | sort -u | while read i;do if [ $i == ${i#`pwd`} ];then echo $i;fi;done | xargs -I% sh -c "ls -1d %/LICENSE*" 2>/dev/null | while read i;do j=`dirname ${i#$srcpath}`;if [[ "$k" != "$j" ]];then printf '\nFor %s:\n' $j;fi;k=$j;echo;cat $i;done >>$file
      mv $file ../LICENSE.$GOOS
   fi
}

if [ -f ../go.mod ]; then
   go mod tidy
   license
else
   license darwin
   license linux
   license windows
fi

# Generate Dependencies

if [ -f ../go.mod ]; then
   dependencies=$(go list -m -f='{{.Dir}} {{.Path}} {{.Version}}' all | while read i j k;do if [[ "$i" == "${i#`pwd`}" && -n "$k" ]]; then echo "$j" "$k"; fi; done)
else
   if [ -d vendor ]; then
      cd vendor
   else
      cd $srcpath
   fi
   dependencies=$(find . -name '.git' | while read i;do cd $i;j=`cat $(cat HEAD | sed -n -E 's/ref: (.*)/\1/p')`;printf '  %s  %s\n' ${j::10} `dirname ${i:2}`;cd $OLDPWD;done)
   cd $OLDPWD # use OLDPWD, because cd - echoes the original directory
fi

echo -n \
"// Copyright © 2021 The Gomon Project.

package core

var (
	// srcpath to strip from source file path in log messages.
	srcpath = \"$srcpath\"

	// vmmp is \"Version, Major, Minor, Patch\"
	vmmp = \"v$version-`git rev-parse --short=12 HEAD`\"

	// Dependencies import paths and versions
	dependencies = \`
$dependencies
\`
)
" >version.go

cat version.go

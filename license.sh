#!/bin/bash
# Copyright © 2021 The Gomon Project.

# Create LICENSE file

license() {
   file=`mktemp`
   rm -f LICENSE
   cat <<EOF >$file
Copyright © 2021 The Gomon Project.

This product includes software with separate copyright notices and license terms as noted below.
EOF

   go list -m -f "{{if .Dir}}{{.Dir}} {{.Path}}{{end}}" all | sort -u | while read i j;do printf "\n======== %s ========\n\n" $j; cat $i/LICENSE*;done 2>/dev/null >>$file
   mv $file LICENSE
}

go mod tidy
license

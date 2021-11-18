// Copyright © 2021 The Gomon Project.

/*
Package log implements the log observation of the "gomon" command.

The log package defines the following command line flags:
* -loglevel: the log level value below which to filter out log records

For Darwin, the local OSLogStore and syslog are monitored.

For Linux, the following flags identify log files to monitor:
* -logdirectory:    the path to the top of a directory hierarchy of logs to monitor
* -logregex:        a regular expression to identify log file names to include
* -logregexexclude: a regular expression to identify log file names to exclude

In addition, the "gomon" process observer identifies all the files open on the system.
Only log files that are actually open are monitored.
*/
package log

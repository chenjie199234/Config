#!/bin/sh
# kubernetes probe port
port8000=`netstat -ltn | grep 8000 | wc -l`
port9000=`netstat -ltn | grep 9000 | wc -l`
if [[ $port9000 -eq 1 && $port8000 -eq 1 ]]
then
exit 0
else
exit 1
fi

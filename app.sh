#!/bin/bash
echo "ECHO IS TO BE STARTED"

for ((i=1;i<=3;i++));
do
   # your-unix-command-here
   echo aaaa >> /tmp/hola.log
   sleep 1
done


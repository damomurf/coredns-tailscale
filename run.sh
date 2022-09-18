#!/bin/sh

/coredns -conf /Corefile &
PIDS="$PIDS $!"

for pid in $PIDS; do
  wait $pid || let "DONE=1"
done

if [ "$DONE" -eq 1 ]; then
  exit 1
fi

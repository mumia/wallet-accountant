#!/usr/bin/env sh

trap 'kill ${!}; exit;' SIGINT SIGTERM

# wait indefinitely
while true
do
  tail -f /dev/null & wait ${!}
done
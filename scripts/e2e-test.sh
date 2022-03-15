#!/bin/bash
timeout=60

kubectl annotate ns podinfo k8s-pause/suspend=true --overwrite
used=0

while true; do
  countTotal=$(kubectl -n podinfo get pods  | grep podinfo |  wc -l)
  countSuspended=$(kubectl -n podinfo get pods  | grep Suspended |  wc -l)

  if [[ "$countTotal" == "$countSuspended" ]]; then
    break;
  fi

  if [[ $used -gt $timeout ]]; then
    echo "Unable to suspend pods"
    exit 1
  fi

  used=$((used + 2))
  sleep 2
done

kubectl annotate ns podinfo k8s-pause/suspend=false --overwrite
used=0

while true; do
  countTotal=$(kubectl -n podinfo get pods  | grep podinfo |  wc -l)
  countRunning=$(kubectl -n podinfo get pods  | grep Running |  wc -l)

  if [[ "$countTotal" == "$countRunning" ]]; then
    break;
  fi

  if [[ $used -gt $timeout ]]; then
    echo "Unable to resume pods"
    exit 1
  fi

  used=$((used + 2))
  sleep 2
done

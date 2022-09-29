#!/bin/bash
set -xe

OFFSET=""

if [ "${IS_ROOT:="false"}" = true ]
then
    OFFSET=0
else
    OFFSET=1
fi

SET_INDEX=$((${HOSTNAME##*-} + $OFFSET))

MOUNT="cert"

kubectl get secret $SECRET_PREFIX-$SET_INDEX -ojsonpath="{.data['tls\.key']}" | base64 -d > mnt/$MOUNT/tls.key
kubectl get secret $SECRET_PREFIX-$SET_INDEX -ojsonpath="{.data['tls\.crt']}" | base64 -d > mnt/$MOUNT/tls.crt
kubectl get secret $SECRET_PREFIX-$SET_INDEX -ojsonpath="{.data['Node-ID']}" | base64 -d > /mnt/$MOUNT/node-id
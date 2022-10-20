#!/bin/bash
set -xe

NETWORK_ID=network-1002

HTTP_PARAMS="--http-host=0.0.0.0 --http-allowed-origins=* --http-port=9650"
STAKING_PARAMS="--staking-tls-key-file=/mnt/cert/tls.key --staking-tls-cert-file=/mnt/cert/tls.crt --staking-port=9651"
API_NODE_PARAMS="--index-enabled"

BOOTSTRAP_PARAMS="--api-admin-enabled=true --log-level=debug"

# if [ ! -d "/mnt/data/$NETWORK_ID" ];
# then 
if [ "${IS_ROOT:-"false"}" = true ] && [ ! -d "/mnt/data/$NETWORK_ID" ];
then 
    BOOTSTRAP_PARAMS="$BOOTSTRAP_PARAMS --bootstrap-ids= --bootstrap-ips="
else 
    ROOT_PORT="${NETWORK_NAME^^}_ROOT_SERVICE_PORT_STAKING"
    ROOT_HOST="${NETWORK_NAME^^}_ROOT_SERVICE_HOST"
    BOOTSTRAP_PARAMS="$BOOTSTRAP_PARAMS --bootstrap-ids=NodeID-$ROOT_NODE_ID --bootstrap-ips=${!ROOT_HOST}:${!ROOT_PORT}"
fi
# fi

CMD="--network-id=$NETWORK_ID --public-ip=$POD_IP --db-dir=/mnt/data --genesis=/mnt/conf/genesis.json $BOOTSTRAP_PARAMS $HTTP_PARAMS"
if [ "${IS_API_NODE:="false"}" = true ];
then
    CMD="$CMD $API_NODE_PARAMS"
else
    CMD="$CMD $STAKING_PARAMS"
fi

echo $CMD > cmd.txt

./camino-node $CMD
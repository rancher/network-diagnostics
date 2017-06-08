#!/bin/bash

echo "Starting CLI config"

if [ -z ${CATTLE_URL} ]; then
    echo "Error: CATTLE_URL not specified"
    exit 1
fi

if [ -z ${CATTLE_ACCESS_KEY} ]; then
    echo "Error: CATTLE_ACCESS_KEY not specified"
    exit 1
fi

if [ -z ${CATTLE_SECRET_KEY} ]; then
    echo "Error: CATTLE_SECRET_KEY not specified"
    exit 1
fi

# Figure out the current environment uuid
CUR_ENV_UUID=`curl -s 169.254.169.250/2016-07-29/self/host/environment_uuid`
if [ $? -ne 0 ]; then
    echo "Error: couldn't figure out current enviornment uuid from metadata"
    exit 1
fi

if [ -z ${CUR_ENV_UUID} ]; then
    echo "Error: couldn't find current enviornment uuid in metadata"
fi

CATTLE=$(dirname ${CATTLE_URL})

CUR_ENV_ID=`curl -s ${CATTLE}/v2-beta/projects?uuid=${CUR_ENV_UUID} | jq .data[0].id`
if [ $? -ne 0 ]; then
    echo "Error: couldn't get environment id from metadata"
    exit 1
fi

if [ -z ${CUR_ENV_ID} ]; then
    echo "Error: couldn't find current enviornment id in metadata"
fi


mkdir -p ${HOME}/.rancher
cat > ${HOME}/.rancher/cli.json << EOF
{"accessKey":"${CATTLE_ACCESS_KEY}","secretKey":"${CATTLE_SECRET_KEY}","url":"${CATTLE}/v2-beta/schemas","environment":${CUR_ENV_ID}}
EOF

echo "CLI config successful"


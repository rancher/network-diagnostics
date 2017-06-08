#!/bin/bash


type rancher >/dev/null 2>&1 || { echo >&2 "I require rancher CLI but it's not installed.  Aborting."; exit 1; }

LOGS_DIR=$1
LOGFILE_NAME=$2
HISTORY_LENGTH=$3

if [ "${LOGS_DIR}" == "" ]; then
    LOGS_DIR=`mktemp -d -t rancher.logs.XXXXXXX`
    if [ $? -ne 0 ]; then
        echo "error: couldn't create tmp directory"
        exit 1
    fi
fi

if [ "${LOGFILE_NAME}" == "" ]; then
    LOGFILE_NAME="rancher-logs-$(date -u +%s%N)"
fi

if [ "${HISTORY_LENGTH}" == "" ]; then
    HISTORY_LENGTH=3
fi

echo "Collecting logs to directory: ${LOGS_DIR}"

cd ${LOGS_DIR}

TMP_DIR="dump.$(date -u +%s%N)"
rm -rf ${TMP_DIR}
mkdir -p ${TMP_DIR}
cd ${TMP_DIR}

# TODO: Check for exit codes

rancher host ls -a > rancher-host-ls-a.log 2>&1
rancher ps -s -a > rancher-ps-s-a.log 2>&1
rancher ps -c -a > rancher-ps-c-a.log 2>&1

CONTAINERS=`rancher ps -c -a -s`
echo "Collecting rancher-agent logs"
echo "${CONTAINERS}" | grep rancher-agent | awk '{system("rancher logs --tail=-1 "$1" > "$2"-"$5"-"$1".log 2>&1");}'

echo "Collecting ipsec logs"
echo "${CONTAINERS}" | grep ipsec | awk '{system("rancher logs --tail=-1 "$1" > "$2"-"$5"-"$1".log 2>&1");}'

echo "Collecting network-services logs"
echo "${CONTAINERS}" | grep network-services | awk '{system("rancher logs --tail=-1 "$1" > "$2"-"$5"-"$1".log 2>&1");}'

echo "Collecting healthcheck logs"
echo "${CONTAINERS}" | grep healthcheck | awk '{system("rancher logs --tail=-1 "$1" > "$2"-"$5"-"$1".log 2>&1");}'

echo "Collecting scheduler logs"
echo "${CONTAINERS}" | grep scheduler | awk '{system("rancher logs --tail=-1 "$1" > "$2"-"$5"-"$1".log 2>&1");}'

NS_CONTAINERS=`echo "${CONTAINERS}" | grep network-support-agent`
if [ "${NS_CONTAINERS}" != "" ]; then
    echo "Collecting network-support-agent logs"
    echo "${NS_CONTAINERS}" | grep network-support-agent | awk '{system("mkdir -p "$5";rancher --host "$5" docker cp "$7":/logs ./"$5"/")}'
else
    echo "Collecting network-support-agent logs: service not running"
fi

cd ..

mv ${TMP_DIR} ${LOGFILE_NAME}
zip -rq ${LOGFILE_NAME}.zip ${LOGFILE_NAME}
rm -rf ${LOGFILE_NAME}

if [ "${HISTORY_LENGTH}" != "-1" ]; then
    TO_DELETE=$(ls -1t | sed '1,'${HISTORY_LENGTH}'d')
    if [ -n "${TO_DELETE}" ]; then
        rm -rf ${TO_DELETE}
    fi
fi

echo "Logs collected to file: ${LOGS_DIR}/${LOGFILE_NAME}.zip. Please send it across to Rancher Support"

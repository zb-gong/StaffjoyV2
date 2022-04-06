#!/bin/bash

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )""/../"
CACHE_MODE=$1

services=( "account" "bot" "company" "email" "sms" )
api_services=( "account" "company" )

set -a
source $ROOT/service_addrs.sh

$ROOT/scripts/kill.sh

for service in "${services[@]}"
do
    echo $service
    mkdir -p $ROOT/logs/$service/
    USE_CACHING=$CACHE_MODE $ROOT/$service/server/$service"server" > $ROOT/logs/$service/logs.txt 2>&1 &
done

for service in "${api_services[@]}"
do
    echo $service"api"
    mkdir -p $ROOT/logs/$service/
    USE_CACHING=$CACHE_MODE $ROOT/$service/api/$service"api" > $ROOT/logs/$service/api_logs.txt 2>&1 &
done

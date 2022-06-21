#!/bin/bash

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )""/../"

all="all"

if [ "$1" == "$all" ]; then
    services=( "account" "bot" "company" "email" "sms" "frontcache" )
    api_services=( "account" "company" "frontcache")
else
    services=( "$1" )
fi

for service in "${services[@]}"
do
    echo "==============================="
    cd $ROOT/$service/server
    go build
    mv server $service"server"
done

for service in "${api_services[@]}"
do
    echo "==============================="
    cd $ROOT/$service/api
    go build
    mv api $service"api"
done

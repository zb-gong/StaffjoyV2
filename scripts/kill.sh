#!/bin/bash

services=( "account" "bot" "company" "email" "sms" "frontcache")

for service in "${services[@]}"
do
    killall $service"server"
    killall $service"api"
done

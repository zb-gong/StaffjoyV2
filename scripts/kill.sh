#!/bin/bash

services=( "account" "bot" "company" "email" "sms" )

for service in "${services[@]}"
do
    killall $service"server"
    killall $service"api"
done

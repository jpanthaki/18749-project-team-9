#!/bin/bash
# script for starting clients, lfd, gfd, and servers
# types: lfd, server, client, gfd

echo "$0 usage: <type> <args>"

# starts lfd
# args <id> <gfdaddr>
start_lfd() {
    local lfdid="LFD$1"
    local serverid="S$1"
    local lfdport=$((9000 + $1))
    local gfdaddr="$2"
    local serverport=$((8080 + $1))
    go run lfd/lfdrunner/lfdrunner.go -id $lfdid -serverid $serverid \
    -port $lfdport -gfdaddr $gfdaddr
}

# args <id>
start_server() {
    local serverid="S$1"
    local lfdport=$((9000 + $1))
    local serverport=$((8080 + $1))
    go run server/srunner/srunner.go -id $serverid -port $serverport \
    -lfdPort $lfdport
}

# args <id> <s1> <s2> <s3>
start_client() {
    local clientid="C$1"
    local s1="$2"
    local s2="$3"
    local s3="$4"
    local think="0.5s"
    go run client/crunner/crunner.go -s1 $s1 -s2 $s2 -s3 $s3 -id $clientid -think $think
}

# args none
start_gfd() {
    go run gfd/gfdrunner/gfdrunner.go
}

if [ $1 = "lfd" ]; then
    echo "starting lfd $2.."
    start_lfd $2 $3 

elif [ $1 = "server" ]; then
    echo "starting server $2.."
    start_server $2

elif [ $1 = "client" ]; then
    echo "starting client $2.."
    start_client $2 $3 $4 $5

elif [ $1 = "gfd" ]; then
    echo "starting gfd.."
    start_gfd
fi

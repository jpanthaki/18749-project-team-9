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

# args <id> <replicationMode>{active, passive} <s1Addr> <s2Addr> <s3Addr>
start_server() {
    local serverid="S$1"
    local lfdport=$((9000 + $1))
    local serverport=$((8080 + $1))
    if [ $# -eq 2 ]; then
        echo "test"
        go run server/srunner/srunner.go -id $serverid -port $serverport \
        -lfdPort $lfdport -replicationMode $2
    elif [ $# -eq 5 ]; then
        go run server/srunner/srunner.go -id $serverid -port $serverport \
        -lfdPort $lfdport -replicationMode $2 -s1Addr $3 -s2Addr $4 -s3Addr $5
    fi
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
    if [ $# -ne 3 ]; then
        echo "incorect number of args: $# for LFD"
        echo "lfd args: <id> <gfdaddr>"
        exit -1
    fi
    echo "starting lfd $2.."
    start_lfd $2 $3 
    exit 0

elif [ $1 = "server" ]; then
    if [ $# -eq 3 ]; then
        echo "starting server with $# args.."
        start_server $2 $3
    elif [ $# -eq 6 ]; then
        echo "starting server with $# args.."
        start_server $2 $3 $4 $5 $6
    else
        echo "incorect number of args: $# for server"
        exit -1
    fi
    exit 0

elif [ $1 = "client" ]; then
    if [ $# -ne 5 ]; then
        echo "incorect number of args: $# for client"
        echo "client args: <id> <s1> <s2> <s3>"
        exit -1
    fi
    echo "starting client $2.."
    start_client $2 $3 $4 $5
    exit 0

elif [ $1 = "gfd" ]; then
    if [ $# -ne 1 ]; then
        echo "incorect number of args: $# for gfd"
        echo "gfd args: none"
        exit -1
    fi
    echo "starting gfd.."
    start_gfd
    exit 0
fi
echo "invalid type {lfd, server, client, gfd}"

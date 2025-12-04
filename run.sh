#!/bin/bash
# script for starting clients, lfd, gfd, and servers
# types: lfd, server, client, gfd

echo "$0 usage: <type> <args>"

# starts lfd
# args <id> <gfdaddr> <method>
start_lfd() {
    local lfdid="LFD$1"
    local serverid="S$1"
    local lfdport=$((9000 + $1))
    local gfdaddr="$2"
    local serverport=$((8080 + $1))
    local method="$3"
    go run lfd/lfdrunner/lfdrunner.go -id $lfdid -serverid $serverid \
    -port $lfdport -gfdaddr $gfdaddr -method $method
}

# args <id> <s1Addr> <s2Addr> <s3Addr>
start_server_active() {
    local serverid="S$1"
    local lfdport=$((9000 + $1))
    local serverport=$((8080 + $1))
    go run server-active/srunner/srunner.go -id $serverid -port $serverport \
        -lfdPort $lfdport -s1Addr $2 -s2Addr $3 -s3Addr $4
}

# args <id> <s1Addr> <s2Addr> <s3Addr>
start_server_passive() {
    local serverid="S$1"
    local lfdport=$((9000 + $1))
    local serverport=$((8080 + $1))
    go run server-passive/srunner/srunner.go -id $serverid -port $serverport \
        -lfdPort $lfdport -s1Addr $2 -s2Addr $3 -s3Addr $4
}

# args <id> <s1> <s2> <s3>
start_client() {
    local clientid="C$1"
    local s1="$2"
    local s2="$3"
    local s3="$4"
    local think="0.5s"
    go run client/client.go -s1 $s1 -s2 $s2 -s3 $s3 -id $clientid -auto
}

# args none
start_gfd() {
    go run gfd/gfdrunner/gfdrunner.go
}

# args <gfdaddr>
start_rm() {
    local gfdaddr=$1
    go run rm/rmrunner/rmrunner.go -gfdaddr $gfdaddr
}

if [ $1 = "lfd" ]; then
    if [ $# -ne 4 ]; then
        echo "incorect number of args: $# for LFD"
        echo "lfd args: <id> <gfdaddr> <method>"
        exit -1
    fi
    echo "starting lfd $2.."
    start_lfd $2 $3 $4
    exit 0

elif [ $1 = "server" ]; then
    if [ $2 = "active" ]; then
        echo "starting active..."
        start_server_active $3 $4 $5 $6
    elif [ $2 = "passive" ]; then
        echo "starting passive..."
        start_server_passive $3 $4 $5 $6 $7
    else
        echo "incorect server type"
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
elif [ $1 = "rm" ]; then
    if [ $# -eq 1 ]; then
        echo "starting rm with default gfdaddr (localhost:9090).."
        start_rm "localhost:9090"
    elif [ $# -eq 2 ]; then
        echo "starting rm with gfdaddr $2.."
        start_rm $2
    else
        echo "incorect number of args: $# for rm"
        echo "rm args: [gfdaddr] (optional, defaults to localhost:9090)"
        exit -1
    fi
    exit 0
fi
echo "invalid type {lfd, server, client, gfd}"

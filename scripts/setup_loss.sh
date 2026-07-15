#!/bin/bash
# Adds packet loss on loopback interface.
# Usage: ./setup_loss.sh [percent]

LOSS_PERCENT=${1:-2}

echo "Adding ${LOSS_PERCENT}% packet loss on loopback..."

sudo tc qdisc del dev lo root 2>/dev/null
sudo tc qdisc add dev lo root netem loss ${LOSS_PERCENT}%

echo "Current tc rules:"
tc qdisc show dev lo
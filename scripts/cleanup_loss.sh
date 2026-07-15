#!/bin/bash
# Removes packet loss rules from loopback interface.

echo "Removing packet loss rules from loopback..."

sudo tc qdisc del dev lo root 2>/dev/null

echo "Current tc rules:"
tc qdisc show dev lo
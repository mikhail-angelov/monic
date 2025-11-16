#!/bin/bash

# Test script to verify CPU efficiency of the monitoring service

echo "Testing Monic monitoring service CPU efficiency..."
echo "=================================================="

# Build the application if not already built
if [ ! -f "./monic" ]; then
    echo "Building application..."
    go build -o monic main.go
fi

# Start the monitoring service in the background
echo "Starting monitoring service..."
./monic &
MONIC_PID=$!

# Wait for service to start
sleep 3

# Monitor CPU usage for 30 seconds
echo "Monitoring CPU usage for 30 seconds..."
top -pid $MONIC_PID -l 30 -stats cpu,command | grep monic

# Stop the service
echo "Stopping monitoring service..."
kill $MONIC_PID
wait $MONIC_PID 2>/dev/null

echo "Test completed."
echo "Expected: CPU usage should be minimal (0-2% when idle)"

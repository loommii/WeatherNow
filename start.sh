#!/bin/bash

PORT=45854
PID=$(lsof -ti :$PORT 2>/dev/null)

if [ -n "$PID" ]; then
  echo "Port $PORT is in use (PID: $PID), killing..."
  kill -9 $PID
  sleep 1
fi

cd "$(dirname "$0")/backend/weatherapi"
go run weatherapi.go

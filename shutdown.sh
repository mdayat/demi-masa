#!/bin/bash

echo "Stopping web service..."
docker compose -f ./web/compose.yaml down

echo "Stopping worker service..."
docker compose -f ./worker/compose.yaml down

echo "Stopping asynqmon service..."
docker compose -f ./asynqmon/compose.yaml down

echo "Stopping shared services..."
docker compose down
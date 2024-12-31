#!/bin/bash

echo "Creating demi_masa network..."
docker network create demi_masa

echo "Starting shared services..."
docker compose up -d

echo "Starting web service..."
docker compose -f ./web/compose.yaml up -d

echo "Starting worker service..."
docker compose -f ./worker/compose.yaml up -d

echo "Starting asynqmon service..."
docker compose -f ./asynqmon/compose.yaml up -d
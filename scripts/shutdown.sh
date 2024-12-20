#!/bin/bash

echo "Stopping worker service..."
docker-compose stop worker

sleep 3

echo "Stopping redis service..."
docker-compose down
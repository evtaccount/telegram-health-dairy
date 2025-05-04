# Makefile для управления ботом в Docker

IMAGE_NAME := telegram-health-dairy
SERVICE := health-dairy

build:
	docker-compose build --no-cache

up:
	docker-compose up -d

down:
	docker-compose down

rebuild: down build up

logs:
	docker logs -f $(SERVICE)

reset:
	rm -rf data/* logs/*

cleanup:
	docker stack rm telegram-health-dairy || true
	docker-compose down || true
	docker rm -f $(shell docker ps -aq) || true
	docker rmi telegram-health-dairy telegram-health-dairy-health-dairy || true
	docker image prune -f
	docker network rm telegram-health-dairy_default || true

# Makefile для управления ботом в Docker

IMAGE_NAME := telegram-health-diary
SERVICE := health-diary

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
	docker stack rm telegram-health-diary || true
	docker-compose down || true
	docker rm -f $(shell docker ps -aq) || true
	docker rmi telegram-health-diary telegram-health-diary-health-diary || true
	docker image prune -f
	docker network rm telegram-tax-bot_default || true

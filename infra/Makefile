.PHONY: up down logs ps migrate seed

up:
	docker compose -f infra/docker-compose.yml up -d --build

down:
	docker compose -f infra/docker-compose.yml down -v

logs:
	docker compose -f infra/docker-compose.yml logs -f --tail=200

ps:
	docker compose -f infra/docker-compose.yml ps

migrate:
	docker compose -f infra/docker-compose.yml exec backend-api novel2av migrate up

seed:
	docker compose -f infra/docker-compose.yml exec backend-api novel2av dev seed

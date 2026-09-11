IMAGE_NAME := xomrkob/mailer-service
NAMESPACE := go-app
DEPLOYMENT := mailer-service
VERSION ?= $(shell git describe --tags --always || echo "latest")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

.PHONY: all build push deploy clean test logs smtp-secret

all: build push deploy

# Применяет реальный SMTP_PASS императивно (не коммитится в git).
# Запуск: make smtp-secret SMTP_PASS='<app-password>'
smtp-secret:
	@[ -n "$(SMTP_PASS)" ] || (echo "Usage: make smtp-secret SMTP_PASS='<app-password>'" && exit 1)
	@echo "Applying secret smtp-credentials (SMTP_PASS from CLI)..."
	kubectl -n $(NAMESPACE) create secret generic smtp-credentials \
		--from-literal=SMTP_PASS="$(SMTP_PASS)" \
		--dry-run=client -o yaml | kubectl apply -f -

build:
	@echo "Building docker image $(IMAGE_NAME):$(VERSION)..."
	docker build -f Dockerfile \
		--build-arg VERSION=$(VERSION) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t $(IMAGE_NAME):$(VERSION) \
		-t $(IMAGE_NAME):latest ..

push:
	@echo "Pushing image $(IMAGE_NAME):$(VERSION)..."
	docker push $(IMAGE_NAME):$(VERSION)
	docker push $(IMAGE_NAME):latest

deploy:
	@echo "Updating K8s deployment..."
	kubectl -n $(NAMESPACE) set image deployment/$(DEPLOYMENT) \
		mailer-service=$(IMAGE_NAME):$(VERSION)
	@echo "Success!"

test:
	go test -v ./...

logs:
	kubectl -n $(NAMESPACE) logs -f -l app=mailer

clean:
	rm -f mailer-worker
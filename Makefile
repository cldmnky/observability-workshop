.PHONY: help deploy build build-site build-api build-all build-wait build-logs build-logs-api refresh deploy-status url rebuild clean uninstall-chart sync-argo

# Configuration
NAMESPACE ?= showroom-workshop
BUILD_NAME_SITE ?= showroom-site-build
BUILD_NAME_API ?= user-info-api-build
BUILD_NAME ?= $(BUILD_NAME_SITE)
DEPLOYMENT_NAME ?= showroom-site
ROUTE_NAME ?= showroom-site
CHART_PATH ?= bootstrap/helm/showroom-site
CHART_RELEASE ?= showroom-site
APPSET_FILE ?= bootstrap/argocd/applicationset-observability.yaml
ARGO_NAMESPACE ?= openshift-gitops
USERS_FILE ?= .config/users.yaml
USER_DATA_SECRET ?= workshop-users-secret

# Colors for output
GREEN := \033[0;32m
YELLOW := \033[0;33m
BLUE := \033[0;34m
RED := \033[0;31m
NC := \033[0m # No Color

help: ## Show this help message
	@echo "Showroom Site - Builds for OpenShift Management"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-20s$(NC) %s\n", $$1, $$2}'

deploy: ## Bootstrap ArgoCD apps and external users data from .config/users.yaml
	@if [ ! -f "$(USERS_FILE)" ]; then \
		printf '%b\n' "$(RED)Error: $(USERS_FILE) not found$(NC)"; \
		printf '%b\n' "$(YELLOW)Create $(USERS_FILE) with your workshop users data (see .config/users.yaml.example)$(NC)"; \
		exit 1; \
	fi
	@printf '%b\n' "$(GREEN)Ensuring namespace $(NAMESPACE) exists...$(NC)"
	@oc create namespace $(NAMESPACE) 2>/dev/null || echo "Namespace already exists"
	@printf '%b\n' "$(GREEN)Creating/updating users secret $(USER_DATA_SECRET)...$(NC)"
	@oc create secret generic $(USER_DATA_SECRET) \
		-n $(NAMESPACE) \
		--from-file=users.yaml=$(USERS_FILE) \
		--dry-run=client -o yaml | oc apply -f -
	@printf '%b\n' "$(GREEN)Applying ApplicationSet...$(NC)"
	@oc apply -f $(APPSET_FILE)
	@printf '%b\n' "$(YELLOW)ArgoCD will sync applications automatically (including showroom-site).$(NC)"
	@printf '%s\n' "Run 'make deploy-status' to verify showroom-site resources once synced."

build: ## Trigger builds for both site and API (default)
	@$(MAKE) build-all

build-site: ## Trigger showroom-site build
	@printf '%b\n' "$(GREEN)Triggering showroom-site build...$(NC)"
	@printf 'apiVersion: shipwright.io/v1beta1\nkind: BuildRun\nmetadata:\n  generateName: $(BUILD_NAME_SITE)-\n  namespace: $(NAMESPACE)\nspec:\n  build:\n    name: $(BUILD_NAME_SITE)\n' | oc create -f -
	@printf '%b\n' "$(GREEN)Site BuildRun created.$(NC)"

build-api: ## Trigger user-info-api build
	@printf '%b\n' "$(BLUE)Triggering user-info-api build...$(NC)"
	@printf 'apiVersion: shipwright.io/v1beta1\nkind: BuildRun\nmetadata:\n  generateName: $(BUILD_NAME_API)-\n  namespace: $(NAMESPACE)\nspec:\n  build:\n    name: $(BUILD_NAME_API)\n' | oc create -f -
	@printf '%b\n' "$(BLUE)API BuildRun created.$(NC)"

build-all: ## Trigger both builds
	@printf '%b\n' "$(GREEN)Triggering both builds...$(NC)"
	@$(MAKE) build-site
	@$(MAKE) build-api

refresh: ## Build site+API images and restart deployment on latest images
	@printf '%b\n' "$(GREEN)Building latest images and refreshing deployment...$(NC)"
	@$(MAKE) build-wait BUILD_NAME=$(BUILD_NAME_SITE)
	@$(MAKE) build-wait BUILD_NAME=$(BUILD_NAME_API)
	@printf '%b\n' "$(YELLOW)Restarting deployment $(DEPLOYMENT_NAME)...$(NC)"
	@oc rollout restart deployment/$(DEPLOYMENT_NAME) -n $(NAMESPACE)
	@oc rollout status deployment/$(DEPLOYMENT_NAME) -n $(NAMESPACE) --timeout=300s
	@printf '%b\n' "$(GREEN)Refresh complete: deployment is running latest built images.$(NC)"

build-wait: ## Trigger a new build and wait for completion
	@printf '%b\n' "$(GREEN)Triggering new build...$(NC)"
	@BUILDRUN=$$(oc create -f - <<< 'apiVersion: shipwright.io/v1beta1\nkind: BuildRun\nmetadata:\n  generateName: $(BUILD_NAME)-\n  namespace: $(NAMESPACE)\nspec:\n  build:\n    name: $(BUILD_NAME)' | cut -d' ' -f1 | cut -d'/' -f2); \
	printf '%b\n' "$(YELLOW)BuildRun: $$BUILDRUN$(NC)"; \
	printf '%b\n' "$(YELLOW)Waiting for build to complete...$(NC)"; \
	for i in {1..120}; do \
		STATUS=$$(oc get buildrun $$BUILDRUN -n $(NAMESPACE) -o jsonpath='{.status.conditions[?(@.type=="Succeeded")].status}' 2>/dev/null || echo "Unknown"); \
		REASON=$$(oc get buildrun $$BUILDRUN -n $(NAMESPACE) -o jsonpath='{.status.conditions[?(@.type=="Succeeded")].reason}' 2>/dev/null || echo "Unknown"); \
		echo "  Status: $$STATUS ($$REASON)"; \
		if [ "$$STATUS" == "True" ]; then \
			printf '%b\n' "$(GREEN)✓ Build completed successfully!$(NC)"; \
			exit 0; \
		elif [ "$$STATUS" == "False" ]; then \
			printf '%b\n' "$(RED)✗ Build failed!$(NC)"; \
			oc get buildrun $$BUILDRUN -n $(NAMESPACE) -o yaml; \
			exit 1; \
		fi; \
		sleep 5; \
	done; \
	printf '%b\n' "$(RED)Build timeout after 10 minutes$(NC)"; \
	exit 1

build-logs: ## Follow logs from the latest showroom-site build
	@printf '%b\n' "$(YELLOW)Finding latest showroom-site BuildRun...$(NC)"
	@BUILDRUN=$$(oc get buildrun -n $(NAMESPACE) -l build.shipwright.io/name=$(BUILD_NAME_SITE) --sort-by=.metadata.creationTimestamp -o jsonpath='{.items[-1].metadata.name}' 2>/dev/null); \
	if [ -z "$$BUILDRUN" ]; then \
		printf '%b\n' "$(RED)No BuildRuns found in namespace $(NAMESPACE)$(NC)"; \
		exit 1; \
	fi; \
	printf '%b\n' "$(GREEN)Following logs for: $$BUILDRUN$(NC)"; \
	POD=$$(oc get pods -n $(NAMESPACE) -l build.shipwright.io/name=$(BUILD_NAME_SITE) --sort-by=.metadata.creationTimestamp -o jsonpath='{.items[-1].metadata.name}' 2>/dev/null); \
	if [ -z "$$POD" ]; then \
		printf '%b\n' "$(YELLOW)Build pod not yet created, waiting...$(NC)"; \
		sleep 5; \
		POD=$$(oc get pods -n $(NAMESPACE) -l build.shipwright.io/name=$(BUILD_NAME_SITE) --sort-by=.metadata.creationTimestamp -o jsonpath='{.items[-1].metadata.name}'); \
	fi; \
	oc logs -f $$POD -n $(NAMESPACE) -c step-build-and-push 2>&1 || \
	oc logs -f $$POD -n $(NAMESPACE) 2>&1

build-logs-api: ## Follow logs from the latest user-info-api build
	@printf '%b\n' "$(YELLOW)Finding latest user-info-api BuildRun...$(NC)"
	@BUILDRUN=$$(oc get buildrun -n $(NAMESPACE) -l build.shipwright.io/name=$(BUILD_NAME_API) --sort-by=.metadata.creationTimestamp -o jsonpath='{.items[-1].metadata.name}' 2>/dev/null); \
	if [ -z "$$BUILDRUN" ]; then \
		printf '%b\n' "$(RED)No API BuildRuns found$(NC)"; \
		exit 1; \
	fi; \
	printf '%b\n' "$(BLUE)Following logs for: $$BUILDRUN$(NC)"; \
	POD=$$(oc get pods -n $(NAMESPACE) -l build.shipwright.io/name=$(BUILD_NAME_API) --sort-by=.metadata.creationTimestamp -o jsonpath='{.items[-1].metadata.name}' 2>/dev/null); \
	if [ -z "$$POD" ]; then \
		printf '%b\n' "$(YELLOW)Build pod not yet created, waiting...$(NC)"; \
		sleep 5; \
		POD=$$(oc get pods -n $(NAMESPACE) -l build.shipwright.io/name=$(BUILD_NAME_API) --sort-by=.metadata.creationTimestamp -o jsonpath='{.items[-1].metadata.name}'); \
	fi; \
	oc logs -f $$POD -n $(NAMESPACE) -c step-build-and-push 2>&1 || \
	oc logs -f $$POD -n $(NAMESPACE) 2>&1

deploy-status: ## Check deployment and pod status
	@printf '%b\n' "$(GREEN)=== Showroom Site Build ===$(NC)"
	@oc get build.shipwright.io $(BUILD_NAME_SITE) -n $(NAMESPACE) 2>/dev/null || printf '%b\n' "$(RED)Site build not found$(NC)"
	@echo ""
	@printf '%b\n' "$(BLUE)=== User Info API Build ===$(NC)"
	@oc get build.shipwright.io $(BUILD_NAME_API) -n $(NAMESPACE) 2>/dev/null || printf '%b\n' "$(RED)API build not found$(NC)"
	@echo ""
	@printf '%b\n' "$(GREEN)=== Recent BuildRuns ===$(NC)"
	@oc get buildrun -n $(NAMESPACE) --sort-by=.metadata.creationTimestamp 2>/dev/null | tail -8 || printf '%b\n' "$(RED)No BuildRuns found$(NC)"
	@echo ""
	@printf '%b\n' "$(GREEN)=== ImageStream ===$(NC)"
	@oc get imagestream $(ROUTE_NAME) -n $(NAMESPACE) 2>/dev/null || printf '%b\n' "$(RED)ImageStream not found$(NC)"
	@echo ""
	@printf '%b\n' "$(GREEN)=== Deployment ===$(NC)"
	@oc get deployment $(DEPLOYMENT_NAME) -n $(NAMESPACE) 2>/dev/null || printf '%b\n' "$(RED)Deployment not found$(NC)"
	@echo ""
	@printf '%b\n' "$(GREEN)=== Pods ===$(NC)"
	@oc get pods -n $(NAMESPACE) -l app.kubernetes.io/name=$(DEPLOYMENT_NAME) 2>/dev/null || printf '%b\n' "$(RED)No pods found$( NC)"
	@echo ""
	@printf '%b\n' "$(GREEN)=== Route ===$(NC)"
	@oc get route $(ROUTE_NAME) -n $(NAMESPACE) -o jsonpath='URL: https://{.spec.host}{"\n"}' 2>/dev/null || printf '%b\n' "$(RED)Route not found$(NC)"

url: ## Get the route URL for the site
	@URL=$$(oc get route $(ROUTE_NAME) -n $(NAMESPACE) -o jsonpath='{.spec.host}' 2>/dev/null); \
	if [ -z "$$URL" ]; then \
		printf '%b\n' "$(RED)Route not found in namespace $(NAMESPACE)$(NC)"; \
		exit 1; \
	fi; \
	printf '%b\n' "$(GREEN)Showroom Site URL:$(NC)"; \
	echo "  https://$$URL"

rebuild: ## Complete rebuild workflow (build + wait + status)
	@printf '%b\n' "$(GREEN)Starting complete rebuild workflow...$(NC)"
	@$(MAKE) build-wait
	@sleep 10
	@$(MAKE) deploy-status
	@echo ""
	@$(MAKE) url

clean: ## Delete old BuildRuns (keeps last 5)
	@printf '%b\n' "$(YELLOW)Cleaning old BuildRuns (keeping last 5)...$(NC)"
	@BUILDRUNS=$$(oc get buildrun -n $(NAMESPACE) --sort-by=.metadata.creationTimestamp -o jsonpath='{.items[*].metadata.name}' | tr ' ' '\n' | head -n -5); \
	if [ -z "$$BUILDRUNS" ]; then \
		printf '%b\n' "$(GREEN)No old BuildRuns to clean$(NC)"; \
	else \
		echo "$$BUILDRUNS" | xargs -I {} oc delete buildrun {} -n $(NAMESPACE); \
		printf '%b\n' "$(GREEN)Cleanup complete$(NC)"; \
	fi

uninstall-chart: ## Uninstall Helm chart
	@printf '%b\n' "$(YELLOW)Uninstalling Helm chart...$(NC)"
	@helm uninstall $(CHART_RELEASE) --namespace $(NAMESPACE) || echo "$(YELLOW)Chart not found$(NC)"
	@printf '%b\n' "$(YELLOW)Deleting namespace...$(NC)"
	@oc delete namespace $(NAMESPACE) --ignore-not-found
	@printf '%b\n' "$(GREEN)Cleanup complete$(NC)"

sync-argo: ## Apply ApplicationSet and sync ArgoCD application
	@$(MAKE) deploy

# Development targets
dev-build: ## Quick dev build (local docker/podman test)
	@printf '%b\n' "$(YELLOW)Building container locally for testing...$(NC)"
	@podman build -t localhost/showroom-site:test -f Containerfile .
	@printf '%b\n' "$(GREEN)Test with: podman run -p 8080:8080 localhost/showroom-site:test$(NC)"

dev-run: ## Run locally built container
	@printf '%b\n' "$(GREEN)Starting local container on http://localhost:8080$(NC)"
	@podman run --rm -p 8080:8080 localhost/showroom-site:test

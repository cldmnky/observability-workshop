.PHONY: help build build-site build-api build-all build-wait build-logs build-logs-api deploy-status url rebuild clean install-chart uninstall-chart sync-argo

# Configuration
NAMESPACE ?= showroom-workshop
BUILD_NAME_SITE ?= showroom-site-build
BUILD_NAME_API ?= user-info-api-build
BUILD_NAME ?= $(BUILD_NAME_SITE)
DEPLOYMENT_NAME ?= showroom-site
ROUTE_NAME ?= showroom-site
CHART_PATH ?= bootstrap/helm/showroom-site
CHART_RELEASE ?= showroom-site

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

build: ## Trigger builds for both site and API (default)
	@$(MAKE) build-all

build-site: ## Trigger showroom-site build
	@echo "$(GREEN)Triggering showroom-site build...$(NC)"
	@printf 'apiVersion: shipwright.io/v1beta1\nkind: BuildRun\nmetadata:\n  generateName: $(BUILD_NAME_SITE)-\n  namespace: $(NAMESPACE)\nspec:\n  build:\n    name: $(BUILD_NAME_SITE)\n' | oc create -f -
	@echo "$(GREEN)Site BuildRun created.$(NC)"

build-api: ## Trigger user-info-api build
	@echo "$(BLUE)Triggering user-info-api build...$(NC)"
	@printf 'apiVersion: shipwright.io/v1beta1\nkind: BuildRun\nmetadata:\n  generateName: $(BUILD_NAME_API)-\n  namespace: $(NAMESPACE)\nspec:\n  build:\n    name: $(BUILD_NAME_API)\n' | oc create -f -
	@echo "$(BLUE)API BuildRun created.$(NC)"

build-all: ## Trigger both builds
	@echo "$(GREEN)Triggering both builds...$(NC)"
	@$(MAKE) build-site
	@$(MAKE) build-api

build-wait: ## Trigger a new build and wait for completion
	@echo "$(GREEN)Triggering new build...$(NC)"
	@BUILDRUN=$$(oc create -f - <<< 'apiVersion: shipwright.io/v1beta1\nkind: BuildRun\nmetadata:\n  generateName: $(BUILD_NAME)-\n  namespace: $(NAMESPACE)\nspec:\n  build:\n    name: $(BUILD_NAME)' | cut -d' ' -f1 | cut -d'/' -f2); \
	echo "$(YELLOW)BuildRun: $$BUILDRUN$(NC)"; \
	echo "$(YELLOW)Waiting for build to complete...$(NC)"; \
	for i in {1..120}; do \
		STATUS=$$(oc get buildrun $$BUILDRUN -n $(NAMESPACE) -o jsonpath='{.status.conditions[?(@.type=="Succeeded")].status}' 2>/dev/null || echo "Unknown"); \
		REASON=$$(oc get buildrun $$BUILDRUN -n $(NAMESPACE) -o jsonpath='{.status.conditions[?(@.type=="Succeeded")].reason}' 2>/dev/null || echo "Unknown"); \
		echo "  Status: $$STATUS ($$REASON)"; \
		if [ "$$STATUS" == "True" ]; then \
			echo "$(GREEN)✓ Build completed successfully!$(NC)"; \
			exit 0; \
		elif [ "$$STATUS" == "False" ]; then \
			echo "$(RED)✗ Build failed!$(NC)"; \
			oc get buildrun $$BUILDRUN -n $(NAMESPACE) -o yaml; \
			exit 1; \
		fi; \
		sleep 5; \
	done; \
	echo "$(RED)Build timeout after 10 minutes$(NC)"; \
	exit 1

build-logs: ## Follow logs from the latest showroom-site build
	@echo "$(YELLOW)Finding latest showroom-site BuildRun...$(NC)"
	@BUILDRUN=$$(oc get buildrun -n $(NAMESPACE) -l build.shipwright.io/name=$(BUILD_NAME_SITE) --sort-by=.metadata.creationTimestamp -o jsonpath='{.items[-1].metadata.name}' 2>/dev/null); \
	if [ -z "$$BUILDRUN" ]; then \
		echo "$(RED)No BuildRuns found in namespace $(NAMESPACE)$(NC)"; \
		exit 1; \
	fi; \
	echo "$(GREEN)Following logs for: $$BUILDRUN$(NC)"; \
	POD=$$(oc get pods -n $(NAMESPACE) -l build.shipwright.io/name=$(BUILD_NAME_SITE) --sort-by=.metadata.creationTimestamp -o jsonpath='{.items[-1].metadata.name}' 2>/dev/null); \
	if [ -z "$$POD" ]; then \
		echo "$(YELLOW)Build pod not yet created, waiting...$(NC)"; \
		sleep 5; \
		POD=$$(oc get pods -n $(NAMESPACE) -l build.shipwright.io/name=$(BUILD_NAME_SITE) --sort-by=.metadata.creationTimestamp -o jsonpath='{.items[-1].metadata.name}'); \
	fi; \
	oc logs -f $$POD -n $(NAMESPACE) -c step-build-and-push 2>&1 || \
	oc logs -f $$POD -n $(NAMESPACE) 2>&1

build-logs-api: ## Follow logs from the latest user-info-api build
	@echo "$(YELLOW)Finding latest user-info-api BuildRun...$(NC)"
	@BUILDRUN=$$(oc get buildrun -n $(NAMESPACE) -l build.shipwright.io/name=$(BUILD_NAME_API) --sort-by=.metadata.creationTimestamp -o jsonpath='{.items[-1].metadata.name}' 2>/dev/null); \
	if [ -z "$$BUILDRUN" ]; then \
		echo "$(RED)No API BuildRuns found$(NC)"; \
		exit 1; \
	fi; \
	echo "$(BLUE)Following logs for: $$BUILDRUN$(NC)"; \
	POD=$$(oc get pods -n $(NAMESPACE) -l build.shipwright.io/name=$(BUILD_NAME_API) --sort-by=.metadata.creationTimestamp -o jsonpath='{.items[-1].metadata.name}' 2>/dev/null); \
	if [ -z "$$POD" ]; then \
		echo "$(YELLOW)Build pod not yet created, waiting...$(NC)"; \
		sleep 5; \
		POD=$$(oc get pods -n $(NAMESPACE) -l build.shipwright.io/name=$(BUILD_NAME_API) --sort-by=.metadata.creationTimestamp -o jsonpath='{.items[-1].metadata.name}'); \
	fi; \
	oc logs -f $$POD -n $(NAMESPACE) -c step-build-and-push 2>&1 || \
	oc logs -f $$POD -n $(NAMESPACE) 2>&1

deploy-status: ## Check deployment and pod status
	@echo "$(GREEN)=== Showroom Site Build ===$(NC)"
	@oc get build.shipwright.io $(BUILD_NAME_SITE) -n $(NAMESPACE) 2>/dev/null || echo "$(RED)Site build not found$(NC)"
	@echo ""
	@echo "$(BLUE)=== User Info API Build ===$(NC)"
	@oc get build.shipwright.io $(BUILD_NAME_API) -n $(NAMESPACE) 2>/dev/null || echo "$(RED)API build not found$(NC)"
	@echo ""
	@echo "$(GREEN)=== Recent BuildRuns ===$(NC)"
	@oc get buildrun -n $(NAMESPACE) --sort-by=.metadata.creationTimestamp 2>/dev/null | tail -8 || echo "$(RED)No BuildRuns found$(NC)"
	@echo ""
	@echo "$(GREEN)=== ImageStream ===$(NC)"
	@oc get imagestream $(ROUTE_NAME) -n $(NAMESPACE) 2>/dev/null || echo "$(RED)ImageStream not found$(NC)"
	@echo ""
	@echo "$(GREEN)=== Deployment ===$(NC)"
	@oc get deployment $(DEPLOYMENT_NAME) -n $(NAMESPACE) 2>/dev/null || echo "$(RED)Deployment not found$(NC)"
	@echo ""
	@echo "$(GREEN)=== Pods ===$(NC)"
	@oc get pods -n $(NAMESPACE) -l app.kubernetes.io/name=$(DEPLOYMENT_NAME) 2>/dev/null || echo "$(RED)No pods found$( NC)"
	@echo ""
	@echo "$(GREEN)=== Route ===$(NC)"
	@oc get route $(ROUTE_NAME) -n $(NAMESPACE) -o jsonpath='URL: https://{.spec.host}{"\n"}' 2>/dev/null || echo "$(RED)Route not found$(NC)"

url: ## Get the route URL for the site
	@URL=$$(oc get route $(ROUTE_NAME) -n $(NAMESPACE) -o jsonpath='{.spec.host}' 2>/dev/null); \
	if [ -z "$$URL" ]; then \
		echo "$(RED)Route not found in namespace $(NAMESPACE)$(NC)"; \
		exit 1; \
	fi; \
	echo "$(GREEN)Showroom Site URL:$(NC)"; \
	echo "  https://$$URL"

rebuild: ## Complete rebuild workflow (build + wait + status)
	@echo "$(GREEN)Starting complete rebuild workflow...$(NC)"
	@$(MAKE) build-wait
	@sleep 10
	@$(MAKE) deploy-status
	@echo ""
	@$(MAKE) url

clean: ## Delete old BuildRuns (keeps last 5)
	@echo "$(YELLOW)Cleaning old BuildRuns (keeping last 5)...$(NC)"
	@BUILDRUNS=$$(oc get buildrun -n $(NAMESPACE) --sort-by=.metadata.creationTimestamp -o jsonpath='{.items[*].metadata.name}' | tr ' ' '\n' | head -n -5); \
	if [ -z "$$BUILDRUNS" ]; then \
		echo "$(GREEN)No old BuildRuns to clean$(NC)"; \
	else \
		echo "$$BUILDRUNS" | xargs -I {} oc delete buildrun {} -n $(NAMESPACE); \
		echo "$(GREEN)Cleanup complete$(NC)"; \
	fi

install-chart: ## Install Helm chart directly (not via ArgoCD)
	@echo "$(GREEN)Installing Helm chart...$(NC)"
	@helm upgrade --install $(CHART_RELEASE) $(CHART_PATH) \
		--namespace $(NAMESPACE) \
		--create-namespace \
		--values $(CHART_PATH)/values.yaml
	@echo "$(GREEN)Chart installed. Initial build may take a few minutes.$(NC)"
	@echo "Run 'make deploy-status' to check progress."

uninstall-chart: ## Uninstall Helm chart
	@echo "$(YELLOW)Uninstalling Helm chart...$(NC)"
	@helm uninstall $(CHART_RELEASE) --namespace $(NAMESPACE) || echo "$(YELLOW)Chart not found$(NC)"
	@echo "$(YELLOW)Deleting namespace...$(NC)"
	@oc delete namespace $(NAMESPACE) --ignore-not-found
	@echo "$(GREEN)Cleanup complete$(NC)"

sync-argo: ## Apply ApplicationSet and sync ArgoCD application
	@echo "$(GREEN)Applying ApplicationSet...$(NC)"
	@oc apply -f bootstrap/argocd/applicationset-observability.yaml
	@echo "$(YELLOW)Waiting for application to be created...$(NC)"
	@sleep 5
	@if command -v argocd >/dev/null 2>&1; then \
		echo "$(GREEN)Syncing ArgoCD application...$(NC)"; \
		argocd app sync $(CHART_RELEASE); \
	else \
		echo "$(YELLOW)argocd CLI not found. ArgoCD will auto-sync within its sync interval.$(NC)"; \
		echo "Or manually sync via UI: https://openshift-gitops-server-openshift-gitops.apps.cluster/"; \
	fi
	@echo "$(GREEN)ArgoCD configured$(NC)"

# Development targets
dev-build: ## Quick dev build (local docker/podman test)
	@echo "$(YELLOW)Building container locally for testing...$(NC)"
	@podman build -t localhost/showroom-site:test -f Containerfile .
	@echo "$(GREEN)Test with: podman run -p 8080:8080 localhost/showroom-site:test$(NC)"

dev-run: ## Run locally built container
	@echo "$(GREEN)Starting local container on http://localhost:8080$(NC)"
	@podman run --rm -p 8080:8080 localhost/showroom-site:test

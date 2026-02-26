.PHONY: help deploy verify-rbac build build-site build-api build-all build-wait build-logs build-logs-api refresh deploy-status url rebuild clean uninstall-chart sync-argo dev-build dev-build-api dev-run dev-stop

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
WORKSHOP_USERS_GROUP ?= workshop-users
OPERATOR_VIEW_NAMESPACES ?= openshift-logging openshift-tempo-operator openshift-operators openshift-monitoring openshift-user-workload-monitoring openshift-netobserv-operator openshift-builds
EXERCISE_NAMESPACE_SUFFIXES ?= observability-demo tracing-demo
MONITORING_CLUSTERROLE ?= workshop-user-monitoring
MONITORING_API_CLUSTERROLE ?= workshop-monitoring-api
MONITORING_API_NAMESPACES ?= openshift-monitoring openshift-user-workload-monitoring observability-demo
APPLICATION_LOGS_CLUSTERROLE ?= cluster-logging-application-view
APPLICATION_LOGS_ROLEBINDING ?= view-application-logs

# Local dev configuration
DEV_POD_NAME ?= showroom-dev-pod
DEV_SITE_IMAGE ?= localhost/showroom-site:test
DEV_API_IMAGE ?= localhost/user-info-api:test
DEV_USERS_FILE ?= .config/users-dev.yaml

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
	@USER_LIST=$$(awk '/^users:/{in_users=1; next} in_users && /^[^[:space:]]/{in_users=0} in_users && /^  [A-Za-z0-9._-]+:/{gsub(":", "", $$1); print $$1}' $(USERS_FILE)); \
	if [ -z "$$USER_LIST" ]; then \
		printf '%b\n' "$(RED)Error: no users found under 'users:' in $(USERS_FILE)$(NC)"; \
		exit 1; \
	fi; \
	printf '%b\n' "$(GREEN)Workshop users discovered: $$USER_LIST$(NC)"
	@printf '%b\n' "$(GREEN)Ensuring namespace $(NAMESPACE) exists...$(NC)"
	@oc create namespace $(NAMESPACE) 2>/dev/null || echo "Namespace already exists"
	@printf '%b\n' "$(GREEN)Creating/updating users secret $(USER_DATA_SECRET)...$(NC)"
	@oc create secret generic $(USER_DATA_SECRET) \
		-n $(NAMESPACE) \
		--from-file=users.yaml=$(USERS_FILE) \
		--dry-run=client -o yaml | oc apply -f -
	@printf '%b\n' "$(GREEN)Creating/updating OpenShift group $(WORKSHOP_USERS_GROUP)...$(NC)"
	@USER_LIST=$$(awk '/^users:/{in_users=1; next} in_users && /^[^[:space:]]/{in_users=0} in_users && /^  [A-Za-z0-9._-]+:/{gsub(":", "", $$1); print $$1}' $(USERS_FILE)); \
	{ \
		echo "apiVersion: user.openshift.io/v1"; \
		echo "kind: Group"; \
		echo "metadata:"; \
		echo "  name: $(WORKSHOP_USERS_GROUP)"; \
		echo "users:"; \
		for user in $$USER_LIST; do echo "- $$user"; done; \
	} | oc apply -f -
	@printf '%b\n' "$(GREEN)Granting group view access in operator namespaces...$(NC)"
	@for ns in $(OPERATOR_VIEW_NAMESPACES); do \
		if oc get namespace $$ns >/dev/null 2>&1; then \
			printf '%s\n' \
				'apiVersion: rbac.authorization.k8s.io/v1' \
				'kind: RoleBinding' \
				'metadata:' \
				'  name: $(WORKSHOP_USERS_GROUP)-view' \
				"  namespace: $$ns" \
				'roleRef:' \
				'  apiGroup: rbac.authorization.k8s.io' \
				'  kind: ClusterRole' \
				'  name: view' \
				'subjects:' \
				'- kind: Group' \
				'  apiGroup: rbac.authorization.k8s.io' \
				'  name: $(WORKSHOP_USERS_GROUP)' | oc apply -f -; \
		else \
			printf '%b\n' "$(YELLOW)Skipping missing namespace $$ns for view RoleBinding$(NC)"; \
		fi; \
	done
	@printf '%b\n' "$(GREEN)Creating/updating monitoring ClusterRole for workshop users...$(NC)"
	@printf '%s\n' \
		'apiVersion: rbac.authorization.k8s.io/v1' \
		'kind: ClusterRole' \
		'metadata:' \
		'  name: $(MONITORING_CLUSTERROLE)' \
		'rules:' \
		'- apiGroups: [monitoring.coreos.com]' \
		'  resources: [servicemonitors, prometheusrules, podmonitors]' \
		'  verbs: [create, delete, get, list, patch, update, watch]' | oc apply -f -
	@printf '%b\n' "$(GREEN)Creating/updating monitoring API ClusterRole for workshop users...$(NC)"
	@printf '%s\n' \
		'apiVersion: rbac.authorization.k8s.io/v1' \
		'kind: ClusterRole' \
		'metadata:' \
		'  name: $(MONITORING_API_CLUSTERROLE)' \
		'rules:' \
		'- apiGroups: [monitoring.coreos.com]' \
		'  resources: [prometheuses, alertmanagers]' \
		'  verbs: [get, list, watch]' \
		'- apiGroups: [monitoring.coreos.com]' \
		'  resources: [prometheuses/api, alertmanagers/api]' \
		'  verbs: [get]' | oc apply -f -
	@printf '%b\n' "$(GREEN)Granting group monitoring API access in monitoring namespaces...$(NC)"
	@for ns in $(MONITORING_API_NAMESPACES); do \
		if oc get namespace $$ns >/dev/null 2>&1; then \
			printf '%s\n' \
				'apiVersion: rbac.authorization.k8s.io/v1' \
				'kind: RoleBinding' \
				'metadata:' \
				'  name: $(MONITORING_API_CLUSTERROLE)' \
				"  namespace: $$ns" \
				'roleRef:' \
				'  apiGroup: rbac.authorization.k8s.io' \
				'  kind: ClusterRole' \
				'  name: $(MONITORING_API_CLUSTERROLE)' \
				'subjects:' \
				'- kind: Group' \
				'  apiGroup: rbac.authorization.k8s.io' \
				'  name: $(WORKSHOP_USERS_GROUP)' | oc apply -f -; \
		else \
			printf '%b\n' "$(YELLOW)Skipping missing namespace $$ns for monitoring API RoleBinding$(NC)"; \
		fi; \
	done
	@printf '%b\n' "$(GREEN)Creating/updating per-user exercise namespaces with monitoring RBAC...$(NC)"
	@USER_LIST=$$(awk '/^users:/{in_users=1; next} in_users && /^[^[:space:]]/{in_users=0} in_users && /^  [A-Za-z0-9._-]+:/{gsub(":", "", $$1); print $$1}' $(USERS_FILE)); \
	for user in $$USER_LIST; do \
		for suffix in $(EXERCISE_NAMESPACE_SUFFIXES); do \
			user_ns=$${user}-$${suffix}; \
			oc create namespace $$user_ns 2>/dev/null || true; \
			printf '%s\n' \
				'apiVersion: rbac.authorization.k8s.io/v1' \
				'kind: RoleBinding' \
				'metadata:' \
				'  name: $(MONITORING_CLUSTERROLE)' \
				"  namespace: $$user_ns" \
				'roleRef:' \
				'  apiGroup: rbac.authorization.k8s.io' \
				'  kind: ClusterRole' \
				'  name: $(MONITORING_CLUSTERROLE)' \
				'subjects:' \
				'- kind: User' \
				'  apiGroup: rbac.authorization.k8s.io' \
				"  name: $$user" | oc apply -f -; \
			printf '%s\n' \
				'apiVersion: rbac.authorization.k8s.io/v1' \
				'kind: RoleBinding' \
				'metadata:' \
				'  name: admin' \
				"  namespace: $$user_ns" \
				'roleRef:' \
				'  apiGroup: rbac.authorization.k8s.io' \
				'  kind: ClusterRole' \
				'  name: admin' \
				'subjects:' \
				'- kind: User' \
				'  apiGroup: rbac.authorization.k8s.io' \
				"  name: $$user" | oc apply -f -; \
			printf '%s\n' \
				'apiVersion: rbac.authorization.k8s.io/v1' \
				'kind: RoleBinding' \
				'metadata:' \
				'  name: $(APPLICATION_LOGS_ROLEBINDING)' \
				"  namespace: $$user_ns" \
				'roleRef:' \
				'  apiGroup: rbac.authorization.k8s.io' \
				'  kind: ClusterRole' \
				'  name: $(APPLICATION_LOGS_CLUSTERROLE)' \
				'subjects:' \
				'- kind: User' \
				'  apiGroup: rbac.authorization.k8s.io' \
				"  name: $$user" | oc apply -f -; \
		done; \
	done
	@printf '%b\n' "$(GREEN)Applying ApplicationSet...$(NC)"
	@oc apply -f $(APPSET_FILE)
	@printf '%b\n' "$(YELLOW)ArgoCD will sync applications automatically (including showroom-site).$(NC)"
	@printf '%s\n' "Run 'make deploy-status' to verify showroom-site resources once synced."
	@printf '%b\n' "$(BLUE)Note: User namespaces are pre-created with monitoring permissions. Users can verify access with 'oc project <namespace>'.$(NC)"

verify-rbac: ## Verify workshop group, operator view bindings, and user namespace monitoring permissions
	@if [ ! -f "$(USERS_FILE)" ]; then \
		printf '%b\n' "$(RED)Error: $(USERS_FILE) not found$(NC)"; \
		exit 1; \
	fi
	@USER_LIST=$$(awk '/^users:/{in_users=1; next} in_users && /^[^[:space:]]/{in_users=0} in_users && /^  [A-Za-z0-9._-]+:/{gsub(":", "", $$1); print $$1}' $(USERS_FILE)); \
	if [ -z "$$USER_LIST" ]; then \
		printf '%b\n' "$(RED)Error: no users found under 'users:' in $(USERS_FILE)$(NC)"; \
		exit 1; \
	fi; \
	FAIL=0; \
	printf '%b\n' "$(GREEN)Verifying OpenShift group $(WORKSHOP_USERS_GROUP)...$(NC)"; \
	if oc get group $(WORKSHOP_USERS_GROUP) >/dev/null 2>&1; then \
		printf '%b\n' "$(GREEN)✓ Group exists: $(WORKSHOP_USERS_GROUP)$(NC)"; \
		GROUP_USERS=$$(oc get group $(WORKSHOP_USERS_GROUP) -o jsonpath='{.users[*]}' 2>/dev/null); \
		for user in $$USER_LIST; do \
			if echo " $$GROUP_USERS " | grep -q " $$user "; then \
				printf '%b\n' "$(GREEN)  ✓ Group contains user: $$user$(NC)"; \
			else \
				printf '%b\n' "$(RED)  ✗ Group missing user: $$user$(NC)"; \
				FAIL=1; \
			fi; \
		done; \
	else \
		printf '%b\n' "$(RED)✗ Group not found: $(WORKSHOP_USERS_GROUP)$(NC)"; \
		FAIL=1; \
	fi; \
	printf '%b\n' "$(GREEN)Verifying group view RoleBindings in operator namespaces...$(NC)"; \
	for ns in $(OPERATOR_VIEW_NAMESPACES); do \
		if oc get namespace $$ns >/dev/null 2>&1; then \
			if oc get rolebinding $(WORKSHOP_USERS_GROUP)-view -n $$ns >/dev/null 2>&1; then \
				RB_ROLE=$$(oc get rolebinding $(WORKSHOP_USERS_GROUP)-view -n $$ns -o jsonpath='{.roleRef.name}' 2>/dev/null); \
				RB_GROUPS=$$(oc get rolebinding $(WORKSHOP_USERS_GROUP)-view -n $$ns -o jsonpath='{.subjects[?(@.kind=="Group")].name}' 2>/dev/null); \
				if [ "$$RB_ROLE" = "view" ] && echo " $$RB_GROUPS " | grep -q " $(WORKSHOP_USERS_GROUP) "; then \
					printf '%b\n' "$(GREEN)  ✓ $$ns: $(WORKSHOP_USERS_GROUP)-view$(NC)"; \
				else \
					printf '%b\n' "$(RED)  ✗ $$ns: $(WORKSHOP_USERS_GROUP)-view has unexpected roleRef/subject$(NC)"; \
					FAIL=1; \
				fi; \
			else \
				printf '%b\n' "$(RED)  ✗ $$ns: missing RoleBinding $(WORKSHOP_USERS_GROUP)-view$(NC)"; \
				FAIL=1; \
			fi; \
		else \
			printf '%b\n' "$(YELLOW)  - Skipping missing namespace $$ns$(NC)"; \
		fi; \
	done; \
	printf '%b\n' "$(GREEN)Verifying monitoring ClusterRole...$(NC)"; \
	if oc get clusterrole $(MONITORING_CLUSTERROLE) >/dev/null 2>&1; then \
		printf '%b\n' "$(GREEN)  ✓ ClusterRole exists: $(MONITORING_CLUSTERROLE)$(NC)"; \
	else \
		printf '%b\n' "$(RED)  ✗ ClusterRole missing: $(MONITORING_CLUSTERROLE)$(NC)"; \
		FAIL=1; \
	fi; \
	printf '%b\n' "$(GREEN)Verifying monitoring API ClusterRole...$(NC)"; \
	if oc get clusterrole $(MONITORING_API_CLUSTERROLE) >/dev/null 2>&1; then \
		printf '%b\n' "$(GREEN)  ✓ ClusterRole exists: $(MONITORING_API_CLUSTERROLE)$(NC)"; \
	else \
		printf '%b\n' "$(RED)  ✗ ClusterRole missing: $(MONITORING_API_CLUSTERROLE)$(NC)"; \
		FAIL=1; \
	fi; \
	printf '%b\n' "$(GREEN)Verifying monitoring API RoleBindings in monitoring namespaces...$(NC)"; \
	for ns in $(MONITORING_API_NAMESPACES); do \
		if oc get namespace $$ns >/dev/null 2>&1; then \
			if oc get rolebinding $(MONITORING_API_CLUSTERROLE) -n $$ns >/dev/null 2>&1; then \
				RB_ROLE=$$(oc get rolebinding $(MONITORING_API_CLUSTERROLE) -n $$ns -o jsonpath='{.roleRef.name}' 2>/dev/null); \
				RB_GROUPS=$$(oc get rolebinding $(MONITORING_API_CLUSTERROLE) -n $$ns -o jsonpath='{.subjects[?(@.kind=="Group")].name}' 2>/dev/null); \
				if [ "$$RB_ROLE" = "$(MONITORING_API_CLUSTERROLE)" ] && echo " $$RB_GROUPS " | grep -q " $(WORKSHOP_USERS_GROUP) "; then \
					printf '%b\n' "$(GREEN)  ✓ $$ns: $(MONITORING_API_CLUSTERROLE)$(NC)"; \
				else \
					printf '%b\n' "$(RED)  ✗ $$ns: $(MONITORING_API_CLUSTERROLE) has unexpected roleRef/subject$(NC)"; \
					FAIL=1; \
				fi; \
			else \
				printf '%b\n' "$(RED)  ✗ $$ns: missing RoleBinding $(MONITORING_API_CLUSTERROLE)$(NC)"; \
				FAIL=1; \
			fi; \
		else \
			printf '%b\n' "$(YELLOW)  - Skipping missing namespace $$ns$(NC)"; \
		fi; \
	done; \
	printf '%b\n' "$(GREEN)Verifying per-user exercise namespaces and monitoring RoleBindings...$(NC)"; \
	for user in $$USER_LIST; do \
		for suffix in $(EXERCISE_NAMESPACE_SUFFIXES); do \
			user_ns=$${user}-$${suffix}; \
			if oc get namespace $$user_ns >/dev/null 2>&1; then \
				printf '%b\n' "$(GREEN)  ✓ Namespace exists: $$user_ns$(NC)"; \
			else \
				printf '%b\n' "$(RED)  ✗ Namespace missing: $$user_ns$(NC)"; \
				FAIL=1; \
				continue; \
			fi; \
			if oc get rolebinding $(MONITORING_CLUSTERROLE) -n $$user_ns >/dev/null 2>&1; then \
				RB_ROLE=$$(oc get rolebinding $(MONITORING_CLUSTERROLE) -n $$user_ns -o jsonpath='{.roleRef.name}' 2>/dev/null); \
				RB_USERS=$$(oc get rolebinding $(MONITORING_CLUSTERROLE) -n $$user_ns -o jsonpath='{.subjects[?(@.kind=="User")].name}' 2>/dev/null); \
				if [ "$$RB_ROLE" = "$(MONITORING_CLUSTERROLE)" ] && echo " $$RB_USERS " | grep -q " $$user "; then \
					printf '%b\n' "$(GREEN)  ✓ $$user_ns: monitoring RoleBinding for $$user$(NC)"; \
				else \
					printf '%b\n' "$(RED)  ✗ $$user_ns: monitoring RoleBinding has unexpected roleRef/subject$(NC)"; \
					FAIL=1; \
				fi; \
			else \
				printf '%b\n' "$(RED)  ✗ $$user_ns: missing RoleBinding $(MONITORING_CLUSTERROLE)$(NC)"; \
				FAIL=1; \
			fi; \
			if oc get rolebinding admin -n $$user_ns >/dev/null 2>&1; then \
				RB_ROLE=$$(oc get rolebinding admin -n $$user_ns -o jsonpath='{.roleRef.name}' 2>/dev/null); \
				RB_USERS=$$(oc get rolebinding admin -n $$user_ns -o jsonpath='{.subjects[?(@.kind=="User")].name}' 2>/dev/null); \
				if [ "$$RB_ROLE" = "admin" ] && echo " $$RB_USERS " | grep -q " $$user "; then \
					printf '%b\n' "$(GREEN)  ✓ $$user_ns: admin RoleBinding for $$user$(NC)"; \
				else \
					printf '%b\n' "$(RED)  ✗ $$user_ns: admin RoleBinding has unexpected roleRef/subject$(NC)"; \
					FAIL=1; \
				fi; \
			else \
				printf '%b\n' "$(RED)  ✗ $$user_ns: missing RoleBinding admin$(NC)"; \
				FAIL=1; \
			fi; \
			if oc get rolebinding $(APPLICATION_LOGS_ROLEBINDING) -n $$user_ns >/dev/null 2>&1; then \
				RB_ROLE=$$(oc get rolebinding $(APPLICATION_LOGS_ROLEBINDING) -n $$user_ns -o jsonpath='{.roleRef.name}' 2>/dev/null); \
				RB_USERS=$$(oc get rolebinding $(APPLICATION_LOGS_ROLEBINDING) -n $$user_ns -o jsonpath='{.subjects[?(@.kind=="User")].name}' 2>/dev/null); \
				if [ "$$RB_ROLE" = "$(APPLICATION_LOGS_CLUSTERROLE)" ] && echo " $$RB_USERS " | grep -q " $$user "; then \
					printf '%b\n' "$(GREEN)  ✓ $$user_ns: application logs RoleBinding for $$user$(NC)"; \
				else \
					printf '%b\n' "$(RED)  ✗ $$user_ns: application logs RoleBinding has unexpected roleRef/subject$(NC)"; \
					FAIL=1; \
				fi; \
			else \
				printf '%b\n' "$(RED)  ✗ $$user_ns: missing RoleBinding $(APPLICATION_LOGS_ROLEBINDING)$(NC)"; \
				FAIL=1; \
			fi; \
		done; \
	done; \
	if [ $$FAIL -eq 0 ]; then \
		printf '%b\n' "$(GREEN)RBAC verification passed.$(NC)"; \
	else \
		printf '%b\n' "$(RED)RBAC verification failed.$(NC)"; \
		exit 1; \
	fi

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
dev-build-api: ## Build user-info-api container for local dev
	@printf '%b\n' "$(YELLOW)Building user-info-api container...$(NC)"
	@podman build -t $(DEV_API_IMAGE) -f user-info-api/Containerfile user-info-api/
	@printf '%b\n' "$(GREEN)API image built: $(DEV_API_IMAGE)$(NC)"

dev-build: dev-build-api ## Build both site and API containers for local dev
	@printf '%b\n' "$(YELLOW)Building showroom-site container...$(NC)"
	@podman build --build-arg PLAYBOOK=site-container-dev.yml -t $(DEV_SITE_IMAGE) -f Containerfile .
	@printf '%b\n' "$(GREEN)All dev images built successfully$(NC)"
	@printf '%b\n' "$(GREEN)Run with: make dev-run$(NC)"

dev-run: ## Run site and API in a pod (multi-container dev environment)
	@printf '%b\n' "$(YELLOW)Checking for existing pod...$(NC)"
	@podman pod exists $(DEV_POD_NAME) 2>/dev/null && $(MAKE) dev-stop || true
	@printf '%b\n' "$(GREEN)Creating pod $(DEV_POD_NAME) with port 8080 exposed...$(NC)"
	@podman pod create --name $(DEV_POD_NAME) -p 8080:8080
	@printf '%b\n' "$(GREEN)Starting user-info-api container...$(NC)"
	@podman run -d \
		--pod $(DEV_POD_NAME) \
		--name $(DEV_POD_NAME)-api \
		-e USER_DATA_FILE=/etc/user-data/users.yaml \
		-e PORT=8081 \
		-v $(PWD)/$(DEV_USERS_FILE):/etc/user-data/users.yaml:ro,z \
		$(DEV_API_IMAGE)
	@printf '%b\n' "$(GREEN)Starting showroom-site container...$(NC)"
	@podman run -d \
		--pod $(DEV_POD_NAME) \
		--name $(DEV_POD_NAME)-site \
		$(DEV_SITE_IMAGE)
	@printf '%b\n' "$(GREEN)Development environment running at http://localhost:8080$(NC)"
	@printf '%b\n' "$(YELLOW)API endpoint: http://localhost:8080/api/user-info$(NC)"
	@printf '%b\n' "$(YELLOW)Stop with: make dev-stop$(NC)"
	@printf '%b\n' "$(YELLOW)View logs: podman logs -f $(DEV_POD_NAME)-site  OR  podman logs -f $(DEV_POD_NAME)-api$(NC)"

dev-stop: ## Stop and remove local dev pod
	@printf '%b\n' "$(YELLOW)Stopping development pod...$(NC)"
	@podman pod stop $(DEV_POD_NAME) 2>/dev/null || true
	@podman pod rm $(DEV_POD_NAME) 2>/dev/null || true
	@printf '%b\n' "$(GREEN)Development environment stopped$(NC)"

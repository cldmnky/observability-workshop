# Multi-User Workshop - Quick Start Guide

## ✅ What Was Implemented

You now have a **complete multi-user workshop solution** with:

1. **OpenShift OAuth integration** - Users login with their OpenShift credentials
2. **Dynamic content injection** - Each user sees their own credentials in the static site
3. **Secure architecture** - OAuth proxy + user-info API + static nginx

## 🏗️ Architecture

```
User → Route (OAuth) → OAuth Proxy → nginx (Antora site)
                           ↓
                    User Info API
                    (reads header)
                           ↓
              JavaScript fetches data
                           ↓
         Replaces {placeholders} in page
```

## 🚀 Deployment Steps

### 1. Configure Users in values.yaml

Edit `bootstrap/helm/showroom-site/values.yaml`:

```yaml
multiUser:
  enabled: true  # Turn on multi-user support

users:
  user1:
    console_url: "https://console-openshift-console.apps.cluster-pdhs8.dynamic.redhatworkshops.io"
    password: "XXX"
    login_command: "oc login --insecure-skip-tls-verify=false -u user1 -p jiSphC0SjNVIm2iF https://api.cluster-pdhs8.dynamic.redhatworkshops.io:6443"
    openshift_cluster_ingress_domain: "apps.cluster-pdhs8.dynamic.redhatworkshops.io"
  user2:
    console_url: "https://console-openshift-console.apps.cluster-pdhs8.dynamic.redhatworkshops.io"
    password: "XXX"
    login_command: "oc login --insecure-skip-tls-verify=false -u user2 -p QjcbhgnyiYj4sauy https://api.cluster-pdhs8.dynamic.redhatworkshops.io:6443"
    openshift_cluster_ingress_domain: "apps.cluster-pdhs8.dynamic.redhatworkshops.io"
```

### 2. Build Both Containers

```bash
# Trigger builds for both showroom-site and user-info-api
make rebuild

# This will:
# 1. Build the showroom-site container (Antora + nginx)
# 2. Build the user-info-api container (Python Flask)
# 3. Wait for both builds to complete
# 4. Show deployment status

# To build individually:
make build-site      # Just the showroom site
make build-api       # Just the user-info-api

# To follow logs:
make build-logs      # Showroom site logs
make build-logs-api  # User-info-api logs
```

### 3. Redeploy with Multi-User

```bash
# Deploy updated chart with OAuth proxy and user-info-api
helm upgrade showroom-site bootstrap/helm/showroom-site \
  --namespace showroom-workshop \
  --values bootstrap/helm/showroom-site/values.yaml

# Or use the Makefile:
make install-chart
```

## 🧪 Testing

### Check Deployment Status

```bash
# Should see 3 containers per pod
oc get pods -n showroom-workshop
# NAME                             READY   STATUS
# showroom-site-xxxxx-yyyyy        3/3     Running

# Check containers
oc get pod -n showroom-workshop -l app.kubernetes.io/name=showroom-site \
  -o jsonpath='{.items[0].spec.containers[*].name}'
# oauth-proxy user-info-api showroom-site
```

### Test User Info API

```bash
# Port-forward to test API
oc port-forward -n showroom-workshop svc/showroom-site 8081:8081 &

# Test user1
curl -H "X-Forwarded-User: user1" http://localhost:8081/api/user-info

# Expected output:
# {
#   "user": "user1",
#   "console_url": "https://console-openshift-console.apps...",
#   "password": "jiSphC0SjNVIm2iF",
#   "login_command": "oc login...",
#   "openshift_cluster_ingress_domain": "apps..."
# }
```

### Access Workshop Site

```bash
# Get URL
make url

# Visit in browser - you'll be redirected to OpenShift OAuth
# Login as user1 or user2
# You should see YOUR credentials on the index page
```

## 📝 Using Placeholders in Content

In any `.adoc` file, use these placeholders:

```asciidoc
Welcome, {user}!

Your access credentials:
* Console: {openshift_console_url}
* Username: {user}
* Password: {password}

Login via CLI:
[source,bash]
----
{login_command}
----
```

**Supported placeholders:**
- `{user}` - Current username (user1, user2, etc.)
- `{password}` - User's password
- `{openshift_console_url}` or `{console_url}` - Console URL
- `{login_command}` - Full oc login command
- `{openshift_cluster_ingress_domain}` - Cluster domain

## 🔧 Troubleshooting

### "Placeholders still showing"

**Check JavaScript console:**
```bash
# From browser DevTools
[User Context] Initializing...
[User Context] User data loaded for: user1
[User Context] Placeholders replaced successfully
```

**If errors:**
```bash
# Check user-info-api logs
oc logs -n showroom-workshop -l app.kubernetes.io/name=showroom-site -c user-info-api

# Check if API is accessible
oc exec -n showroom-workshop deployment/showroom-site -c showroom-site -- \
  curl http://localhost:8081/api/user-info
```

### "OAuth login not working"

**Check ServiceAccount:**
```bash
oc describe sa showroom-site-oauth -n showroom-workshop

# Should have annotation:
# serviceaccounts.openshift.io/oauth-redirectreference.showroom
```

**Check Route:**
```bash
oc get route showroom-site -n showroom-workshop -o yaml

# Should have:
# tls:
#   termination: reencrypt
```

### "User data not found"

**Check ConfigMap:**
```bash
oc get configmap workshop-users -n showroom-workshop -o yaml

# Should contain your users with passwords
```

## 🔐 Security Notes

- **Passwords in ConfigMap** are not encrypted at rest
- For production:
  - Use temporary passwords
  - Set `HIDE_PASSWORDS: "true"` in user-info-api deployment
  - Use External Secrets Operator for credential management

## 📚 Documentation

Full documentation: [docs/MULTI-USER-SETUP.md](docs/MULTI-USER-SETUP.md)

## 🎯 What Happens When Users Access the Site

1. User navigates to showroom URL
2. Route redirects to OpenShift OAuth login
3. User enters OpenShift credentials (user1/password1)
4. OAuth proxy validates and forwards request with `X-Forwarded-User: user1` header
5. nginx serves static Antora HTML
6. Browser loads `user-context.js`
7. JavaScript calls `/api/user-info`
8. User Info API reads header, returns user1's data
9. JavaScript replaces all `{placeholders}` with real values
10. User sees their personalized workshop content!

---

**Ready to test?** Follow the deployment steps above! 🚀

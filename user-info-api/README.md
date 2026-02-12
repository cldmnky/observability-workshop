# User Info API Server

Simple Flask API server that provides user-specific workshop data for multi-user environments.

## Functionality

- Reads authenticated username from OAuth proxy headers (`X-Forwarded-User`)
- Returns user-specific credentials and URLs from ConfigMap
- Supports hot-reload of user data

## Endpoints

- `GET /api/user-info` - Returns current user's information
- `GET /api/users` - Lists all available users
- `GET /healthz` - Health check

## Environment Variables

- `USER_DATA_FILE` - Path to users YAML file (default: `/etc/user-data/users.yaml`)
- `PORT` - Server port (default: `8081`)
- `HIDE_PASSWORDS` - Hide passwords in responses (default: `false`)
- `DEFAULT_CONSOLE_URL` - Fallback console URL
- `DEFAULT_API_URL` - Fallback API URL
- `DEFAULT_INGRESS_DOMAIN` - Fallback ingress domain

## User Data Format

```yaml
users:
  user1:
    console_url: https://console-openshift-console.apps.cluster.example.com
    password: secretpassword
    login_command: oc login -u user1 -p secretpassword https://api.cluster.example.com:6443
    openshift_cluster_ingress_domain: apps.cluster.example.com
  user2:
    ...
```

## Running Locally

```bash
export USER_DATA_FILE=./users.yaml
pip install -r requirements.txt
python app.py
```

## Docker Build

```bash
podman build -t user-info-api:latest .
```

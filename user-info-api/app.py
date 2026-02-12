#!/usr/bin/env python3
"""
User Info API Server for Multi-User Workshops
Reads OAuth user from X-Forwarded-User header and returns user-specific data
"""

import os
import json
import yaml
from flask import Flask, jsonify, request
from flask_cors import CORS

app = Flask(__name__)
CORS(app)  # Enable CORS for cross-origin requests

# Load user data from ConfigMap-mounted file or environment
USER_DATA_FILE = os.getenv('USER_DATA_FILE', '/etc/user-data/users.yaml')
DEFAULT_CONSOLE_URL = os.getenv('DEFAULT_CONSOLE_URL', 'https://console-openshift-console.apps.cluster.example.com')
DEFAULT_API_URL = os.getenv('DEFAULT_API_URL', 'https://api.cluster.example.com:6443')
DEFAULT_INGRESS_DOMAIN = os.getenv('DEFAULT_INGRESS_DOMAIN', 'apps.cluster.example.com')

# Cache for user data
user_data_cache = {}

def load_user_data():
    """Load user data from YAML file"""
    global user_data_cache
    
    if not os.path.exists(USER_DATA_FILE):
        print(f"Warning: User data file not found: {USER_DATA_FILE}")
        return {}
    
    try:
        with open(USER_DATA_FILE, 'r') as f:
            data = yaml.safe_load(f)
            # Support both flat structure and nested 'users' key
            if 'users' in data:
                user_data_cache = data['users']
            else:
                user_data_cache = data
            print(f"Loaded user data for {len(user_data_cache)} users")
            return user_data_cache
    except Exception as e:
        print(f"Error loading user data: {e}")
        return {}

def get_user_info(username):
    """Get user-specific information"""
    # Reload user data if cache is empty (hot reload support)
    if not user_data_cache:
        load_user_data()
    
    user_info = user_data_cache.get(username, {})
    
    # Build response with defaults
    return {
        'user': username,
        'console_url': user_info.get('console_url', user_info.get('openshift_console_url', DEFAULT_CONSOLE_URL)),
        'password': user_info.get('password', ''),
        'login_command': user_info.get('login_command', f'oc login --insecure-skip-tls-verify=false -u {username} -p <password> {DEFAULT_API_URL}'),
        'openshift_cluster_ingress_domain': user_info.get('openshift_cluster_ingress_domain', DEFAULT_INGRESS_DOMAIN),
        'api_url': user_info.get('api_url', DEFAULT_API_URL)
    }

@app.route('/healthz')
def healthz():
    """Health check endpoint"""
    return jsonify({'status': 'ok'}), 200

@app.route('/api/user-info')
def user_info():
    """Return user-specific information based on authenticated user"""
    
    # Get username from OAuth proxy header
    username = request.headers.get('X-Forwarded-User')
    
    # Fallback to other common headers
    if not username:
        username = request.headers.get('X-Auth-Request-User')
    if not username:
        username = request.headers.get('X-Forwarded-Preferred-Username')
    
    # For development/testing without OAuth
    if not username:
        username = request.args.get('user', 'user1')
    
    print(f"User info request for: {username}")
    
    user_data = get_user_info(username)
    
    # Don't expose password in production unless explicitly enabled
    if os.getenv('HIDE_PASSWORDS', 'false').lower() == 'true':
        user_data['password'] = '***'
        user_data['login_command'] = user_data['login_command'].replace(user_data.get('password', ''), '***')
    
    return jsonify(user_data), 200

@app.route('/api/users')
def list_users():
    """List all available users (for admin/debugging)"""
    if not user_data_cache:
        load_user_data()
    
    return jsonify({
        'users': list(user_data_cache.keys()),
        'count': len(user_data_cache)
    }), 200

if __name__ == '__main__':
    # Load user data on startup
    load_user_data()
    
    # Run server
    port = int(os.getenv('PORT', '8081'))
    app.run(host='0.0.0.0', port=port, debug=False)

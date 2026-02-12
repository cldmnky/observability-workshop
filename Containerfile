# Multi-stage build for Antora site
# Stage 1: Build the Antora site
FROM registry.access.redhat.com/ubi9/nodejs-20:latest AS builder

USER root

# Install Antora CLI and site generator
RUN npm install -g @antora/cli@3.1 @antora/site-generator@3.1

# Copy site configuration and content
WORKDIR /workspace
COPY site-container.yml .
COPY ui-config.yml .
COPY content/ ./content/

# Build the Antora site
RUN antora site-container.yml

# Stage 2: Serve with nginx
FROM registry.access.redhat.com/ubi9/nginx-124:latest

# Copy built site from builder stage
COPY --from=builder /workspace/www /opt/app-root/src

# Copy custom nginx configuration with sub_filter for JavaScript injection
# UBI nginx includes /opt/app-root/etc/nginx.default.d/*.conf, not /etc/nginx/conf.d/
COPY nginx.conf /opt/app-root/etc/nginx.default.d/showroom-sub_filter.conf

# nginx runs on port 8080 by default in UBI
EXPOSE 8080

# Run as non-root user (default nginx user)
USER 1001

CMD ["nginx", "-g", "daemon off;"]

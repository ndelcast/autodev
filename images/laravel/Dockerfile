FROM php:8.4-cli-alpine

# PHP extensions
RUN apk add --no-cache \
    git nodejs npm curl sqlite-dev \
    && docker-php-ext-install pdo_sqlite

# Composer
COPY --from=composer:2 /usr/bin/composer /usr/local/bin/composer

# GitHub CLI
RUN apk add --no-cache github-cli

# Claude Code CLI
RUN npm install -g @anthropic-ai/claude-code

# Script d'exécution
COPY autodev-exec.sh /usr/local/bin/autodev-exec
RUN chmod +x /usr/local/bin/autodev-exec

# Non-root user (Claude Code refuses --dangerously-skip-permissions as root)
RUN adduser -D -h /home/autodev autodev
RUN mkdir -p /workspace /output /prompt && chown -R autodev:autodev /workspace /output /prompt
USER autodev

WORKDIR /workspace

FROM node:22-alpine

RUN apk add --no-cache git github-cli
RUN npm install -g @anthropic-ai/claude-code

COPY autodev-exec.sh /usr/local/bin/autodev-exec
RUN chmod +x /usr/local/bin/autodev-exec

# Non-root user (Claude Code refuses --dangerously-skip-permissions as root)
RUN adduser -D -h /home/autodev autodev
RUN mkdir -p /workspace /output /prompt && chown -R autodev:autodev /workspace /output /prompt
USER autodev

WORKDIR /workspace

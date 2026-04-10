FROM alpine:3.20

RUN apk add --no-cache git nodejs npm github-cli
RUN npm install -g @anthropic-ai/claude-code

COPY autodev-exec.sh /usr/local/bin/autodev-exec
RUN chmod +x /usr/local/bin/autodev-exec

WORKDIR /workspace

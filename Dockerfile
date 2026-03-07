# Asterisk OCI image — container-first MCP server.
#
# Build & run (hot-swap):
#   just container-restart
#
# Manual steps:
#   just build                    # produces bin/asterisk via origami fold
#   docker build -t asterisk .
#   docker run -d --name asterisk-server -p 9100:9100 -p 3001:3001 \
#     asterisk serve --transport http --kami-port 3001
#
# Cursor .cursor/mcp.json:
#   { "mcpServers": { "asterisk": { "url": "http://localhost:9100/mcp" } } }

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY bin/asterisk /app/asterisk
COPY internal/ /app/internal/
ENTRYPOINT ["/app/asterisk"]
EXPOSE 9100 3001

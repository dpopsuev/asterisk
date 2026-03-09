# Asterisk Domain Server — serves embedded domain data via MCP.
# Built by `origami fold --domain-only` + docker build, or
# `origami fold --domain-only --container` (automatic).
#
# This image is ONE piece of the distributed architecture:
#   Gateway (:9000) → RCA Engine (:9200) + Knowledge (:9100) + Domain (:9300)
#
# Build:
#   origami fold --domain-only --container    (automatic)
#   OR: origami fold --domain-only && docker build -t origami-asterisk-domain .
#
# Run (standalone):
#   docker run -p 9300:9300 origami-asterisk-domain

FROM gcr.io/distroless/static-debian12
COPY bin/asterisk-domain-serve /domain-serve
ENTRYPOINT ["/domain-serve"]
EXPOSE 9300

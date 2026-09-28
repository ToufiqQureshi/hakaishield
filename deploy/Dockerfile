# Build stage. CGO is off so the result is a static binary that runs on the
# distroless image below without a libc.
FROM golang:1.26-alpine AS build

WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/hakaishield .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/hakaishield-train ./cmd/hakaishield-train

# Runtime stage. distroless has no shell and no package manager, so there is
# nothing for someone who gets code execution to pick up and use.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/hakaishield /hakaishield
COPY --from=build /out/hakaishield-train /hakaishield-train

# Listens on 8443, not 443. Binding a port below 1024 needs root or a
# capability; publishing 443->8443 on the host avoids needing either.
EXPOSE 8443
USER nonroot:nonroot
ENTRYPOINT ["/hakaishield"]

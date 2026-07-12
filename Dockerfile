FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /agent ./cmd/agent

FROM alpine:3.20
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /agent /app/agent
COPY configs/ /app/configs/
EXPOSE 8080
ENTRYPOINT ["/app/agent"]
CMD ["-addr", ":8080", "-manifest", "configs/repos.json"]

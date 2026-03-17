FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/pgwatch-ai ./cmd

FROM alpine:3.20

RUN adduser -D -u 10001 appuser

WORKDIR /app
COPY --from=builder /bin/pgwatch-ai /usr/local/bin/pgwatch-ai

ENV GEMINI_API_KEY=
ENV GEMINI_MODEL=gemini-2.5-flash
ENV PWAI_SINK_DSN=

USER appuser

ENTRYPOINT ["pgwatch-ai"]
CMD ["ask", "check replication lag status"]

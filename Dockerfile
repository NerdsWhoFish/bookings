FROM node:26-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
COPY internal/webui/ /src/internal/webui/
RUN npm run build

FROM golang:1.27-alpine AS go
ARG VERSION=dev
ARG COMMIT=unknown
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/internal/webui/dist/ ./internal/webui/dist/
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" -o /out/bookings ./cmd/bookings

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=go /out/bookings /bookings
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/bookings"]

ARG BASE_IMAGE_REGISTRY=
FROM ${BASE_IMAGE_REGISTRY}golang:1.25-alpine@sha256:1ae0735f00daffa3aaf1363a5184c0d2dc55c78e3db4ec70241cdac97bf84b59 AS build

WORKDIR /src
ENV GOTOOLCHAIN=local
ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=sum.golang.google.cn
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /out/zbt-backend ./cmd/server
RUN go test ./...

FROM ${BASE_IMAGE_REGISTRY}alpine:3.22@sha256:5291449c3df73caf6ed85e649dec1b9e818b39a5d8c871e97afc13e9cd5e8fa8
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/zbt-backend /app/zbt-backend
EXPOSE 8080
CMD ["/app/zbt-backend"]

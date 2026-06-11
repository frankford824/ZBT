FROM golang:1.23-alpine AS build

WORKDIR /src
ENV GOTOOLCHAIN=local
ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=sum.golang.google.cn
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /out/zbt-backend ./cmd/server

FROM alpine:3.22
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/zbt-backend /app/zbt-backend
EXPOSE 8080
CMD ["/app/zbt-backend"]

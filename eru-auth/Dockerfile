FROM golang:1.20.4-alpine3.18

WORKDIR /build

# source files copied to build folder
COPY . .
WORKDIR eru-auth

# Build eru-auth
RUN GOOS=linux CGO_ENABLED=0 go build -a -ldflags '-s -w -extldflags "-static"' -o app .

FROM alpine:3.15.0

WORKDIR /build/eru-auth/app
COPY --from=0 /build/eru-auth/app .
RUN mkdir -p /app/config
CMD ["./app", "run"]


FROM golang:1.22.2-alpine3.19

WORKDIR /build

# source files copied to build folder
COPY . .
WORKDIR eru-functions

# Build eru-files
RUN GOOS=linux CGO_ENABLED=0 go build -a -ldflags '-s -w -extldflags "-static"' -o app .

FROM alpine:3.19.0

WORKDIR /build/eru-functions/app
COPY --from=0 /build/eru-functions/app .
RUN mkdir -p /app/config
CMD ["./app", "run"]


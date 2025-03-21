FROM golang:1.22.2-alpine3.19

WORKDIR /build

# source files copied to build folder
COPY . .
WORKDIR eru-ai

# Install playwright cli with right version for later use
RUN PWGO_VER=$(grep -oE "playwright-go v\S+" go.mod | sed 's/playwright-go //g') \
    && go install github.com/playwright-community/playwright-go/cmd/playwright@${PWGO_VER}


# Build eru-ai
RUN GOOS=linux CGO_ENABLED=0 go build -a -ldflags '-s -w -extldflags "-static"' -o app .



FROM ubuntu:noble

WORKDIR /build/eru-ai/app
COPY --from=0 /build/eru-ai/app .
RUN mkdir -p /app/config

COPY --from=0 /go/bin/playwright .

RUN apt-get update && apt-get install -y ca-certificates tzdata \
    # Install dependencies and all browsers (or specify one)
    && ./playwright install --with-deps \
    && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /app/config

CMD ["./app", "run"]
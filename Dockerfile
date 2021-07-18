# Support setting various labels on the final image
ARG COMMIT=""
ARG VERSION=""
ARG BUILDNUM=""

# Build Gbina in a stock Go builder container
FROM golang:1.16-alpine as builder

RUN apk add --no-cache gcc musl-dev linux-headers git

ADD . /go-binacoin
RUN cd /go-binacoin && go run build/ci.go install ./cmd/gbina

# Pull Gbina into a second stage deploy alpine container
FROM alpine:latest

RUN apk add --no-cache ca-certificates
COPY --from=builder /go-binacoin/build/bin/gbina /usr/local/bin/

EXPOSE 8545 8546 30303 30303/udp
ENTRYPOINT ["gbina"]

# Add some metadata labels to help programatic image consumption
ARG COMMIT=""
ARG VERSION=""
ARG BUILDNUM=""

LABEL commit="$COMMIT" version="$VERSION" buildnum="$BUILDNUM"

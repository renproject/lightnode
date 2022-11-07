FROM debian:bullseye-slim AS final
# Set up final runner first, so that it caches

# Install Filecoin and Solana dependencies.

RUN apt-get update && \
    apt install -y \
    ocl-icd-opencl-dev \
    ca-certificates \
    libgmp3-dev \
    libudev-dev \
    libssl-dev \
    libhwloc15 \
    libhwloc-dev && \
    rm -rf /var/lib/apt/lists/*

FROM renbot/multichain:latest as builder

# Compile cosmwasm dependency
WORKDIR /lightnode
RUN wget https://github.com/CosmWasm/go-cosmwasm/archive/v0.16.1.tar.gz
RUN tar -xzf v0.16.1.tar.gz
WORKDIR ./wasmvm-0.16.1
RUN ls -lah
RUN make build-rust

WORKDIR /lightnode

ARG GITHUB_TOKEN

RUN apt-get update && apt-get install -y ocl-icd-opencl-dev libgmp3-dev libhwloc-dev libhwloc15

# Use GitHub personal access token to fetch dependencies.
RUN git config --global url."https://tok-kkk:${GITHUB_TOKEN}@github.com".insteadOf "https://github.com"

# Mark private repositories.
ENV GOPRIVATE=github.com/renproject/darknode

# Copy and download go dependencies first so that it caches
COPY go.mod .
COPY go.sum .
RUN mkdir extern
RUN cp -r $GOPATH/src/github.com/filecoin-project/filecoin-ffi ./extern
RUN cp -r $GOPATH/src/github.com/renproject/solana-ffi ./extern
RUN go mod download

COPY . .

RUN go mod tidy
# Build the code inside the container.
RUN go build -ldflags="-s -w" ./cmd/lightnode 

FROM final

WORKDIR /lightnode
COPY --from=builder /lightnode/lightnode .
COPY --from=builder /lightnode/wasmvm-0.16.1/api/libwasmvm.so /usr/lib/

CMD ["./lightnode"]  

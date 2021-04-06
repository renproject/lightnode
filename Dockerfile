FROM golang as base

RUN apt-get update

# Install Filecoin dependencies.
RUN apt install -y ocl-icd-opencl-dev libgmp3-dev

# Install Solana dependencies.
RUN apt install -y libudev-dev libssl-dev

FROM renbot/multichain:latest as builder

# Compile cosmwasm dependency
WORKDIR /lightnode
RUN wget https://github.com/CosmWasm/go-cosmwasm/archive/v0.10.0.tar.gz
RUN tar -xzf v0.10.0.tar.gz
WORKDIR ./wasmvm-0.10.0
RUN ls -lah
RUN make build

WORKDIR /lightnode

ARG GITHUB_TOKEN

RUN apt install -y ocl-icd-opencl-dev libgmp3-dev

# Use GitHub personal access token to fetch dependencies.
RUN git config --global url."https://${GITHUB_TOKEN}:x-oauth-basic@github.com/".insteadOf "https://github.com/"

# Mark private repositories.
ENV GOPRIVATE=github.com/renproject/darknode

# Download Go dependencies.
COPY go.mod .
COPY go.sum .

# Use multichain image's filecoin
RUN go mod edit -replace=github.com/filecoin-project/filecoin-ffi=$GOPATH/src/github.com/filecoin-project/filecoin-ffi

# Use multichain image's solana 
RUN go mod edit -replace=github.com/renproject/solana-ffi=$GOPATH/src/github.com/renproject/solana-ffi

RUN go mod download

# Copy the code into the container.
COPY . .


# Build the code inside the container.
RUN go build -ldflags="-s -w" ./cmd/lightnode 

FROM base
WORKDIR /lightnode
COPY --from=builder /lightnode/lightnode .
COPY --from=builder /lightnode/wasmvm-0.10.0/api/libgo_cosmwasm.so /usr/lib/

CMD ["./lightnode"]  

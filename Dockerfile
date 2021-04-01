FROM renbot/multichain:latest
RUN apt-get update

# Install Filecoin dependencies.
RUN apt install -y ocl-icd-opencl-dev libgmp3-dev

# Install Solana dependencies.
RUN apt install -y libudev-dev libssl-dev

# Use GitHub personal access token to fetch dependencies.
ARG GITHUB_TOKEN
RUN git config --global url."https://${GITHUB_TOKEN}:x-oauth-basic@github.com/".insteadOf "https://github.com/"

# Mark private repositories.
ENV GOPRIVATE=github.com/renproject/darknode

# Download Go dependencies.
WORKDIR /lightnode
COPY go.mod .
COPY go.sum .
RUN go mod edit -dropreplace github.com/filecoin-project/filecoin-ffi
RUN go mod edit -dropreplace github.com/renproject/solana-ffi
RUN go mod download

# Copy the code into the container.
COPY . .
RUN go mod edit -replace=github.com/filecoin-project/filecoin-ffi=./extern/filecoin-ffi
RUN go mod edit -replace=github.com/renproject/solana-ffi=./extern/solana-ffi

# Build the code inside the container.
RUN go build ./cmd/lightnode

CMD ./lightnode

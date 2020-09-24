FROM golang

# Set necessary environmet variables needed for our image.
ENV GO111MODULE=on

# Use our GitHub access token to fetch dependencies.
ARG GITHUB_TOKEN
RUN git config --global url."https://${GITHUB_TOKEN}:x-oauth-basic@github.com/".insteadOf "https://github.com/"

# Download dependencies.
WORKDIR /lightnode
COPY go.mod .
COPY go.sum .
RUN go mod download

# Copy the code into the container.
COPY . .

# Download Filecoin dependencies
RUN apt-get autoclean
RUN apt-get update
RUN apt-get install -y jq
RUN apt-get install -y ocl-icd-opencl-dev
RUN git submodule add --force https://github.com/filecoin-project/filecoin-ffi.git extern/filecoin-ffi
WORKDIR /lightnode/extern/filecoin-ffi
RUN git checkout 777a6fbf4446b1112adfd4fa5dd88e0c88974122
RUN make
WORKDIR /lightnode
RUN go mod edit -replace=github.com/filecoin-project/filecoin-ffi=./extern/filecoin-ffi

# Build the code inside the container.
RUN go build ./cmd/lightnode
CMD ./lightnode
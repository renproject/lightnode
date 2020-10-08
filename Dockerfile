FROM golang

# Set necessary environmet variables needed for our image.
ENV GO111MODULE=on

# Use our GitHub access token to fetch dependencies.
ARG GITHUB_TOKEN
RUN git config --global url."https://${GITHUB_TOKEN}:x-oauth-basic@github.com/".insteadOf "https://github.com/"

# Download Filecoin dependencies
RUN apt-get autoclean
RUN apt-get update
RUN apt-get install -y jq
RUN apt-get install -y ocl-icd-opencl-dev
RUN git clone https://github.com/filecoin-project/filecoin-ffi.git /extern/filecoin-ffi
WORKDIR /extern/filecoin-ffi
RUN git checkout a62d00da59d1b0fb35f3a4ae854efa9441af892d
RUN make

# Download dependencies.
WORKDIR /lightnode
COPY go.mod .
COPY go.sum .
RUN go mod download

# Copy the code into the container.
COPY . .
RUN go mod edit -replace=github.com/filecoin-project/filecoin-ffi=../extern/filecoin-ffi

# Build the code inside the container.
RUN go build ./cmd/lightnode
CMD ./lightnode
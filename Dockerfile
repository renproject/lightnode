FROM renbot/multichain:latest

# Use GitHub personal access token to fetch dependencies.
ARG GITHUB_TOKEN
RUN git config --global url."https://${GITHUB_TOKEN}:x-oauth-basic@github.com/".insteadOf "https://github.com/"

# Mark private repositories
ENV GOPRIVATE=github.com/renproject/darknode

# Download dependencies.
WORKDIR /lightnode
COPY go.mod .
COPY go.sum .
RUN go mod download

# Copy the code into the container.
COPY . .
RUN go mod edit -replace=github.com/filecoin-project/filecoin-ffi=$(go env GOPATH)/src/github.com/filecoin-project/filecoin-ffi

# Build the code inside the container.
RUN go build ./cmd/lightnode

CMD ./lightnode

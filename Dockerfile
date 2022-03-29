FROM golang:1.18.0-alpine3.15

WORKDIR  /usr/src/github-workflow-dashboard

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

# more details on -buildvcs=false flag here: https://github.com/golang/go/issues/51748#issuecomment-1070636297
RUN go build -v -buildvcs=false -o /usr/local/bin/github-workflow-dashboard ./cmd/...

EXPOSE 8080

CMD ["github-workflow-dashboard"]
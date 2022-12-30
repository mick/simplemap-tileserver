FROM golang:1.19-bullseye

WORKDIR /tileserver

RUN apt-get update -y && apt-get install libsqlite3-dev -y

COPY . .
RUN go mod download

RUN --mount=type=cache,target=/root/.cache/go-build \
GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o ./tileserver

CMD [ "./tileserver" ]
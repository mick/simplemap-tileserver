FROM golang:1.18-bullseye

WORKDIR /tileserver

RUN apt-get update -y && apt-get install libsqlite3-dev -y

COPY . .
RUN go mod download

RUN go build -o ./tileserver

CMD [ "./tileserver" ]
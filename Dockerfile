FROM golang:1.25.4

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./

RUN go build -v -o GoRainbowTable . 

CMD [ "./GoRainbowTable" ]
FROM golang:1.24.2-alpine as builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLRED=0 GOOS=linux go build -o teamAndProgects ./cmd

FROM alpine:3.21

WORKDIR /app

COPY --from=builder /app/teamAndProgects ./teamAndProgects

EXPOSE 8080

CMD [ "./teamAndProgects" ]
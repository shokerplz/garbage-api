FROM golang:1.18.1-alpine
RUN mkdir /app
WORKDIR /app
COPY go.mod ./
COPY go.sum ./
COPY *.go ./
RUN go build -o /bookking-api
EXPOSE 8080
CMD [ "/bookking-api" ]

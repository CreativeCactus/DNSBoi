FROM golang
RUN go get github.com/gorilla/mux
RUN go get github.com/sirupsen/logrus
COPY ./ /go/app/dnsboi/
WORKDIR /go/app/dnsboi/
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
    go build -o dnsboi /go/app/dnsboi/main.go

FROM golang:alpine
COPY --from=0 /go/app/dnsboi/ /home/
RUN chmod +x /home/dnsboi
ENTRYPOINT /home/dnsboi
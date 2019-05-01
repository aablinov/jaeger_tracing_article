FROM golang
RUN mkdir -p /go/src/github.com/ablinov/tracing_article
ADD . /go/src/github.com/ablinov/tracing_article
WORKDIR /go/src/github.com/ablinov/tracing_article
RUN go build -o service
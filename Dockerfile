FROM golang
COPY *.go /tmp/
RUN cd /tmp && go get -d -v
RUN go build -o /usr/bin/happybot /tmp/*.go

TARGET=happybot

build:
	go build -o $(TARGET) *.go

clean:
	rm -rf $(TARGET)

fmt:
	go fmt *.go
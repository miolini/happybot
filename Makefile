TARGET=happybot
TOKEN=$(shell cat .token)

build:
	go build -o $(TARGET) *.go

clean:
	rm -rf $(TARGET)

fmt:
	go fmt *.go

docker_build:
	docker build -t miolini/happybot .

docker: docker_build docker_stop
	docker run --name happybot -d -it miolini/happybot happybot -t $(TOKEN)

docker_shell: docker_build docker_stop
	docker run --name happybot -it miolini/happybot bash

docker_stop:
	docker rm -f happybot || true
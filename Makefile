VERSION = 1.0.0
LDFLAGS = -ldflags '-s -w'
GOARCH = amd64
linux: export GOOS=linux
darwin: export GOOS=darwin

all: linux darwin

linux:
	go build $(LDFLAGS)
	mkdir -p release
	rm -f release/ec2-run-${VERSION}-${GOOS}_${GOARCH}.zip
	zip release/ec2-run-${VERSION}-${GOOS}_${GOARCH}.zip ec2-run

darwin:
	go build $(LDFLAGS)
	mkdir -p release
	rm -f release/ec2-run-${VERSION}-${GOOS}_${GOARCH}.zip
	zip release/ec2-run-${VERSION}-${GOOS}_${GOARCH}.zip ec2-run

.PHONY: clean
clean:
	rm -rf release
	rm -f ec2-run

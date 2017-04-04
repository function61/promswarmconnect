.PHONY: all build

all: build

build:
	# compile statically so this works on Alpine that doesn't have glibc
	CGO_ENABLED=0 go build --ldflags '-extldflags "-static"'

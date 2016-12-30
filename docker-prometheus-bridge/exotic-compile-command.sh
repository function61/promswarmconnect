
# this exotic command was required to make it compile statically
# (thanks http://matthewkwilliams.com/index.php/2014/09/28/go-executables-are-statically-linked-except-when-they-are-not/)
#
# why statically? Alpine & busybox doesn't have glibc but musl, and normal Go compilation dynamically links to glibc

CGO_ENABLED=0 go build --ldflags '-extldflags "-static"' docker-prometheus-bridge.go

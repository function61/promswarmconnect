#!/bin/sh -eu

# ugly hack - would've used forego as process manager but it
# requires glibc and thus doesn't work with busybox
docker-prometheus-bridge &

exec prometheus \
	-config.file=/etc/prometheus/prometheus.yml \
	-storage.local.path=/prometheus \
	-web.console.libraries=/usr/share/prometheus/console_libraries \
	-web.console.templates=/usr/share/prometheus/consoles

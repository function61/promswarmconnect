FROM prom/prometheus:v1.4.1

ENV METRICS_ENDPOINT=:8081/metrics
LABEL METRICS_ENDPOINT=:8081/metrics

ADD conf/targets-from-swarm.initially_empty.json /etc/prometheus/targets-from-swarm.json
ADD conf/prometheus.yml /etc/prometheus/prometheus.yml
ADD app /bin/docker-prometheus-bridge
ADD run.sh /bin

RUN chmod +x /bin/docker-prometheus-bridge

# reset entrypoint from base image
ENTRYPOINT []

CMD run.sh

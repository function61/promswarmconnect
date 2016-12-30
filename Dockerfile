FROM prom/prometheus:v1.4.1

ADD conf/targets-from-swarm.initially_empty.json /etc/prometheus/targets-from-swarm.json
ADD conf/prometheus.yml /etc/prometheus/prometheus.yml
ADD docker-prometheus-bridge/docker-prometheus-bridge /bin/docker-prometheus-bridge
ADD run.sh /bin

RUN chmod +x /bin/docker-prometheus-bridge

# reset entrypoint from base image
ENTRYPOINT []

CMD run.sh

FROM scratch

CMD ["/promswarmconnect"]

ADD rel/promswarmconnect_linux-amd64 /promswarmconnect

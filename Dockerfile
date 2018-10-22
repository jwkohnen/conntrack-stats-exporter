FROM	golang:1.11 as build
WORKDIR /conntrack-stats-exporter
COPY	. .
RUN	./build.sh

FROM	debian:stretch-slim
COPY	--from=build /conntrack-stats-exporter/conntrack-stats-exporter /usr/local/sbin/
RUN	apt-get update \
&&	apt-get install -y conntrack \
&&	apt-get clean \
&&	rm -rf /var/lib/apt/lists/*
ENTRYPOINT [ "/usr/local/sbin/conntrack-stats-exporter" ]

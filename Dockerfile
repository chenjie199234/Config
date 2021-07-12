FROM debian:stable-slim
RUN apt-get update && apt-get install -y ca-certificates && mkdir /root/app && mkdir /root/app/k8sconfig && mkdir /root/app/remoteconfig
WORKDIR /root/app
ENV DISCOVERY_SERVER_GROUP='<DISCOVERY_SERVER_GROUP>' \
    DISCOVERY_SERVER_NAME='<DISCOVERY_SERVER_NAME>' \
    DISCOVERY_SERVER_PORT='<DISCOVERY_SERVER_PORT>'
ENV SERVER_VERIFY_DATA='<SERVER_VERIFY_DATA>' \
    CONFIG_TYPE='<CONFIG_TYPE>' \
    RUN_ENV='<RUN_ENV>'
COPY main probe.sh AppConfig.json SourceConfig.json ./
ENTRYPOINT ["./main"]

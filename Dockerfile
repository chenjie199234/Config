FROM busybox:1.28

RUN mkdir /root/app
RUN mkdir /root/app/k8sconfig
RUN mkdir /root/app/remoteconfig
WORKDIR /root/app
SHELL ["/bin/sh","-c"]
ENV DISCOVERY_SERVER_GROUP='<DISCOVERY_SERVER_GROUP>' \
    DISCOVERY_SERVER_NAME='<DISCOVERY_SERVER_NAME>' \
    DISCOVERY_SERVER_PORT='<DISCOVERY_SERVER_PORT>'
ENV SERVER_VERIFY_DATA='<SERVER_VERIFY_DATA>' \
    CONFIG_TYPE='<CONFIG_TYPE>' \
    RUN_ENV='<RUN_ENV>'
COPY main probe.sh AppConfig.json SourceConfig.json ./
ENTRYPOINT ["./main"]
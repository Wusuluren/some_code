FROM        alpine:3.11
EXPOSE      9000
ADD         ./output/gateway-server /
#ADD			./conf/all.yml /conf/
ENTRYPOINT  ["/gateway-server"]

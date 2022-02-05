FROM        alpine:3.11
EXPOSE      9002
ADD         ./output/ip-server /
#ADD         ./conf/all.yml /conf/
ENTRYPOINT  ["/ip-server"]

FROM        alpine:3.11
EXPOSE      9001
ADD         ./output/time-server /
#ADD         ./conf/all.yml /conf/
ENTRYPOINT  ["/time-server"]

FROM        alpine:3.11
ADD         ./output/reset-counter /
ADD         ./conf/all.yml /conf/
ENTRYPOINT  ["/reset-counter"]

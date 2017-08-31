FROM debian:stretch

COPY metacontroller /usr/bin

CMD /usr/bin/metacontroller

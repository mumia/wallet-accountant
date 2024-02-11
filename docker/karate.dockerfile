FROM eclipse-temurin:21-jre

WORKDIR /workdir

#COPY . /workdir
COPY ./docker/karate/karate.sh /workdir/karate.sh
COPY ./docker/karate/karate-1.4.1.jar /workdir/karate.jar
COPY ./test /workdir/test

RUN chmod +x /workdir/karate.sh

ENTRYPOINT /workdir/karate.sh

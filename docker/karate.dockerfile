FROM eclipse-temurin:21-jre

COPY . /workdir
COPY ./docker/karate/karate.sh /app/karate.sh
COPY ./docker/karate/karate-1.4.1.jar /app/karate.jar


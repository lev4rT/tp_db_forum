FROM golang:1.15.2-buster AS build

# Копируем исходный код в Docker-контейнер
ADD . /opt/build/golang/
WORKDIR /opt/build/golang/
RUN go build main.go


FROM ubuntu:20.04 AS release

MAINTAINER Denis Kolosov

# Make the "en_US.UTF-8" locale so postgres will be utf-8 enabled by default
RUN apt -y update && apt install -y locales gnupg2
RUN locale-gen en_US.UTF-8
RUN update-locale LANG=en_US.UTF-8

#
# Install postgresql
#
ENV PGVER 12
ENV DEBIAN_FRONTEND noninteractive
RUN apt-get update -y && apt-get install -y postgresql postgresql-contrib

# Run the rest of the commands as the ``postgres`` user created by the ``postgres-$PGVER`` package when it was ``apt-get installed``
USER postgres

# Create a PostgreSQL role named ``docker`` with ``docker`` as the password and
# then create a database `docker` owned by the ``docker`` role.
RUN /etc/init.d/postgresql start &&\
    psql --command "CREATE USER docker WITH SUPERUSER PASSWORD 'docker';" &&\
    createdb -O docker docker &&\
    /etc/init.d/postgresql stop


# Adjust PostgreSQL configuration so that remote connections to the
# database are possible.
RUN echo "host all  all    0.0.0.0/0  md5" >> /etc/postgresql/$PGVER/main/pg_hba.conf

# And add ``listen_addresses`` to ``/etc/postgresql/$PGVER/main/postgresql.conf``
RUN echo "listen_addresses='*'\nsynchronous_commit = off\nfsync = off\nmax_connections = 100\nshared_buffers = 512MB\neffective_cache_size = 1144MB\nmaintenance_work_mem = 320MB\ncheckpoint_completion_target = 0.7\nwal_buffers = 16MB\ndefault_statistics_target = 100\nrandom_page_cost = 1.1\neffective_io_concurrency = 200\nwork_mem = 10485kB\nmin_wal_size = 1GB\nmax_wal_size = 4GB\nmax_worker_processes = 2\nmax_parallel_workers_per_gather = 1\nmax_parallel_workers = 2\nmax_parallel_maintenance_workers = 1" >> /etc/postgresql/$PGVER/main/postgresql.conf

# Expose the PostgreSQL port
EXPOSE 5432

# Add VOLUMEs to allow backup of config, logs and databases
VOLUME  ["/etc/postgresql", "/var/log/postgresql", "/var/lib/postgresql"]

# Back to the root user
USER root

# Объявлем порт сервера
EXPOSE 5000

# Собранный ранее сервер
COPY --from=build /opt/build/golang/ .

#
# Запускаем PostgreSQL и сервер
#
#CMD service postgresql start && ./main
ENV PGPASSWORD docker
CMD service postgresql start &&  psql -h localhost -d docker -U docker -p 5432 -a -q -f ./script.sql && ./main
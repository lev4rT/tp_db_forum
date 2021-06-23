FROM golang:1.16 AS build

# Копируем исходный код в Docker-контейнер
ADD . /opt/app
WORKDIR /opt/app
RUN go build ./main.go


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
RUN echo "listen_addresses='*'\nsynchronous_commit = off\nfsync = off\nshared_buffers = 256MB\neffective_cache_size = 1536MB\n" >> /etc/postgresql/$PGVER/main/postgresql.conf
RUN echo "wal_buffers = 1MB\nwal_writer_delay = 50ms\nrandom_page_cost = 1.0\nmax_connections = 100\nwork_mem = 8MB\nmaintenance_work_mem = 128MB\ncpu_tuple_cost = 0.0030\ncpu_index_tuple_cost = 0.0010\ncpu_operator_cost = 0.0005" >> /etc/postgresql/$PGVER/main/postgresql.conf
RUN echo "full_page_writes = off" >> /etc/postgresql/$PGVER/main/postgresql.conf
RUN echo "log_statement = none" >> /etc/postgresql/$PGVER/main/postgresql.conf
RUN echo "log_duration = off " >> /etc/postgresql/$PGVER/main/postgresql.conf
RUN echo "log_lock_waits = on" >> /etc/postgresql/$PGVER/main/postgresql.conf
RUN echo "log_min_duration_statement = 100000" >> /etc/postgresql/$PGVER/main/postgresql.conf
RUN echo "log_filename = 'query.log'" >> /etc/postgresql/$PGVER/main/postgresql.conf
RUN echo "log_directory = '/var/log/postgresql'" >> /etc/postgresql/$PGVER/main/postgresql.conf
RUN echo "log_destination = 'csvlog'" >> /etc/postgresql/$PGVER/main/postgresql.conf
RUN echo "logging_collector = on" >> /etc/postgresql/$PGVER/main/postgresql.conf
RUN echo "log_temp_files = '-1'" >> /etc/postgresql/$PGVER/main/postgresql.conf

# Expose the PostgreSQL port
EXPOSE 5432

# Add VOLUMEs to allow backup of config, logs and databases
VOLUME  ["/etc/postgresql", "/var/log/postgresql", "/var/lib/postgresql"]

# Back to the root user
USER root

# Объявлем порт сервера
EXPOSE 5000

WORKDIR /usr/workdir

COPY . .

# Собранный ранее сервер
COPY --from=build /opt/app/main .

#
# Запускаем PostgreSQL и сервер
#
#CMD service postgresql start && ./main
ENV PGPASSWORD docker
CMD service postgresql start &&  psql -h localhost -d docker -U docker -p 5432 -a -q -f ./script.sql && ./main
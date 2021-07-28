FROM registry.ci.openshift.org/openshift/release:golang-1.16 AS builder
WORKDIR /go/src/github.com/bradmwilliams/mysql-client
COPY . .
RUN make

FROM registry.ci.openshift.org/openshift/mysql:latest
COPY --from=builder /go/src/github.com/bradmwilliams/mysql-client/mysql-client /usr/bin/
#ENTRYPOINT ["/usr/bin/prowconfig-tester"]
CMD ["/usr/bin/mysql-client"]

FROM centos:7
RUN yum install -y skopeo && mkdir /autoupdate -p
ADD autoupdate /autoupdate/autoupdate
ADD config /autoupdate/config
ADD template /autoupdate/template
ADD image /autoupdate/image
ADD scripts /autoupdate/scripts
WORKDIR /autoupdate
ENTRYPOINT ["./autoupdate"]

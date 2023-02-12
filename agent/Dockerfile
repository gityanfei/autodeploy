FROM centos:7
RUN mkdir  -p /agent
ADD config/sqconfig /agent/config/sqconfig
#ADD scripts /agent/scripts
ADD agent /agent
WORKDIR /agent
CMD ["./agent"]

FROM golang
MAINTAINER yunlong.lee@163.com

VOLUME /opt

RUN apt-get update -y
RUN go get github.com/tools/godep
RUN apt-get update && apt-get install -y wget git make ; \ 

cd /opt ; \ 
mkdir -p skytime ; \ 

export PATH=$GOROOT/bin:$GOPATH/bin:$PATH ; \ 
go get -d github.com/skytime ; \ 
cd $GOPATH/src/github.com/skytime ; \ 
make ; cp -r * /opt/skytime

# Expose ports
EXPOSE 7090
EXPOSE 8000
EXPOSE 8001

CMD ["cd /opt/skytime && ./bin/skytime"]

FROM circleci/golang:1.11.2

RUN go get -u github.com/tcnksm/ghr \
    && go get -u github.com/stevenmatthewt/semantics

RUN curl -L -s https://github.com/golang/dep/releases/download/v0.5.0/dep-linux-amd64 -o /go/bin/dep \
    && chmod +x /go/bin/dep

RUN mkdir -p /go/src/github.com/mitchellh/gox
RUN git clone --branch master https://github.com/mitchellh/gox.git /go/src/github.com/mitchellh/gox
RUN cd /go/src/github.com/mitchellh/gox && git reset --hard e05df8dccde397491cd731693a93d4506b5850d3
RUN cd /go/src/github.com/mitchellh/gox && dep ensure && go install -v ./...

RUN go get github.com/mattn/goveralls

RUN sudo apt-get install ruby
RUN sudo gem install asciidoctor

RUN wget https://github.com/jgm/pandoc/releases/download/2.4/pandoc-2.4-1-amd64.deb
RUN sudo dpkg -i pandoc-2.4-1-amd64.deb

RUN go get -u golang.org/x/lint/golint
RUN go get -u github.com/gordonklaus/ineffassign
RUN wget -O gometalinter.tar.gz https://github.com/alecthomas/gometalinter/releases/download/v2.0.11/gometalinter-2.0.11-linux-amd64.tar.gz \
    && tar -xvf gometalinter.tar.gz \
    && rm gometalinter.tar.gz
ENV PATH="${PATH}:/go/gometalinter"

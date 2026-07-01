from rockylinux:9

RUN dnf install -y bash rpm-build systemd systemd-rpm-macros rpmdevtools make gcc which git golang
RUN curl -L https://go.dev/dl/go1.26.4.linux-amd64.tar.gz | tar -zx -C /usr/local
RUN echo "export PATH=/usr/local/go/bin:$PATH" >> /.bashrc
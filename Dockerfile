FROM fedora:31

RUN dnf install -y \
  # Commong tools
  git \
  tar \
  flex \
  byacc \
  unzip \
  wget \
  make \
  file \
  python \
  # Linux x86 and x64
  gcc \
  gcc-c++ \
  libgcc.i686 \
  gcc-c++.i686 \
  glibc-devel \
  glibc-static \
  glibc-devel.i686 \
  glib2-static.i686 \
  libpcap.x86_64 \
  libpcap.i686 \
  libpcap-devel.x86_64 \
  libpcap-devel.i686 \
  # Windows x64
  mingw32-gcc.x86_64 \
  mingw64-gcc.x86_64 \
  mingw32-wpcap.noarch \
  mingw64-wpcap.noarch \
  && dnf clean packages

# For ARM cross compilation
RUN dnf install -y dnf-plugins-core && \
  dnf copr enable -y lantw44/arm-linux-gnueabi-toolchain && \
  dnf install -y arm-linux-gnueabi-{binutils,gcc,glibc} && \
  dnf clean packages

RUN mkdir -p \
  /opt \
  /go/src/github.com/malfunkt/arpfox

ENV ANDROID_NDK_URL=https://dl.google.com/android/repository/android-ndk-r20-linux-x86_64.zip

ENV LIBPCAP_URL=https://www.tcpdump.org/release/libpcap-1.9.0.tar.gz

ENV GO_TARBALL=https://dl.google.com/go/go1.14.2.linux-amd64.tar.gz

# Android toolchain
RUN wget --quiet -O /opt/android-ndk.zip $ANDROID_NDK_URL

RUN cd /opt && \
  unzip -q android-ndk.zip && \
  rm android-ndk.zip

RUN cd /opt/android-ndk-* && \
  build/tools/make_standalone_toolchain.py \
    --arch=arm \
    --api=23 \
    --install-dir=/opt/android-toolchain \
  && rm -rf /opt/android-ndk-*

# libpcap (for linux-arm and android)
RUN curl --silent -L $LIBPCAP_URL | tar -xzf - -C /opt

RUN cd /opt/libpcap-* && \
  CC="/opt/android-toolchain/bin/arm-linux-androideabi-gcc" \
  LD="/opt/android-toolchain/bin/arm-linux-androideabi-ld" \
	./configure --prefix=/opt/android-toolchain --host=arm-linux --with-pcap=linux && \
	make clean install

RUN cd /opt/libpcap-* && \
  CC="/usr/bin/arm-linux-gnueabi-gcc" \
  LD="/usr/bin/arm-linux-gnueabi-ld" \
  ./configure --prefix=/usr/arm-linux-gnueabi --host=arm-linux --with-pcap=linux && \
  make clean install

# Go
RUN curl --silent -L $GO_TARBALL | tar -xzf - -C /usr/local

ENV GOROOT /usr/local/go
ENV GOPATH /go
ENV PATH $PATH:$GOROOT/bin:$GOPATH/bin
ENV GO111MODULE=on

WORKDIR /go/src/github.com/malfunkt/arpfox
COPY . .

RUN go mod vendor

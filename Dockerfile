FROM fedora:23

RUN dnf install -y \
  gcc \
  gcc-c++ \
  libgcc.i686 \
  gcc-c++.i686 \
  glibc-devel \
  glibc-static \
  glibc-devel.i686 \
  glib2-static.i686 && \
  dnf clean packages

RUN dnf install -y \
  gcc-arm-linux-gnu.x86_64 \
  glibc-arm-linux-gnu-devel.noarch \
  glibc-arm-linux-gnu.noarch && \
  dnf clean packages

# For some reason, ld gets confused and tries to load
# "/usr/arm-linux-gnu/lib/libpthread.so.0" relatively from withint
# "/usr/arm-linux-gnu/" which makes Go fail with the following message:
#
# # runtime/cgo
# /usr/bin/arm-linux-gnu-ld: cannot find /usr/arm-linux-gnu/lib/libpthread.so.0 inside /usr/arm-linux-gnu/
# /usr/bin/arm-linux-gnu-ld: cannot find /usr/arm-linux-gnu/lib/libpthread_nonshared.a inside /usr/arm-linux-gnu/
#
# I tried to find a clean and easy solution but failed, so I added this
# workaround which tricks ld:
RUN ln -s /usr /usr/arm-linux-gnu/usr

# For windows

RUN dnf install -y mingw32-gcc.x86_64 mingw64-gcc.x86_64 && dnf clean packages

# libpcap for linux and windows on x64 and x86
RUN dnf install -y \
  libpcap.x86_64 \
  libpcap.i686 \
  libpcap-devel.x86_64 \
  libpcap-devel.i686 \
  mingw32-wpcap.noarch \
  mingw64-wpcap.noarch && \
  dnf clean packages

# Android toolchain

RUN dnf install -y flex byacc unzip wget make file && dnf clean packages

RUN mkdir -p /opt

ENV ANDROID_NDK_URL=https://dl.google.com/android/repository/android-ndk-r12b-linux-x86_64.zip

RUN wget -O /opt/android-ndk.zip $ANDROID_NDK_URL

RUN cd /opt && \
  unzip android-ndk.zip && \
  rm android-ndk.zip

RUN cd /opt/android-ndk-* && \
  build/tools/make_standalone_toolchain.py --arch=arm --api=19 --install-dir=/opt/android-toolchain && \
  rm -rf /opt/android-ndk-*

# libpcap for linux arm

RUN dnf install -y tar && dnf clean packages

ENV LIBPCAP_URL=http://www.tcpdump.org/release/libpcap-1.6.2.tar.gz

RUN curl -L $LIBPCAP_URL | tar xvzf - -C /opt

RUN cd /opt/libpcap-* && \
  CC=/opt/android-toolchain/bin/arm-linux-androideabi-gcc \
  LD=/opt/android-toolchain/bin/arm-linux-androideabi-ld \
	./configure --prefix=/opt/android-toolchain --host=arm-linux --with-pcap=linux && \
	make clean install

RUN cd /opt/libpcap-* && \
  CC="/usr/bin/arm-linux-gnu-gcc -I/usr/arm-linux-gnu/include --sysroot=/usr/arm-linux-gnu" \
  LD="/usr/bin/arm-linux-gnu-ld -L/usr/arm-linux-gnu/lib" \
  ./configure --prefix=/usr/arm-linux-gnu --host=arm-linux --with-pcap=linux && \
  make clean install

RUN mkdir -p /app

# Go

RUN dnf install -y git mercurial && dnf clean packages

ENV GO_TARBALL=https://storage.googleapis.com/golang/go1.7.1.linux-amd64.tar.gz
RUN curl $GO_TARBALL | tar -xvzf - -C /usr/local

ENV GOROOT /usr/local/go
ENV GOPATH /app
ENV PATH $PATH:$GOROOT/bin:$GOPATH/bin

RUN mkdir -p /app/src/github.com/malfunkt/arpfox
WORKDIR /app/src/github.com/malfunkt/arpfox

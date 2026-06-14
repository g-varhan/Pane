Name:           pane
Version:        0.1.0
Release:        1%{?dist}
Summary:        A modular, high-performance hypervisor orchestration engine using KVM, io_uring, and eBPF

License:        ASL 2.0
URL:            https://github.com/g-varhan/Pane
Source0:        %{name}-%{version}.tar.gz

BuildRequires:  golang, cargo, rust, clang, make, liburing-devel, glibc-devel
Requires:       liburing, glibc

%description
Pane is a low-latency, modular hypervisor orchestration engine designed to embed KVM virtual machine lifecycle operations directly into platforms.

%prep
%setup -q

%build
# 1. Build C VMM static library
make -C pane-vmm clean
make -C pane-vmm CFLAGS="-Wall -Wextra -Werror -O2"

# 2. Build Rust core orchestration library
cd pane-core
cargo build --release
cd ..

# 3. Build Go gRPC Server
cd pane-api
CGO_ENABLED=1 go build -ldflags="-s -w" -o pane-api main.go
cd ..

%install
rm -rf $RPM_BUILD_ROOT
mkdir -p %{buildroot}%{_bindir}
mkdir -p %{buildroot}%{_includedir}
mkdir -p %{buildroot}%{_libdir}

# Install binaries
install -m 755 pane-api/pane-api %{buildroot}%{_bindir}/pane-api

# Install headers and libraries
install -m 644 pane-vmm/include/pane_vmm.h %{buildroot}%{_includedir}/pane_vmm.h
install -m 644 pane-vmm/libpane_vmm.a %{buildroot}%{_libdir}/libpane_vmm.a

%clean
rm -rf $RPM_BUILD_ROOT

%files
%{_bindir}/pane-api
%{_includedir}/pane_vmm.h
%{_libdir}/libpane_vmm.a

%changelog
* Sun Jun 14 2026 Pane Developers <info@pane.dev> - 0.1.0-1
- Initial release of Pane

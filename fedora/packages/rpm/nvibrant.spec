Name:           nvibrant
Version:        1.3.1
Release:        1%{?dist}
Summary:        NVIDIA digital vibrance and dithering on Wayland
License:        MIT
URL:            https://github.com/Tremeschin/nvibrant
Source0:        %{url}/releases/download/v%{version}/nvibrant-%{version}-py3-none-manylinux_2_17_x86_64.whl
Source1:        %{url}/raw/v%{version}/license.txt#/nvibrant-license.txt
ExclusiveArch:  x86_64

BuildRequires:  python3-devel
BuildRequires:  python3-installer
Requires:       python3
Requires:       python3-packaging
Provides:       nvibrant-bin = %{version}-%{release}
Conflicts:      nvibrant-bin

%description
nvibrant controls NVIDIA digital vibrance and dithering under Wayland through
the nvidia-modeset device.

%prep
cp -p %{SOURCE1} license.txt

%build

%install
python3 -m installer --destdir=%{buildroot} %{SOURCE0}
%py3_shebang_fix %{buildroot}%{_bindir}/nvibrant

%files
%license license.txt
%{_bindir}/nvibrant
%{python3_sitearch}/nvibrant/
%{python3_sitearch}/nvibrant-%{version}.dist-info/

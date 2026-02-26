# Prerequisites
libpcap-dev is required to build the project.

### Ubuntu
```bash
sudo apt-get install libpcap-dev
```

### Arch Linux
```bashbash
sudo pacman -S libpcap
```

### Fedora
```bash
sudo dnf install libpcap-devel
```

### Alpine
```bash
sudo apk add libpcap-dev
```

# Installation
Packet capture requires elevated privileges to access network interfaces. You can either run the program with `sudo` or set the necessary capabilities on the executable.
```bash
go build -o pkt_cap cmd/pkt_cap/main.go
sudo setcap 'cap_net_raw,cap_net_admin=eip' ./pcap 
```
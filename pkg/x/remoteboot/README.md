#  InfraKit Remote Boot [EXPERIMENTAL]

The Remote Boot functionality simplifies the deployment of Operating System instances by booting them to remote systems over the network. This functionality is provided by using DHCP to initially configure the newly booting system. TFTP then provides the initial booting image, which makes use of iPXE functionality to chain the booting of a remote Kernel/Initrd. Then a HTTP server hosts the kernel and initrd that iPXE will bootstrap the system with.

## Building:
Follow the majority of steps on the [InfraKit github repository](http://github.com/docker/infrakit) and build the infrakit tool with the following make command:

`make build/infrakit`

## Usage:

The remoteboot function requires **root** priviliges in order to be used, which is due to:
- UDP Ports in the < 1024 range (67 DHCP, 69 TFTP)
- TCP Port 80 HTTP (No HTTPS)
- Binding to an adapter in order to broadcast

This typically means that sudo will be required to start the services e.g. `sudo ./build/infrakit x remoteboot`

A PXE boot loader is needed to automate the booting process as the system comes up. In order to simplify this and provide the functionality the remoteboot functionality makes use of the [http://ipxe.org](http://ipxe.org) project and their open source bootloader. In order to use this, initially start the remoteboot function with the following command:

`./infrakit x remoteboot pulliPXE` 

This will download the iPXE boot loader from the iPXE project site. On first run remoteboot will create a infrakit.ipxe file that contains the auto-built paths to the kernel and initrd. If you change the kernel you'd like to boot from then you'll need to rename or delete the ipxe script.

#### Required Arguments:

**First argument** - path to a kernel file

**Second argument** - path to an initrd file

**Third argument** - kernel command line options e.g. "console=tty0"

e.g. `sudo ./infrakit x remoteboot ./linuxkit-kernel ./linuxkit-initrd.img "console=tty0"  ... flags`
#### Required Flags:

`--adapter=<X>` This flag is required in order to select the network adapter to bind to.

`--startAddress=<x.x.x.x>` This flag determines the first address that will be advertised by the DCHP server. The `--leaseCount=X` will determine the total amount of leases that will be advertised by the DCHP server, however this defaults to 20.

### Service Flags:

`--addressDHCP=<x.x.x.x>` This specifies the address of the DHCP server will advertise from. If this isn't used, then the IP address will be automatically detected from the interface specified by the `--adapter=<X>` flag.

`--addressTFTP=<x.x.x.x>` This specifies the address of the TFTP server that is advertised by the DHCP server, modify this if you wish to hand off ot another TFTP server.

`--addressHTTP=<x.x.x.x>` This specifies the address of the HTTP server that will host the linuxkit.ipxe *(hardcoded)* in its root directory along with the kernel and initrd.

`--enableDHCP/TFTP/HTTP` These three flags will enable the relevant services upon startup.

`--dns=<x.x.x.x> / --gateway=<x.x.x.x>` These flags will set the relevant network gateway and DNS server through DHCP. If left blank then these will all default to either the `--addressDHCP` or the address of the interface specified through `--address=<X>`.

`--iPXEPath=./<FILENAME>` This is used to specify an alternative PXE bootloader, this defaults to `./undionly.kpxe` 

## Example Usage:

```
$ make build/infrakit

$ sudo build/infrakit x remoteboot pulliPXE
Beginning of iPXE download... Completed

$ sudo build/infrakit x remoteboot ./linuxkit-kernel \
./linuxkit-initrd.img \
"console=tty0" \
--adapter=en0 \
--startAddress=192.168.0.2 \
--leaseCount=50 \
--enableDHCP \
--enableTFTP \
--enableHTTP 

2017/08/07 08:54:02 Binding to en0 / 192.168.0.1
2017/08/07 08:54:02 Starting Remote Boot Services, press CTRL + c to stop
2017/08/07 08:54:02 RemoteBoot => Starting DHCP
2017/08/07 08:54:02 RemoteBoot => Starting TFTP
2017/08/07 08:54:02 Opening and caching undionly.kpxe
2017/08/07 08:54:02 RemoteBoot => Starting HTTP
2017/08/07 08:54:02 Auto generating ./infrakit.ipxe
```

### DHCP
**NOTE**:  As this advertising networking configuration it is advisable to only use on test networks. As a minimum speak to your local friendly networking administrator before advertising networking details on their network. 


## Reporting security issues

The maintainers take security seriously. If you discover a security issue,
please bring it to their attention right away!

Please **DO NOT** file a public issue, instead send your report privately to
[security@docker.com](mailto:security@docker.com).

Security reports are greatly appreciated and we will publicly thank you for it.
We also like to send gifts—if you're into Docker schwag, make sure to let
us know. We currently do not offer a paid security bounty program, but are not
ruling it out in the future.


## Copyright and license

Copyright © 2016 Docker, Inc. All rights reserved, except as follows. Code
is released under the Apache 2.0 license. The README.md file, and files in the
"docs" folder are licensed under the Creative Commons Attribution 4.0
International License under the terms and conditions set forth in the file
"LICENSE.docs". You may obtain a duplicate copy of the same license, titled
CC-BY-SA-4.0, at http://creativecommons.org/licenses/by/4.0/.

## Container Storage Interface (CSI) plugin for Packet

### System configuration

Since we control the initial setup of the hosts, we can make sure they are configured
correctly with isciadm and multipath.  Packet is currently relying on the attach and detach
scripts to rewrite system configuration and restart services, notably iscsiadm and multipath
  * multipath configuration must use-friendly-names
  * iscsiadm initiator name
```
    INAME=$(curl -sSL https://metadata.packet.net/metadata | jq -r .iqn)
    perl -pi -e "s/InitiatorName=.*/InitiatorName=$INAME/" /etc/iscsi/initiatorname.iscsi
    systemctl restart iscsid
```

### References

Packet API:
  *  https://www.packet.net/developers/api/volumes/
  *  https://github.com/packethost/packngo
  *  https://github.com/ebsarr/packet
  *  https://github.com/packethost/packet-block-storage/


packet-flex-volume:
  *  https://github.com/karlbunch/packet-k8s-flexvolume/blob/master/flexvolume/packet/plugin.py#L463

  *  create: https://github.com/karlbunch/packet-k8s-flexvolume/blob/master/flexvolume/packet/plugin.py#L350
  *  attach:
    *  https://github.com/karlbunch/packet-k8s-flexvolume/blob/master/flexvolume/packet/plugin.py#L497
    *  https://github.com/packethost/packet-python/blob/master/packet/Volume.py#L38
  *  iscsi, multipath: https://github.com/karlbunch/packet-k8s-flexvolume/blob/master/flexvolume/packet/plugin.py#L515
  *  mount: https://github.com/karlbunch/packet-k8s-flexvolume/blob/master/flexvolume/packet/plugin.py#L544

iscsi:
 *   https://coreos.com/os/docs/latest/iscsi.html
 *   https://eucalyptus.atlassian.net/wiki/spaces/STOR/pages/84312154/iscsiadm+basics
 *   https://linux.die.net/man/8/multipath
 *   https://access.redhat.com/documentation/en-us/red_hat_enterprise_linux/6/html/dm_multipath/

mount:
  *   https://coreos.com/os/docs/latest/mounting-storage.html
  *   https://oguya.ch/posts/2015-09-01-systemd-mount-partition/
  *  ? https://github.com/coreos/bugs/issues/2254
  *  ? https://github.com/kubernetes/kubernetes/issues/59946#issuecomment-380401916
  *  ? https://github.com/kubernetes/kubernetes/pull/63176

CSI design
  *  https://github.com/container-storage-interface/spec/blob/master/spec.md#rpc-interface

CSI examples
  *  https://github.com/kubernetes-csi/drivers
  *  https://github.com/libopenstorage/openstorage/tree/master/csi
  *  https://github.com/thecodeteam/csi-vsphere

  *  https://github.com/openebs/csi-openebs/
  *  https://github.com/digitalocean/csi-digitalocean

  *  https://github.com/GoogleCloudPlatform/compute-persistent-disk-csi-driver/
  *  https://github.com/GoogleCloudPlatform/compute-persistent-disk-csi-driver/blob/master/deploy/kubernetes/README.md


grpc server

  *    https://github.com/GoogleCloudPlatform/compute-persistent-disk-csi-driver/blob/6702720a9de93b57d73fa8912ef04ce6327a00e3/pkg/gce-csi-driver/server.go
  *  https://github.com/digitalocean/csi-digitalocean/blob/783dcec9b26da4ee9c36b6472e180ebb904c465d/driver/driver.go
  *  https://dev.to/chilladx/how-we-use-grpc-to-build-a-clientserver-system-in-go-1mi


Documentation
  *  https://kubernetes.io/blog/2018/04/10/container-storage-interface-beta/
  *  https://github.com/kubernetes/community/blob/master/contributors/design-proposals/resource-management/device-plugin.md#unix-socket
  *  https://github.com/kubernetes/community/blob/master/contributors/design-proposals/storage/container-storage-interface.md
  *  https://github.com/kubernetes/community/blob/master/sig-storage/volume-plugin-faq.md
  *  https://github.com/kubernetes/community/blob/master/sig-storage/volume-plugin-faq.md#working-with-out-of-tree-volume-plugin-options
  *  https://kubernetes-csi.github.io/docs/Drivers.html
  *  https://kubernetes.io/blog/2018/01/introducing-container-storage-interface/
  *  https://kubernetes.io/docs/concepts/storage/volumes/
  *  https://github.com/container-storage-interface/spec/blob/master/spec.md#rpc-interface

grpc
   * https://grpc.io/docs/quickstart/go.html
   * https://github.com/golang/protobuf
   * https://grpc.io/docs/tutorials/basic/go.html
   * https://developers.google.com/protocol-buffers/docs/proto3


protobuf spec
  *  https://github.com/container-storage-interface/spec


seems interesting ...
  *  https://github.com/kubernetes/kubernetes/issues/59946
  *  https://github.com/kubernetes/kubernetes/issues?utf8=%E2%9C%93&q=is%3Aissue+is%3Aopen+iSCSI

### CSI vs FlexVolumes

FlexVolumes are an older spec, since k8s 1.2, and will be supported in the future.  Requires root access to install on each node and assumes OS-based tools are installed. "The Storage SIG suggests implementing a CSI driver if possible"

### CSI design summary

Kubernetes will introduce a new in-tree volume plugin called CSI.

This plugin, in kubelet, will make volume mount and unmount rpcs to a unix domain socket on the host machine. The driver component responds to these requests in a specialized way. (https://github.com/kubernetes/community/blob/master/contributors/design-proposals/storage/container-storage-interface.md#kubelet-to-csi-driver-communication)


Lifecycle management of volume is done by the controller-manager (https://github.com/kubernetes/community/blob/master/contributors/design-proposals/storage/container-storage-interface.md#master-to-csi-driver-communication) and communication is mediated through the api-server, which requires that the external component watch the k8s api for changes. The design document suggests a sidecar “Kubernetes to CSI” proxy.

The concern here is that
  - communication to the diver is done through a local unix domain socket
  - the driver is untrusted and cannot be allowed to run on the master node
  - the controller manager runs on the master node
  - the driver doesn't have any kubernetes-awareness, doesn't have k8s client code or how to watch the api serer

This section: https://github.com/kubernetes/community/blob/master/contributors/design-proposals/storage/container-storage-interface.md#recommended-mechanism-for-deploying-csi-drivers-on-kubernetes
shows the recommended deployment which puts the driver in a container inside a pod, sharing it with a k8s-aware container, with communication between those two via a unix domain socket "in the pod"


"The in-tree volume plugin’s SetUp and TearDown methods will trigger the NodePublishVolume and NodeUnpublishVolume CSI calls via Unix Domain Socket. "

Sequence:
 - Create          CreateVolume
    - Attach       ControllerPublish
        - Mount, Format    NodeStageVolume (called once only per volume)
            - Bind Mount NodePublishVolume
            - Bind Unmount NodeUnpublishVolume
        - Unmount  NodeUnstageVolume
    - Detach       ControllerUnpublish
 - Destroy         DeleteVolume


### Credentials

https://github.com/kubernetes/community/blob/master/contributors/design-proposals/storage/container-storage-interface.md#csi-credentials


### Deployment Helpers

    * external-attacher https://github.com/kubernetes-csi/external-attacher
    * external-provisioner https://github.com/kubernetes-csi/external-provisioner
    * driver-registrar https://github.com/kubernetes-csi/driver-registrar
    * liveness-probe https://github.com/kubernetes-csi/livenessprobe

### Mounting

Mounting a filesystem is an os task, not cloud provider.

DO, GCE create a mounter type
 *   https://github.com/digitalocean/csi-digitalocean/blob/master/driver/mounter.go
 *   https://github.com/GoogleCloudPlatform/compute-persistent-disk-csi-driver/blob/master/pkg/mount-manager/mounter.go
VSphere calls out to a separate library
 *   https://github.com/akutz/gofsutil
why not use [sys](https://godoc.org/golang.org/x/sys/unix#Mount)? Well, it seems we need to exec out in order to call mkfs anyway

  * https://github.com/thecodeteam/csi-vsphere/blob/master/service/node.go


on our coreos installs,
 *   iscsid
 *   multipathd
are present but not running


###

grpc server exposes the Identity, Controller and Node

packet client interacts with packet provisioner, must receive credentials
( might be inside of CreateVolumeRequest.ControllerCreateSecrets, or might be part of the inifital client configuration as in https://github.com/digitalocean/csi-digitalocean/blob/master/driver/driver.go#L58 )


mounter is embeded in Node, mounts and formats as necessary


### Deployment

GCE for example, deploys the node server as a daemonset, running
```
          image: gcr.io/dyzz-csi-staging/csi/gce-driver:latest
          args:
            - "--v=5"
            - "--endpoint=$(CSI_ENDPOINT)"
            - "--nodeid=$(KUBE_NODE_NAME)"
```
alongside
```
    - csi-driver-registrar (https://github.com/kubernetes-csi/driver-registrar)
```
The controller server is deployed as a single-replica stateful set
```
          image: gcr.io/dyzz-csi-staging/csi/gce-driver:latest
          args:
            - "--v=5"
            - "--endpoint=$(CSI_ENDPOINT)"
            - "--nodeid=$(KUBE_NODE_NAME)"
```
alongside
  * csi-attacher (watches volumeAttach api events https://github.com/kubernetes-csi/external-attacher)
  * csi-external-provisioner (watches PersistentVolumeClaim events, https://github.com/kubernetes-csi/external-provisioner)
containers

So the driver itself is exactly the same in the two configurations.  In the case of the node, it is communicating over grpc with kubelet. The controller is communicating over grpc with the sidecar containers.


kubelet configuration:
   for alpha, kubelet expects a domain socket at a known location, /var/lib/kubelet/plugins/[SanitizedCSIDriverName]/csi.sock
where SanitizedCSIDriverName ... is the name or type specified in a particular PeristentVolumeClaim(?)  For beta, there's a process for publishing the domain socket location



#### Weird node deployment

So

Right

Okay

Under ordinary circumstances, we'd have a shell script running on the host and calling various commands to mount or format this and that.  The first oddity is that all of this is implemented in Go.  (It doesn't have to be, but for consistencies sake it seems straigthforward)  So the sheel commands of `mkfs.ext4` and whatnot are wrapped up in os/exec.Command.  This is pretty common, some people apparently like to script in Go for lots of sysadminy things.

Recall, though, how this is deployed.  It's a container in a pod in a daemonset on the worker nodes.  "Container" means that the execution environment is not the node itself, but the restricted container sandbox. So when Go reaches out to os/exec, it's getting the sandbox, not the node.  There's another level of indirection before we can reach the things that iscsiadmin and multipath must manipulate.

##### iscisadmin in container

There are images with iscsiadmin packaged.  Running them with

```
core@spck8q2o0h-worker-1 ~ $ ls
core@spck8q2o0h-worker-1 ~ $ docker run --rm -it tripleomaster/centos-binary-iscsid:current-tripleo bash

()[root@f304b64502ac /]# iscsiadm --mode discovery --type sendtargets --portal $portal --discover
Failed to get D-Bus connection: Operation not permitted
iscsiadm: can not connect to iSCSI daemon (111)!
Failed to get D-Bus connection: Operation not permitted
iscsiadm: can not connect to iSCSI daemon (111)!
iscsiadm: Cannot perform discovery. Initiatorname required.
iscsiadm: Could not perform SendTargets discovery: could not connect to iscsid
```
so that container doesn't have sufficient permissions

better:
->  docker run  --privileged --rm -v /etc/iscsi/:/etc/iscsi/  -v /var/lib/iscsi/:/var/lib/iscsi/ -it tripleomaster/centos-binary-iscsid:current-tripleo bash

but login still fails
https://bugs.launchpad.net/ubuntu/+source/lxc/+bug/1226855


it's about the network namespace, so net=host fixes it:
```
 $  docker run --net host  --privileged --rm -v /etc/iscsi/:/etc/iscsi/  -v /var/lib/iscsi/:/var/lib/iscsi/ -it tripleomaster/centos-binary-iscsid:current-tripleo bash
()[root@spck8q2o0h-worker-1 /]# iscsiadm --mode discovery --type sendtargets --portal 10.144.144.226 --discover
10.144.144.226:3260,1 iqn.2013-05.com.daterainc:tc:01:sn:25c3e33abe64b2ba
10.144.145.219:3260,1 iqn.2013-05.com.daterainc:tc:01:sn:25c3e33abe64b2ba
()[root@spck8q2o0h-worker-1 /]# iscsiadm --mode node --targetname iqn.2013-05.com.daterainc:tc:01:sn:25c3e33abe64b2ba  --portal 10.144.144.226  --login
Logging in to [iface: default, target: iqn.2013-05.com.daterainc:tc:01:sn:25c3e33abe64b2ba, portal: 10.144.144.226,3260] (multiple)
Login to [iface: default, target: iqn.2013-05.com.daterainc:tc:01:sn:25c3e33abe64b2ba, portal: 10.144.144.226,3260] successful.
```

##### multipath in container
```
docker run --rm -ti \
  --net=host --privileged \
  -v /etc/multipath/:/etc/multipath/ \
  -v /etc/multipath.conf:/etc/multipath.conf \
  -v /dev:/dev \
   tripleoupstream/centos-binary-multipathd bash

```

* https://docs.racket-lang.org/multipath-daemon/index.html
* https://github.com/moby/moby/issues/14767#issuecomment-269481738

daemon and client are communicating over an abstract socket, @/org/kernel/linux/storage/multipathd
this value is fixed, not configurable.

client inside docker container has trouble with this! even with --net=host.

```
()[root@spck8q2o0h-worker-1 /]# lsof -U
()[root@spck8q2o0h-worker-1 /]#
```


##### script in container

in an ubuntu-based cluster:
```
docker run --net=host -it --privileged --rm \
  --hostname scratch \
  -v /etc/iscsi/:/etc/iscsi/  \
  -v /var/lib/iscsi/:/var/lib/iscsi/ \
  -v /etc/multipath/:/etc/multipath/ \
  -v /etc/multipath.conf:/etc/multipath.conf \
  -v /sys/devices:/sys/devices \
  -v /etc/udev:/etc/udev \
  -v /run/udev:/run/udev \
  -v /dev:/dev \
  -v /usr/bin/:/opt/host-bin:ro \
  ntfrnzn/scratch \
  bash


apt-get install -y strace

#  -v /sys/devices:/sys/devices \

strace -f -e trace=open /opt/host-bin/packet-block-storage-attach -v
```

so, sometimes multipath hangs, sometimes not.  It is _not_ waiting on a
file.  It hangs when invoking -f on a volume to remove, and strace says:
```
strace multipath -f volume-f15126bc
```

```
semget(0xd4d8821, 1, IPC_CREAT|IPC_EXCL|0600) = 131076
semctl(131076, 0, SETVAL, 0x1)          = 0
semctl(131076, 0, GETVAL, 0x7f0da7627caa) = 1
close(4)                                = 0
semop(131076, [{0, 1, 0}], 1)           = 0
semctl(131076, 0, GETVAL, 0x7f0da7627c47) = 2
ioctl(3, DM_DEV_REMOVE, 0x555e95c3d490) = 0
semget(0xd4d8821, 1, 0)                 = 131076
semctl(131076, 0, GETVAL, 0x800)        = 2
semop(131076, [{0, -1, IPC_NOWAIT}], 1) = 0
semop(131076, [{0, 0, 0}], 1
```

on the host itself it just completes properly,
```
close(4)                                = 0
semop(851969, [{0, 1, 0}], 1)           = 0
semctl(851969, 0, GETVAL, 0x7f63b3b0dc47) = 2
ioctl(3, DM_DEV_REMOVE, 0x55634f8ea490) = 0
semget(0xd4de42b, 1, 0)                 = 851969
semctl(851969, 0, GETVAL, 0x800)        = 1
semop(851969, [{0, -1, IPC_NOWAIT}], 1) = 0
semop(851969, [{0, 0, 0}], 1)           = 0
semctl(851969, 0, IPC_RMID, 0)          = 0
lstat("/dev/mapper/volume-f15126bc", 0x7ffe1f241f70) = -1 ENOENT (No such file or directory)
close(3)                                = 0
munmap(0x7f63b1f2a000, 2101264)         = 0
munmap(0x7f63b232e000, 2105360)         = 0
munmap(0x7f63b212c000, 2101296)         = 0
exit_group(0)                           = ?
+++ exited with 0 +++
```
#####
```
# FROM ubuntu:18.04
# RUN apt-get update
# RUN apt-get install -y wget multipath-tools open-iscsi curl jq
```
```
# /opt/host-bin/packet-block-storage-attach
portal: 10.144.144.227 iqn: iqn.2013-05.com.daterainc:tc:01:sn:1e9c06a93aad6c8f
Error: We couldn't log in iqn iqn.2013-05.com.daterainc:tc:01:sn:1e9c06a93aad6c8f
portal: 10.144.145.188 iqn: iqn.2013-05.com.daterainc:tc:01:sn:1e9c06a93aad6c8f
Error: We couldn't log in iqn iqn.2013-05.com.daterainc:tc:01:sn:1e9c06a93aad6c8f
Error: Block device /dev/mapper/volume-f15126bc is NOT available for use

# iscsiadm --mode discovery --type sendtargets --portal 10.144.144.226 --discover
iscsiadm: No portals found
```


:(



---
inside the container, running multipath -v 3:
```
May 17 17:24:53 | sda: blacklisted, udev property missing
May 17 17:24:53 | sdb: blacklisted, udev property missing
May 17 17:24:53 | sdc: blacklisted, udev property missing
May 17 17:24:53 | sdd: blacklisted, udev property missing

but outside on the host,
May 17 17:19:26 | sda: udev property ID_WWN whitelisted
May 17 17:19:26 | sda: not found in pathvec
May 17 17:19:26 | sda: mask = 0x3f
May 17 17:19:26 | sda: dev_t = 8:0
...
May 17 17:19:26 | sdb: udev property ID_WWN whitelisted
May 17 17:19:26 | sdb: not found in pathvec
May 17 17:19:26 | sdb: mask = 0x3f
May 17 17:19:26 | sdb: dev_t = 8:16
...
May 17 17:19:26 | sdc: udev property ID_WWN whitelisted
May 17 17:19:26 | sdc: not found in pathvec
May 17 17:19:26 | sdc: mask = 0x3f
May 17 17:19:26 | sdc: dev_t = 8:32
May 17 17:19:26 | sdc: size = 209715200
May 17 17:19:26 | sdc: vendor = DATERA
May 17 17:19:26 | sdc: product = IBLOCK


You can see the difference with
```
    udevadm info --query=all --name=/dev/sdc
```
where on the host we get all the properties (more than 40) including ID_WWN and SCSI_IDENT_* but in the containeronly 9 are shown.

https://stackoverflow.com/questions/41753218/udevadm-does-not-show-all-attributes-inside-a-docker-container


### Packet attach scripts == NodeStageVolume

    * ensure that iscsid and multipathd are
        * installed
        * properly configured
        * restarted
        * I mean, really properly configured
        * restarted
    * pull the volume metadata
    * iscsiadmin discover + login creates the device locally /dev/sd[X] (block-device-name=sdc)
    * find the id with /lib/udev/scsi_id -g -u -d /dev/sd[X]
    * configure /etc/multipath/bindings with volumename and scsid id
    * call multipath to create it.

problem:
   script expects to find "/dev/mapper/$volname" but  "/dev/mapper/$wwid" is present and links to /dev/dm-2, not /dev/

script will remove the "mpatha SAMSUNG_MZ7KM240HMHQ-00005_S3F5NY0J401892" entry in /etc/multipath/bindings, is this necessary? idk, it gets recreated anyway
Make sure "user_friendly_names     yes" in the config
now it works ...
```
for p in $portals; do iscsiadm --mode discovery --type sendtargets --portal $p --discover; done
for p in $portals; do iscsiadm --mode node --targetname $iqn --portal $p --login; done

volname=$(curl -sSL https://metadata.packet.net/metadata | jq -r .volumes[0].name)

for p in $portals;
do
    bdname=`ls -l /dev/disk/by-path/ | grep "$iqn" | grep $p | awk {'print $11'} | sed 's/..\/..\///'`
    wwid=`/lib/udev/scsi_id -g -u -d /dev/$bdname`
    # echo "$volname $wwid"
    echo "$volname $wwid" >> /etc/multipath/bindings
done

sed -i "/^mpath.*/d" /etc/multipath/bindings
multipath -v 2 $volname  # and Ctrl-c :(
multipath -ll
```

### format and mount

OK, nice comment here: https://github.com/karlbunch/packet-k8s-flexvolume/blob/master/flexvolume/packet/plugin.py#L519 "sometimes it ends up on /dev/mapper/volumeName others /dev/mapper/WWID"  Tell me about it.

flex volume plugin supports only zfs

```
mkfs.ext4 -F /dev/mapper/volume-4ea47168

Creating filesystem with 26214400 4k blocks and 6553600 inodes
Filesystem UUID: 7c3a1d11-596f-4694-8dca-554e141bd60d
Superblock backups stored on blocks:
	32768, 98304, 163840, 229376, 294912, 819200, 884736, 1605632, 2654208,
	4096000, 7962624, 11239424, 20480000, 23887872

Allocating group tables: done
Writing inode tables: done
Creating journal (131072 blocks): done
Writing superblocks and filesystem accounting information: done
```

```
mkdir /mnt/test-vol-4ea47168
mount -t ext4 /dev/mapper/volume-4ea47168  /mnt/test-vol-4ea47168  # other options?
```

### Unmount

```
umount  /mnt/test-vol-4ea47168
```

### Unstage

https://github.com/packethost/packet-block-storage/blob/master/packet-block-storage-detach

again this?
```
egrep -v "^#" /etc/multipath/bindings | wc -l
3
```

cleanup mpath entries in bindings file
```
sed -i "/^mpath.*/d" /etc/multipath/bindings
if [ `ls /dev/mapper/mpath* >/dev/null 2>/dev/null` ]; then
  [ $_V -eq 1 ] && echo "mpath volume entries found, cleaning up"
	for i in `ls /dev/mapper/mpath* | cut -d / -f4`; do
		multipath -f $i
	done
fi
```
```
egrep -v "^#" /etc/multipath/bindings | wc -l
1
```

```
iqn=iqn.2013-05.com.daterainc:tc:01:sn:4a69ef2d9075d89e
volname=volume-4ea47168
```

```
# iscsiadm -m session
tcp: [1] 10.144.144.226:3260,1 iqn.2013-05.com.daterainc:tc:01:sn:4a69ef2d9075d89e
```
gives portal + iqn.  If the iqn matches, then

1. log out of it
```

for p in $portals;
do
 iscsiadm --mode node --targetname $iqn --portal $p --logout
done

Logging out of session [sid: 1, target: iqn.2013-05.com.daterainc:tc:01:sn:4a69ef2d9075d89e, portal: 10.144.144.226,3260]
Logout of [sid: 1, target: iqn.2013-05.com.daterainc:tc:01:sn:4a69ef2d9075d89e, portal: 10.144.144.226,3260] successful.```
2. remove it from multipath bindings and update multipath
```
sed -i "/^$volname.*/d" /etc/multipath/bindings
multipath -f $volname
```


If it's still there on /dev/sdc, then something didn't work.  Try logging out of iscsiadmin again
```
Disk /dev/sdc: 100 GiB, 107374182400 bytes, 209715200 sectors
Units: sectors of 1 * 512 = 512 bytes
Sector size (logical/physical): 512 bytes / 512 bytes
I/O size (minimum/optimal): 512 bytes / 1048576 bytes
```

should no longer have the iqn in this list
 ls -al /dev/disk/by-path/


### ControllerDetach

remotely,

```
$ curl -s -X DELETE -H "X-Auth-Token: $PACKET_TOKEN" https://api.packet.net/storage/attachments/$ATTACH_ID
```

should be silent, if it answers
```
{"errors":["Cannot detach since volume is actively being used on your server"]}
```
then icssiadmin has not logged out correctly

### ControllerDeleteVolume

```
# curl -s -H "X-Auth-Token: $PACKET_TOKEN" https://api.packet.net/projects/$PROJECT_ID/storage


curl -X DELETE -s -H "X-Auth-Token: $PACKET_TOKEN" https://api.packet.net/storage/$VOLUME_ID
```
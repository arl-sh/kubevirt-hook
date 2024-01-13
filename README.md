# KubeVirt USB device passthrough

> ⚠️ This repository is deprecated and unsupported. An [official solution](https://kubevirt.io/user-guide/virtual_machines/host-devices/#usb-host-passthrough) has been introduced in v1.1.

This repository is the source for a Docker container intended to run as a [KubeVirt hook sidecar](https://github.com/kubevirt/kubevirt/tree/main/cmd/sidecars).\
The built image is available on GitHub's Container Registry: `ghcr.io/au2001-homelab/kubevirt-usbdevice-hook`

## Requirements

This hook requires the following changes to the official KubeVirt repository:
- `qemu-kvm-device-usb-host` must be installed in the launcher image, and
- `/dev/bus/usb` must be mounted inside the VM's launcher container.

Theses changes have been made to the official KubeVirt repository on [my fork](https://github.com/au2001-homelab/kubevirt/tree/usbhost-0.59).\
The built images are available on [Docker Hub](https://hub.docker.com/r/au2001/virt-operator).

These images can be applied by patching KubeVirt's operator deployment, either manually or with Kustomize.\
The image of the operator pods needs to be changed, as well as the [according environment variables](https://github.com/kubevirt/kubevirt/blob/release-0.59/pkg/virt-operator/util/config.go#L44-L49).

## Usage

You must apply the following annotation to each VirtualMachine you wish to use USB passthrough in:
```yaml
hooks.kubevirt.io/hookSidecars: '[{"image": "ghcr.io/au2001-homelab/kubevirt-usbdevice-hook"}]'
```

Next, you need to add one annotation per USB device you wish to passthrough, following this template:
```yaml
usbdevice.vm.kubevirt.io/<name>: <vendorId>:<deviceId>
```

The `<name>` placeholder can be any unique value, and is only used for identification.\
The `<vendorId>` and `<deviceId>` placeholders identify the USB device and can be found with [`lsusb`](https://linux.die.net/man/8/lsusb).

## Caveats

The USB devices are only attached to the VM at start time.\
They are all made optional, which means the VM will successfully start even if some devices are not plugged in.\
If a device is unplugged while the VM is running, the VM will not stop. It will instead see the device as unplugged.

**If a device is plugged in while the VM is running, the VM will NOT be able to access it, even if it was plugged in before.**\
**In this case, the VM must be restarted.**

# Dell

TODO: Use gather_facts: false, when possible

Install AlmaLinux 9 from the minimal ISO. Set its MAC in DHCP server with a static IP. Create an `admin` user and allow it to become root.

```sh
ssh-copy-id admin@192.168.122.125
```

Deploy Kubernetes using Ansible:

```sh
sudo dnf install ansible linux-system-roles ansible-collection-kubernetes-core python3-kubernetes
cd ansible
ansible-playbook -i inventory.file -u admin -K create_ansible_user.yml
ansible-playbook -i inventory.file -u ansible update_dnf_packages.yml
ansible-playbook -i inventory.file -u ansible install_dnf_automatic.yml
ansible-playbook -i inventory.file -u ansible configure_network.yml
ansible-playbook -i inventory.file -u ansible install_k3s.yml
ansible-playbook -i inventory.file -u ansible install_argocd.yml
kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d
kubectl port-forward service/argocd-server 8090:80 -n argocd
# Log in and set a new password
kubectl -n argocd delete secret argocd-initial-admin-secret
```

Deploy applications using ArgoCD:

```sh
cd argocd
kubectl apply -f multus-application.yaml
kubectl apply -f kubevirt-application.yaml
kubectl apply -f cdi-application.yaml
kubectl apply -f metallb-application.yaml
kubectl apply -f homeassistant-application.yaml
# Enable File editor addon, show it in side panel, open configuration.yaml, add:
# http:
#   server_port: 80
# Change IP configuration to static and give it IP 192.168.0.9/24 (until VLAN etc. is set up)
kubectl apply -f argocd-application.yaml
# TODO: File server for ISOs and stuff like that, so it does not have to be downloaded every time
```

Test LoadBalacer connectivity:

```sh
kubectl create deployment hello-server --image=gcr.io/google-samples/hello-app:1.0
kubectl expose deployment hello-server --type LoadBalancer --port 80 --target-port 8080
kubectl get all
# Test connectivity
```

Edit `/etc/hosts`:

```
192.168.0.9 assistant.local
192.168.0.9 argocd.local
```

## Cheatsheet

```
cat <<EOF | kubectl create -f -
apiVersion: cdi.kubevirt.io/v1beta1
kind: DataVolume
metadata:
  name: almalinux
spec:
  source:
      http:
         url: "https://repo.almalinux.org/almalinux/9.4/isos/x86_64/AlmaLinux-9.4-x86_64-minimal.iso"
  storage:
    volumeMode: Filesystem
    resources:
      requests:
        storage: "3G"
EOF
```

```
cat <<EOF | kubectl create -f -
apiVersion: cdi.kubevirt.io/v1beta1
kind: DataVolume
metadata:
  name: disk
spec:
  source:
    blank: {}
  storage:
    resources:
      requests:
        storage: 20Gi
EOF
```

```
cat <<EOF | kubectl apply -f -
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  labels:
    kubevirt.io/vm: desktop
  name: desktop
spec:
  runStrategy: RerunOnFailure
  template:
    metadata:
      labels:
        kubevirt.io/vm: desktop
    spec:
      domain:
        devices:
          disks:
          - disk:
              bus: virtio
            name: disk1
          - disk:
              bus: virtio
            name: disk2
            bootOrder: 1
        resources:
          requests:
            memory: 2G
      volumes:
      - name: disk1
        dataVolume:
          name: disk
      - name: disk2
        dataVolume:
          name: almalinux
EOF
```

## Home assistant

* Install ESPHome addon.

### BLE to WiFi proxy

1. Flash initial STM32 over USB.
  * Connect it using data cable.
  * `sudo chmod a+rw /dev/ttyUSB0` (TODO: Udev rule)
  * <https://web.esphome.io/?dashboard_wizard> (TODO: HTTPs for HA)
  * Connect and flash.
2. Find it in ESPHome and claim it.
3. Add this to the end of the config:
  ```
esp32_ble_tracker:
scan_parameters:
  interval: 1100ms
  window: 1100ms
    
bluetooth_proxy:
  ```
4. Save it and install it.
5. Enable BTHome from HA Devices menu.

I have issues with my ESP32 dev kit - the Bluetooth reception is very low.

### Flash temperature and humidity sensor - BLE

Using <https://github.com/pvvx/ATC_MiThermometer>. Flashed from Chrome running on Windows - Android and Fedora struggle with Bluetooth.

1. Get Mi Temperature and Humidity Monitor 2.
2. Go to <https://pvvx.github.io/ATC_MiThermometer/TelinkMiFlasher.html>.
3. Tick Get advertising MAC.
4. Filter LYWSD03.
5. Connect.
6. Do Activation.
7. Copy the Token and Bind Key and save them in a safe location.
8. Flash Custom Firmware ATC_v48.bin.
9. Start Flashing.
10. Filter ATC.
11. Connect again.
12. Name it MI<INDEX>.
13. Set a PIN, write it on a piece of paper and place it under the sensor cover.
14. Disconnect.

### Flash temperature and humidity sensor - Zigbee

My BLE proxy has very weak signal reception. Try converting one of the sensors to the experimental Zigbee protocol, for which I have a proper receiver.

Using <https://github.com/pvvx/ATC_MiThermometer>. Flashed from Chrome running on Windows - Android and Fedora struggle with Bluetooth.

1. Get Mi Temperature and Humidity Monitor 2.
2. Go to <https://pvvx.github.io/ATC_MiThermometer/TelinkMiFlasher.html>.
5. Connect.
6. Do Activation.
7. Copy the Token and Bind Key and save them in a safe location.
8. Flash Custom Firmware for Zigbee.
9. Start Flashing.
10. Remove and put back the battery.
11. Bridge between the two pins next to the battery for 10 seconds to enter pairing mode.
12. Register the device in HA's Zigbee administration.

### Add temporature and humidity sensor to HA

1. Go to Devices.
2. New BTHome sensor devices should be listed.
3. Add them.

### Create a graph of humidity and temperature

1. Create a new dashboard.
2. Add a Sensor card.
3. Select Humidity or Temperature from the sensor.

### Downgrade firmware of Zigbee dongle

Use <https://darkxst.github.io/silabs-firmware-builder/> to flash firmware from <https://github.com/itead/Sonoff_Zigbee_Dongle_Firmware/tree/master/Dongle-E/NCP>.

Note: I'm not sure this was necessary.

### Switch to Z2M

1. Disconnect all devices from ZHA.
2. Install the Z2M addon <https://github.com/zigbee2mqtt/hassio-zigbee2mqtt#installation>.
3. Configure the dongle.

Serial:

```
port: >-
  /dev/serial/by-id/usb-ITEAD_SONOFF_Zigbee_3.0_USB_Dongle_Plus_V2_20240123101015-if00
```

### Install HACS

<https://hacs.xyz/docs/use/download/download/#to-download-hacs>

1. Install the addon.
2. Run it.
3. Restart HA.
4. Add HACS device.

### Install Better Thermostat

<https://better-thermostat.org/>

1. Install it from HACS.
2. Install the UI too.
3. Restart HA.

# OPNSense

```
kubectl create namespace opnsense
kubectl config set-context --current --namespace=opnsense
```

```
cat <<EOF | kubectl create -f -
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: wan
spec:
  config: |
    {
      "cniVersion": "0.3.1",
      "name": "wan", 
      "type": "bridge", 
      "bridge": "br0", 
      "macspoofchk": true, 
      "disableContainerInterface": true
    }
EOF
```

```
bzip2 -d OPNsense-24.7-nano-amd64.img.bz2
virtctl image-upload dv opnsense --size=20Gi --image-path=OPNsense-24.7-nano-amd64.img --uploadproxy-url=https://127.0.0.1:8443 --force-bind --insecure
```

```
cat <<EOF | kubectl apply -f -
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  labels:
    kubevirt.io/vm: opnsense
  name: opnsense
spec:
  runStrategy: RerunOnFailure
  template:
    metadata:
      labels:
        kubevirt.io/vm: opnsense
    spec:
      domain:
        devices:
          disks:
          - disk:
              bus: virtio
            name: disk1
          interfaces:
          - name: lan
            masquerade: {}
          - name: wan
            bridge: {}
        resources:
          requests:
            memory: 1G
      networks:
      - name: lan
        pod: {}
      - name: wan
        multus:
          networkName: wan
      volumes:
      - name: disk1
        dataVolume:
          name: opnsense
EOF
```

## TODO

TODO:
- update docs to open firewall for all
- Disable ServiceLB and Klipper
- Deploy Kuberentes Dashboard
- Replace MetalLB Services with an Ingress where possible
- Set static MAC for HA IOT interface and assign it with an IP
- Put HA IOT interface on a VLAN
- Expose HA running on IOT VLAN to MAIN VLAN over router
- Give printer a static IP and allow IOT network's HA to access it

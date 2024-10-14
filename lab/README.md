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
```

Deploy applications using ArgoCD:

```sh
cd argocd
kubectl apply -f multus-application.yaml
kubectl apply -f kubevirt-application.yaml
kubectl apply -f cdi-application.yaml
kubectl apply -f metallb-application.yaml
kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d
kubectl port-forward service/argocd-server 8090:80 -n argocd
# Sync the applications and set a password
```

Test LoadBalacer connectivity:

```sh
kubectl create deployment hello-server --image=gcr.io/google-samples/hello-app:1.0
kubectl expose deployment hello-server --type LoadBalancer --port 80 --target-port 8080
kubectl get all
# Test connectivity
```

```sh
curl -L -o /tmp/ha.qcow2.xz https://github.com/home-assistant/operating-system/releases/download/13.1/haos_ova-13.1.qcow2.xz
xz -d /tmp/ha.qcow2.xz

kubectl edit storageprofile local-path
spec:
  claimPropertySets: 
  - accessModes:
    - ReadWriteOnce
    volumeMode: 
      Filesystem


sudo dnf install usbutils
lsusb
# note 1a86:55d4 of Zigbee dongle

kubectl edit -n kubevirt kubevirt kubevirt

spec:
  configuration:
    developerConfiguration: 
      featureGates:
        - HostDevices
    permittedHostDevices:
      usb:
        - resourceName: kubevirt.io/zigbee
          selectors:
            - vendor: "1a86"
              product: "55d4"

cat <<EOF | kubectl create -f -
apiVersion: cdi.kubevirt.io/v1beta1
kind: DataVolume
metadata:
  name: "ha-dv"
spec:
  source:
      http:
         url: "https://github.com/home-assistant/operating-system/releases/download/13.1/haos_ova-13.1.qcow2.xz"
  storage:
    volumeMode: Filesystem
    resources:
      requests:
        storage: "40G"
EOF

#TODO: https://kubevirt.io/user-guide/storage/disks_and_volumes/#datavolume-vm-behavior keep DV in template
cat <<EOF | kubectl apply -f -
---
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  labels:
    kubevirt.io/vm: homeassistant
  name: homeassistant
spec:
  runStrategy: RerunOnFailure
  template:
    metadata:
      labels:
        kubevirt.io/vm: homeassistant
    spec:
      domain:
        devices:
          disks:
          - disk:
              bus: virtio
            name: disk1
          hostDevices:
          - deviceName: kubevirt.io/zigbee
            name: zigbee-dongle
        firmware:
          bootloader:
            efi:
              secureBoot: false
        resources:
          requests:
            memory: 2000M
      volumes:
      - name: disk1
        dataVolume:
          name: ha-dv
---
apiVersion: v1
kind: Service
metadata:
  name: homeassistant
spec:
  loadBalancerIP: 192.168.0.9
  ports:
  - port: 80
    protocol: TCP
    targetPort: 8123
  selector:
    kubevirt.io/vm: homeassistant
  type: LoadBalancer
EOF
```

kubectl expose pod virt-launcher-vmi-alpine-datavolume-dq9zh --type LoadBalancer --port 80 --target-port 8123
http://192.168.0.4/onboarding.html

TODO:
- pass through zigbee
- update docs to open firewall for all
- add yaml for service

TODO:
- NAD
- Use Tekton to decompress Home Assistant
- Bridge binding plugin.
- Disable ServiceLB and Klipper
- Deploy Kuberentes Dashboard

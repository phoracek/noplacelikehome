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

TODO:
- update docs to open firewall for all

TODO:
- Disable ServiceLB and Klipper
- Deploy Kuberentes Dashboard
- Replace MetalLB Services with an Ingress where possible
- Set static MAC for HA IOT interface and assign it with an IP
- Put HA IOT interface on a VLAN
- Expose HA running on IOT VLAN to MAIN VLAN over router

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
kubectl apply -f homeassistant-application.yaml
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

TODO:
- update docs to open firewall for all
- add yaml for service

TODO:
- NAD
- Use Tekton to decompress Home Assistant
- Bridge binding plugin.
- Disable ServiceLB and Klipper
- Deploy Kuberentes Dashboard

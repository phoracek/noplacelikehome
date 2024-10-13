# Dell

TODO: Use gather_facts: false, when possible

Install AlmaLinux 9 from the minimal ISO. Set its MAC in DHCP server with a static IP. Create an `admin` user and allow it to become root.

```sh
ssh-copy-id admin@192.168.122.125
```

```sh
sudo dnf install ansible linux-system-roles ansible-collection-kubernetes-core python3-kubernetes
ansible-playbook -i inventory.file -u admin -K create_ansible_user.yml
ansible-playbook -i inventory.file -u ansible update_dnf_packages.yml
ansible-playbook -i inventory.file -u ansible install_dnf_automatic.yml
ansible-playbook -i inventory.file -u ansible configure_network.yml
ansible-playbook -i inventory.file -u ansible install_k3s.yml
ansible-playbook -i inventory.file -u ansible install_argocd.yml
```

```sh
kubectl apply -f argocd/multus-application.yaml
kubectl apply -f argocd/kubevirt-application.yaml
kubectl apply -f argocd/cdi-application.yaml
kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d
kubectl port-forward service/argocd-server 8090:80 -n argocd
# Sync the applications and set a password
```

TODO:
- NAD
- ArgoCD
- MetalLB
- Bridge binding plugin.

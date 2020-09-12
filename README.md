# noplacelikehome

Bunch of dotfiles.

# Workstation

Install the OS first. Use Fedora 32 Server netinstall. Pick the minimal
installation package.

Get all the secrets (ssh keys, key chains, ...) on the host.

Install initial dependencies, initialize the most important directory, fetch
sources of this project and start the deployment:

```shell
sudo dnf install -y git ansible

mkdir code
git clone git@github.com:phoracek/noplacelikehome.git ~/code/noplacelikehome

cd ~/code/noplacelikehome
ansible-playbook play.yml --ask-become-pass --extra-vars '{"workstation": true}'
```

# Server

Install additional tooling on whatever server you use:

```shell
sudo dnf install -y git ansible

git clone https://github.com/phoracek/noplacelikehome

cd ~/noplacelikehome
ansible-playbook play.yml --ask-become-pass
```

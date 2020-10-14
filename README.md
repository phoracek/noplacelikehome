# noplacelikehome

Bunch of dotfiles. Tested on ThinkPad T470s and T490s.

# Workstation

Install the OS first. Use Fedora 33 Server netinstall. Pick the minimal
installation package with additional "Common NetworkManager Submodules" to help
with the initial WiFi configuration. Set the hostname, login to WiFi. Set
partitioning so everything is used for the root directory and enable encryption.
Create a new user and make them an administrator. Add all the needed keyboard
layouts. Set the timezone. Begin installation.

It is not possible to install WiFi drivers during the installation of minimal
Fedora. Therefore, after the system is installed, connect the laptop to the
internet using a cable and finally install needed drivers:

``` shell
sudo nmcli c up ETHERNET_CONNECTION
sudo dnf install -y iwl7260-firmware
sudo reboot
```

Get all the secrets (ssh keys, key chains, ...) on the host. Here's how you
mount USB drive via command line and copy secrets from there:

``` shell
# Find the right drive
sudo fdisk -l

# Create a mound point
mkdir /tmp/usb_drive

# Mount the drive
sudo mount /dev/sda1 /tmp/usb_drive

# Copy keys
cp -r /tmp/usb_drive/.ssh .

# Set correct permissions
chmod -R 0700 ~/.ssh

# Unmount the drive
sudo umount /dev/sda1
```

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

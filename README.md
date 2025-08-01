# noplacelikehome

Bunch of dotfiles. Tested on ThinkPad T470s and T490s, Fedora 40 i3 spin.

``` shell
# Packages
sudo dnf install -y git alacritty lxpolkit zsh xkill keepassxc
sudo dnf remove -y network-manager-applet volumeicon

# Docking station (Lenovo Thunderbolt III gen 2)
sudo dnf install -y bolt
boltctl list
boltctl enroll <domain>

# Git
git config --global user.name "Your Name"
git config --global user.email "youremail@yourdomain.com"

# Automatic upgrades
sudo dnf install -y dnf-automatic
sudo sed -i 's/^apply_updates *= *no/apply_updates = yes/' /etc/dnf/automatic.conf
sudo systemctl enable dnf-automatic.timer
sudo systemctl start dnf-automatic.timer

# Rust
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
~/.cargo/bin/rustup component add rust-analyzer

# C
sudo dnf groupinstall -y "C Development Tools And Libraries"

# Go
curl -O -L https://go.dev/dl/go1.23.2.linux-amd64.tar.gz
mkdir -p ~/.local/go
tar -C ~/.local -xzf go1.23.2.linux-amd64.tar.gz
rm -f go1.23.2.linux-amd64.tar.gz
~/.local/go/bin/go install golang.org/x/tools/gopls@latest

# i3 config
~/.cargo/bin/rustc ./i3/backlight.rs -o /tmp/backlight
sudo mv /tmp/backlight /usr/bin/backlight
sudo chown root:root /usr/bin/backlight
sudo chmod 4775 /usr/bin/backlight
cp ./i3/config ~/.config/i3/config
cp ./i3/status.sh ~/.config/i3/status.sh
cp ./i3/pomodoro.sh ~/.config/i3/pomodoro.sh
cp ./i3/tasks.sh ~/.config/i3/tasks.sh
sudo cp ./i3/note.sh /usr/bin/note
cp ./i3/monitor-setup.sh ~/.config/i3/monitor-setup.sh
sudo cp ./i3/99-monitor-hotplug.rules /etc/udev/rules.d/
sudo udevadm control --reload-rules
sudo udevadm trigger

# Zsh config
sh -c "$(curl -fsSL https://raw.github.com/ohmyzsh/ohmyzsh/master/tools/install.sh)"
cp zsh/zshrc ~/.zshrc

# Helix
curl -L https://github.com/helix-editor/helix/releases/download/25.07.1/helix-25.07.1-x86_64.AppImage -o /tmp/hx
cp -r ./helix/* ~/.config/helix
chmod +x /tmp/hx
sudo mv /tmp/hx /usr/bin
hx --health

# Virtualization
sudo dnf install -y virt-manager virt-install libvirt
sudo systemctl start libvirtd
sudo systemctl enable libvirtd
sudo usermod -a -G libvirt $(whoami)
newgrp libvirt
sudo sed -i '/unix_sock_group/c\unix_sock_group = "libvirt"' /etc/libvirt/libvirtd.conf
sudo sed -i '/unix_sock_ro_perms/c\unix_sock_ro_perms = "0770"' /etc/libvirt/libvirtd.conf
sudo systemctl restart libvirtd.service

# Codecs
sudo dnf install -y https://download1.rpmfusion.org/free/fedora/rpmfusion-free-release-$(rpm -E %fedora).noarch.rpm
sudo dnf install -y https://download1.rpmfusion.org/nonfree/fedora/rpmfusion-nonfree-release-$(rpm -E %fedora).noarch.rpm
sudo dnf install -y intel-media-driver compat-ffmpeg4 
sudo dnf swap ffmpeg-free ffmpeg --allowerasing
```

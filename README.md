# noplacelikehome

Bunch of dotfiles. Tested on ThinkPad T470s and T490s, Fedora 40 i3 spin.

``` shell
# Packages
sudo dnf install -y git alacritty lxpolkit zsh xkill
sudo dnf remove -y network-manager-applet volumeicon

# Git
git config --global user.name "Your Name"
git config --global user.email "youremail@yourdomain.com"

# Automatic upgrades
sudo dnf install -y dnf-automatic
sudo systemctl enable dnf-automatic-install.timer
sudo systemctl start dnf-automatic-install.timer

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

# Zsh config
sh -c "$(curl -fsSL https://raw.github.com/ohmyzsh/ohmyzsh/master/tools/install.sh)"
cp zsh/zshrc ~/.zshrc

# Helix
curl -L https://github.com/helix-editor/helix/releases/download/24.07/helix-24.07-x86_64.AppImage -o /tmp/hx
cp -r ./helix/* ~/.config/helix
chmod +x /tmp/hx
sudo mv /tmp/hx /usr/bin
hx --health

# Codecs
sudo dnf install -y https://download1.rpmfusion.org/free/fedora/rpmfusion-free-release-$(rpm -E %fedora).noarch.rpm
sudo dnf install -y https://download1.rpmfusion.org/nonfree/fedora/rpmfusion-nonfree-release-$(rpm -E %fedora).noarch.rpm
sudo dnf install -y intel-media-driver compat-ffmpeg4 
sudo dnf swap ffmpeg-free ffmpeg --allowerasing
```

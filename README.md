# noplacelikehome

Bunch of dotfiles. Tested on ThinkPad T470s and T490s, Fedora 37 i3 spin.

``` shell
# Packages
sudo dnf install -y git alacritty lxpolkit zsh sqlite xkill

# Git
git config --global user.name "Your Name"
git config --global user.email "youremail@yourdomain.com"

# Automatic upgrades
sudo dnf install -y dnf-automatic
sudo systemctl enable dnf-automatic-install.timer
sudo systemctl start dnf-automatic-install.timer

# Rust
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh

# Rust Analyzer
mkdir -p ~/.local/bin
rustup component add rust-analyzer
ln -s $(rustup which --toolchain stable rust-analyzer) ~/.local/bin/rust-analyzer

# C
sudo dnf groupinstall -y "C Development Tools And Libraries"

# Go
curl -O -L https://go.dev/dl/go1.19.4.linux-amd64.tar.gz
rm -rf ~/.local/go
mkdir -p ~/.local/go
tar -C ~/.local -xzf go1.19.4.linux-amd64.tar.gz
rm -f go1.19.4.linux-amd64.tar.gz

# gopls
go install golang.org/x/tools/gopls@latest

# i3 config
~/.cargo/bin/rustc ./i3/backlight.rs -o /tmp/backlight
sudo mv /tmp/backlight /usr/bin/backlight
sudo chown root:root /usr/bin/backlight
sudo chmod 4775 /usr/bin/backlight
cp ./i3/config ~/.config/i3/config
cp ./i3/status.sh ~/.config/i3/status.sh

# Zsh config
sh -c "$(curl -fsSL https://raw.github.com/ohmyzsh/ohmyzsh/master/tools/install.sh)"
cp zsh/zshrc ~/.zshrc

# Helix
mkdir -p ~/.config/helix
cp ./helix/* ~/.config/helix
git clone https://github.com/helix-editor/helix /tmp/helix
pushd /tmp/helix
git checkout 23.03
cargo install --locked --path helix-term
cp -r runtime ~/.config/helix
popd
hx --health

# Codecs
sudo dnf install -y https://mirrors.rpmfusion.org/free/fedora/rpmfusion-free-release-$(rpm -E %fedora).noarch.rpm https://mirrors.rpmfusion.org/nonfree/fedora/rpmfusion-nonfree-release-$(rpm -E %fedora).noarch.rpm
sudo dnf groupupdate -y multimedia --setop="install_weak_deps=False" --exclude=PackageKit-gstreamer-plugin
sudo dnf groupupdate -y sound-and-video
sudo dnf install -y intel-media-driver
```

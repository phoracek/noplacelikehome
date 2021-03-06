# noplacelikehome

Bunch of dotfiles. Tested on ThinkPad T470s and T490s, Fedora 34 i3 spin.

``` shell
# Packages
sudo dnf install -y git emacs

# Automatic upgrades
sudo dnf install -y dnf-automatic
sudo systemctl enable dnf-automatic-install.timer
sudo systemctl start dnf-automatic-install.timer

# Doom Emacs
git clone --depth 1 https://github.com/hlissner/doom-emacs ~/.emacs.d
~/.emacs.d/bin/doom install

# Rust
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh

# C
sudo dnf groupinstall -y "C Development Tools And Libraries"

# i3 config
~/.cargo/bin/rustc ./i3/backlight.rs -o /tmp/backlight
sudo mv /tmp/backlight /usr/bin/backlight
sudo chown root:root /usr/bin/backlight
sudo chmod 4775 /usr/bin/backlight
cp ./i3/config ~/.config/i3/config
cp ./i3/status.sh ~/.config/i3/status.sh

# Doom Emacs config
systemctl --user enable emacs
systemctl --user start emacs
cp ./emacs/config.el ~/.doom.d/config.el
cp ./emacs/init.el ~/.doom.d/init.el
~/.emacs.d/bin/doom sync
```

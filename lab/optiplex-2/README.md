# optiplex-2

`192.168.90.2`, AlmaLinux 10. Dev box and Forgejo Actions runner host.
Network: [lab README](../README.md#reaching-the-networks-behind-the-mikrotik).

| User | For |
|---|---|
| `petr` | dev box |
| `forgejo-runner` | `general` and `bench-access` runners |
| `forgejo-imagebuild` | `image-build` runner |
| `admin`, `ansible` | administration |

## Deploy

Install the OS as under [Storage](#at-installation), with an `admin` user that
can `sudo`.

```sh
ssh-copy-id -i ~/.ssh/id_rsa.pub admin@192.168.90.2
cd ansible
cp group_vars/server.yml.example group_vars/server.yml   # runner credentials
ansible-galaxy collection install -r requirements.yml
ansible-playbook -i inventory.file -u admin   -K create_ansible_user.yml
ansible-playbook -i inventory.file -u ansible    configure_storage.yml
ansible-playbook -i inventory.file -u ansible    update_dnf_packages.yml
ansible-playbook -i inventory.file -u ansible    install_dnf_automatic.yml
ansible-playbook -i inventory.file -u ansible    configure_host.yml
ansible-playbook -i inventory.file -u ansible    install_packages.yml
ansible-playbook -i inventory.file -u ansible    configure_resources.yml
ansible-playbook -i inventory.file -u ansible    create_dev_user.yml
ansible-playbook -i inventory.file -u ansible    deploy_glances.yml
ansible-playbook -i inventory.file -u ansible    deploy_forgejo_runners.yml
ansible-playbook -i inventory.file -u ansible    deploy_forgejo_imagebuild_runner.yml
```

`install_dnf_automatic.yml` installs updates daily and reboots when needed.

## Forgejo runners

| `runs-on` | Runner | Capacity | For |
|---|---|---|---|
| `general` | `forgejo-runner@general` | 2 | builds, linting, tests |
| `bench-access` | `forgejo-runner@benchaccess` | 1 | hardware bench jobs |
| `image-build` | image-build runner | 1 | ramus's `image.yml` |

`general` and `bench-access`:

- One `forgejo-runner@<name>` user unit per `forgejo_runners` entry: docker
  executor, the user's rootless Podman, `node:20-bookworm` by default.
- Removing or renaming an entry removes its instance on the next deploy.
- `bench-access` job containers have `--cpu-shares=4096`.
- One shared Actions cache server, `forgejo-cache.service`, on
  `127.0.0.1:4000`. Delete `~/.config/forgejo-cache/secret` and redeploy both
  runner playbooks to rotate its secret.

`image-build`: host executor, as `forgejo-imagebuild`, with a persistent
native-overlay layer store. Its jobs run on the host, outside any container.

`podman-prune.timer` removes dangling images for both users; tagged images
stay.

To add a runner, create it in Forgejo under **Site Administration → Actions →
Runners**, put its UUID and token in `group_vars/server.yml`, and run the runner
playbook.

```sh
sudo journalctl --user-unit forgejo-runner@general -f
sudo journalctl --user-unit forgejo-cache -f
sudo journalctl --user-unit forgejo-runner -f _UID=$(id -u forgejo-imagebuild)
```

## Resource sharing

`configure_resources.yml`:

- `user.slice` may not use the last two E-cores; they stay free for system
  services.
- Every `user-<uid>.slice` has equal CPU and IO weight.

## Glances

<https://glances.optiplex-2.lab.pacmag.cz>, via the OptiPlex's Caddy and
VoidAuth. Port 61208 is open to `192.168.0.252` only. CPU alerts at 90% and
95%.

## Storage

```
LV root  70G    →  /                      system only
LV swap  16G
LV data  rest   →  /srv                   workload data
                    ├── home/        ──bind──▶  /home
                    └── containers/  ──bind──▶  /var/lib/containers
```

`configure_storage.yml` creates the binds, with SELinux equivalences. Run it
before anything uses Podman.

### At installation

Anaconda, **Installation Destination** → **Custom**, **LVM**, xfs:

| Mount point | Capacity | Name |
|---|---|---|
| `/boot/efi` | 600 MiB | |
| `/boot` | 2 GiB | |
| `/` | 70 GiB | `root` |
| `swap` | 16 GiB | |
| `/srv` | the rest | `data` |

No `/home`. Create `admin` as an administrator.

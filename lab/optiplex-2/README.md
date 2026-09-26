# optiplex-2

The second OptiPlex (`192.168.90.2`, AlmaLinux 10), a Forgejo Actions runner
host. It sits on its own network behind the MikroTik — how that network is set
up, and what it can and cannot reach, is in the [lab
README](../README.md#reaching-the-networks-behind-the-mikrotik).

It has two kinds of users, each with an account of its own: `petr`, using it
as a dev box, and the Forgejo runners (not set up yet). `admin` and `ansible`
exist only to administer the host.

## Deploy

The OS is installed by hand with an `admin` user that can `sudo`, and with the
disk laid out as described under [Storage](#at-installation). Put your key
on it first — the playbooks after the first log in as `ansible` with the same
key, `~/.ssh/id_rsa`:

```sh
ssh-copy-id -i ~/.ssh/id_rsa.pub admin@192.168.90.2
```

Then:

```sh
cd ansible
ansible-galaxy collection install -r requirements.yml
ansible-playbook -i inventory.file -u admin   -K create_ansible_user.yml
ansible-playbook -i inventory.file -u ansible    configure_storage.yml
ansible-playbook -i inventory.file -u ansible    update_dnf_packages.yml
ansible-playbook -i inventory.file -u ansible    install_dnf_automatic.yml
ansible-playbook -i inventory.file -u ansible    configure_host.yml
ansible-playbook -i inventory.file -u ansible    install_packages.yml
ansible-playbook -i inventory.file -u ansible    create_dev_user.yml
ansible-playbook -i inventory.file -u ansible    deploy_glances.yml
```

`create_ansible_user.yml` creates the `ansible` user with passwordless sudo and
your key; everything after it runs as that user.

`install_dnf_automatic.yml` has `dnf-automatic.timer` install updates daily,
not only download them (dnf-automatic's default), and reboot the host when an
update needs it. A reboot kills any CI job running at that moment — Forgejo
marks it failed and it can be re-run.

`create_dev_user.yml` adds `petr`: key login with `~/.ssh/id_rsa`, no sudo,
and lingering enabled so rootless containers and user services keep running
after logout. `install_packages.yml` installs Podman, git and tmux; Podman runs
rootless under each user, with its storage in their home.

## Glances

Glances runs here as a rootful Quadlet container (`quadlet/glances.container`,
deployed by `deploy_glances.yml`) and is served by the OptiPlex's Caddy at
<https://glances.optiplex-2.lab.pacmag.cz>, behind the same VoidAuth gate as the
OptiPlex's own Glances. TLS, auth and the vhost all live on the OptiPlex; this
host only runs the backend.

- It uses the host network, so it reports the host's real interfaces and
  listens on an ordinary host port, 61208.
- Its web UI has no authentication and shows every process's full command
  line, so firewalld opens 61208 to `192.168.0.252` only. Anything else on the
  LAN has to come through Caddy and VoidAuth.
- The OptiPlex reaches it over its static route to `192.168.90.0/24`
  (`optiplex/ansible/configure_network.yml`); the MikroTik already forwards the
  home LAN to this network.

Bringing it up needs, besides `deploy_glances.yml` here, on the OptiPlex side:

1. An A record `glances.optiplex-2.lab.pacmag.cz` → `192.168.0.252` in the
   WEDOS zone:
   `../optiplex/scripts/wedos-dns.py set glances.optiplex-2.lab.pacmag.cz 192.168.0.252 --apply`.
2. `configure_network.yml` (the route) and `deploy_services_caddy.yml` (the
   vhost) re-run, and `deploy_services.yml` for the Dashy tile.
3. The new hostname covered by a ProxyAuth domain rule in VoidAuth's admin UI,
   like the OptiPlex's Glances.

## Storage

Root is for the system only. Everything a workload can grow — user homes,
container images, containers and volumes, and whatever comes later — lives on
one data LV mounted at `/srv`, and is bind-mounted to where it is normally
expected. A job that fills the disk fills `/srv`; the OS, its logs and dnf
keep working.

```
LV root  70G    →  /                      system only
LV swap  16G
LV data  rest   →  /srv                   all workload data
                    ├── home/        ──bind──▶  /home
                    └── containers/  ──bind──▶  /var/lib/containers
```

The installer creates only `/srv`; `configure_storage.yml` does the rest. Each
bind has an SELinux equivalence rule (`/srv/home = /home`, `/srv/containers =
/var/lib/containers`), so files are labelled exactly as the stock policy
labels the path they're seen at, and a relabel doesn't break them. Services see
their normal paths and need no reconfiguration. A new workload location is one
more entry in `srv_binds`.

On its first run the playbook copies what's already in `/home` — the
installer's `admin` and `ansible` — into `/srv/home` before binding over it;
the originals stay hidden under the mount on root, a few KB. Run it right after
`create_ansible_user.yml`, before anything installs or starts Podman.

### At installation

In the AlmaLinux installer (Anaconda):

1. **Installation Destination** → select the disk → under *Storage
   Configuration* choose **Custom** → **Done**. If the disk has an old
   install, delete its partitions first (the **−** button).
2. Set the partitioning scheme dropdown to **LVM**.
3. Add each mount point with **+** — *Mount Point* and *Desired Capacity*:

   | Mount point | Desired capacity |
   |-------------|------------------|
   | `/boot/efi` | `600 MiB` |
   | `/boot` | `2 GiB` |
   | `/` | `70 GiB` |
   | `swap` | `16 GiB` |
   | `/srv` | leave empty — takes all remaining space |

   No `/home`: it stays a plain directory on root until the playbook binds
   `/srv/home` over it.
4. Select `/` and `/srv` in turn and, in the right-hand panel, set **File
   System** to xfs and **Name** to `root` and `data`; **Update Settings**.
5. **Done** → **Accept Changes**.
6. Under **User Creation**, create `admin` with **Make this user
   administrator** ticked.

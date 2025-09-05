# Deployment (systemd)

This project ships a sample systemd unit for running `modeld` as a service.

## Unit template

See `deploy/modeld.service` for a template unit:

```ini
[Unit]
Description=modeld-go (model manager)
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=%h/Projects/modeld-go-1/bin/modeld --config %h/Projects/modeld-go-1/configs/models.yaml --addr :8080
WorkingDirectory=%h/Projects/modeld-go-1
Restart=on-failure
Environment=GOMAXPROCS=0
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=read-only

[Install]
WantedBy=multi-user.target
```

Adjust paths to match your environment, ensure the config file and models directory exist, and set any environment variables you need.

## Steps

1) Build the binary
```bash
make build
```

2) Copy or symlink the unit file
```bash
mkdir -p ~/.config/systemd/user
cp deploy/modeld.service ~/.config/systemd/user/
```

3) Reload and enable
```bash
systemctl --user daemon-reload
systemctl --user enable --now modeld.service
```

4) Inspect logs
```bash
journalctl --user -u modeld.service -f
```

5) Stop/disable
```bash
systemctl --user stop modeld.service
systemctl --user disable modeld.service
```

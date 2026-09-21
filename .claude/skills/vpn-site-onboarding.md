---
name: vpn-site-onboarding
description: How to onboard a FreePBX site - VPN Secret + DB row + egress policy. Read when adding a new site to WevetelBastion.
---

# VPN Site Onboarding

Onboarding has two independent halves: the **VPN Secret** (so a session pod can tunnel) and the **DB row** (so the site appears in the catalogue). Both are required.

## 1. Fill the site YAML

```bash
cp example-site.yaml.example my-site.yaml
```

```yaml
site:
  name: kl-branch-01          # unique; becomes Secret name vpn-kl-branch-01
  freepbx_ip: 172.16.243.249  # BARE IPv4, reachable only through the tunnel
  freepbx_port: 443
  required_permission: sites.access.standard   # standard | sensitive | restricted
  notes: KL branch
vpn:
  profile_file: vpn/kl-branch-01.ovpn   # relative to this YAML
  username: <vpn-user>                  # both or neither
  password: <vpn-pass>
  ca_file: vpn/kl-branch-01-ca.pem      # only if the .ovpn doesn't inline it
```

Decoded with `KnownFields(true)` - **unknown keys are a hard error**. Hostnames/IPv6 in `freepbx_ip` are rejected.

`required_permission` is the site's sensitivity tier and its **only** access gate: one of `sites.access.standard`, `sites.access.sensitive`, `sites.access.restricted`, validated by `permission.ValidSiteTier` (`cmd/wevetel/commands/vpn_add.go`, `internal/domain/permission/sitetier.go`). Omitting it defaults to `standard`, the least sensitive. **The tiers are independent, not a ladder**: unlike the `min_role_level >= ` comparison they replaced, holding `sites.access.restricted` grants nothing at the other two tiers, so set the true sensitivity of the site. The numeric `min_role_level` key was retired with the `RoleLevel` ladder and a file still carrying it is rejected as an unknown field.

## 2. Write the k8s Secret

```bash
wevetel vpn:add -f my-site.yaml --dry-run   # validate; prints key NAMES only, no values
wevetel vpn:add -f my-site.yaml             # creates/updates Secret vpn-<name>
```

Secret keys: `client.ovpn` (required), optional `ca.crt`/`client.crt`/`client.key`, and `credentials` (username\npassword) when given. This writes **only the Secret** - not the DB row.

## 3. Register the DB row (needs `sites.store`)

```bash
curl -s -X POST http://localhost:8080/api/v1/secure/bastion/sites/store \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"kl-branch-01","freepbx_ip":"172.16.243.249","freepbx_port":443,
       "freepbx_scheme":"https","freepbx_path":"/","insecure_tls":true,
       "required_permission":"sites.access.standard"}'
```

The route's permission is `sites.store`, stated in the `routeGuards` row in `internal/surfaces/routes/router.go`. A rejected body answers 422 with a per-field map keyed by the json tag.

> `freepbx_scheme`, `freepbx_path`, `insecure_tls` live in the **DB row**, not the YAML. The pod builds the portal URL from `scheme://ip:port/path` and gets `insecure_tls` as the `INSECURE_TLS` env var. `insecure_tls: true` makes the session Chrome ignore certificate errors for that site.

## 4. Confirm egress

`k8s/02_networkpolicy.yaml` pins session-pod egress to one VPN endpoint (`113.23.226.6/32` TCP `22888` in the committed config). Update it to match this site's VPN endpoint, or the tunnel - and the session - never comes up (the pod's entrypoint fails closed).

Last updated: auto-generated from codebase scan
